/* --------------------------------------------------------------------
   Access-file import  +  Pull-and-decrypt
   -------------------------------------------------------------------- */
import { CipherSuite, Aes128Gcm, HkdfSha256 }
  from "https://cdn.jsdelivr.net/npm/@hpke/core@1.7.2/+esm";
import { HybridkemX25519Kyber768 }
  from "https://cdn.jsdelivr.net/npm/@hpke/hybridkem-x25519-kyber768@1.6.1/+esm";

const out   = document.getElementById("output");
const dec   = new TextDecoder();
const file  = document.getElementById("accessFile");
const pullF = document.getElementById("pullForm");

const toArr  = b64 => Uint8Array.from(atob(b64), c => c.charCodeAt(0));
const cacheK = id  => `hpke:${id}`;
const suite  = () => new CipherSuite({
  kem:  new HybridkemX25519Kyber768(),
  kdf:  new HkdfSha256(),
  aead: new Aes128Gcm(),
});

/* -----------------------------------------------------------------
   access-file import  (trim heavy CryptoKey fields before storage)
   -----------------------------------------------------------------*/
const fileInput = document.getElementById("accessFile");

/* ---------- import access JSON (key-pair) --------------------------- */
fileInput.addEventListener("change", async () => {
  const f = fileInput.files[0];
  if (!f) return;

  try {
    const meta = JSON.parse(await f.text());               // {pubB64, privB64, kid}
    const match = f.name.match(/^noisybuffer-keypair-([0-9a-f-]+)\.json$/i);
    const appID = meta.appID || (match && match[1]) || "";

    if (!appID || !meta.pubB64 || !meta.privB64)
      throw new Error("missing fields in JSON");

    // store slim version (fits under 5 MB localStorage quota)
    localStorage.setItem(
      `hpke:${appID}`,
      JSON.stringify({ kid: meta.kid ?? 0, pubB64: meta.pubB64, privB64: meta.privB64 })
    );

    document.getElementById("pullAppId").value = appID;   // pre-fill pull form
    out.textContent = `access file imported for ${appID} ✓`;
  } catch (err) {
    out.textContent = "invalid access file";
    console.error(err);
  }
});



/* ---------- 2. pull & decrypt ------------------------------------- */
pullF.addEventListener("submit", async ev => {
  ev.preventDefault();
  const appID = pullF.pullAppId.value.trim();
  const metaS = localStorage.getItem(cacheK(appID));

  if (!metaS) return (out.textContent = "import access file first");
  const { privB64 } = JSON.parse(metaS);

  try {
    const privKey = await suite().kem.deserializePrivateKey(toArr(privB64));
    const rsp = await fetch(`/api/nb/v1/pull?appID=${encodeURIComponent(appID)}`);
    if (!rsp.ok) throw `HTTP ${rsp.status}`;

    const msgs = [], S = suite();
    for (const line of (await rsp.text()).trim().split("\\n")) {
      if (!line) continue;
      try {
        const blob = toArr(line);
        const enc  = blob.slice(0, S.kem.encSize);
        const ct   = blob.slice(S.kem.encSize);
        const ctx  = await S.createRecipientContext({ recipientKey: privKey, enc });
        msgs.push(dec.decode(await ctx.open(ct)));
      } catch { /* skip corrupt line */ }
    }
    out.textContent = msgs.join("\\n") || "(no messages)";
  } catch (e) {
    out.textContent = `decrypt error: ${e}`;
  }
});
