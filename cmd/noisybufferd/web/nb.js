/* nb.js — NoisyBuffer widget (init-function flavour)
 *
 *  Usage:
 *    <script src="https://cdn.noisybuffer.com/nb.js"></script>
 *    <script>
 *      NB.init({ appId: "YOUR_UUID", apiBase: "/api/nb/v1" });
 *    </script>
 *
 *  All <form data-noisybuffer> elements are wired automatically.
 */
;(function (global) {
  const txt = new TextEncoder();
  const b64 = u8 => btoa(String.fromCharCode.apply(null, u8));
  const u8  = s  => Uint8Array.from(atob(s), c => c.charCodeAt(0));

  // --- load HPKE libs dynamically so nb.js itself stays small ----------
  async function loadSuite() {
    const [{ CipherSuite, Aes128Gcm, HkdfSha256 },
           { HybridkemX25519Kyber768 }] = await Promise.all([
      import("https://cdn.jsdelivr.net/npm/@hpke/core@1.7.2/+esm"),
      import("https://cdn.jsdelivr.net/npm/@hpke/hybridkem-x25519-kyber768@1.6.1/+esm"),
    ]);
    return new CipherSuite({
      kem:  new HybridkemX25519Kyber768(),
      kdf:  new HkdfSha256(),
      aead: new Aes128Gcm(),
    });
  }

  /* ------------------------------------------------ NB namespace ---- */
  const NB = global.NB || (global.NB = {});

  NB.init = function init(cfg) {
    if (!cfg || !cfg.appId) {
      console.error("NB.init: {appId} is required");
      return;
    }
    const APP_ID = cfg.appId;
    const API    = cfg.apiBase || "/api/nb/v1";

    // run once DOM ready
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", () => setup(APP_ID, API));
    } else {
      setup(APP_ID, API);
    }
  };

  /* ------------------------------------------------ setup per page -- */
  async function setup(APP_ID, API) {
    const suite = await loadSuite();

    // 1. fetch & cache public key
    let cache;
    async function getKey() {
      if (cache) return cache;
      const r = await fetch(`${API}/pub?appID=${encodeURIComponent(APP_ID)}`);
      if (!r.ok) throw new Error("public key fetch failed");
      const { kid, pub } = await r.json();
      cache = {
        kid,
        pubKey: await suite.kem.deserializePublicKey(u8(pub)),
      };
      return cache;
    }

    // 2. attach handler to every form
    document.querySelectorAll("form[data-noisybuffer]").forEach(form => {
      form.addEventListener("submit", async ev => {
        ev.preventDefault();

        /* status bubble */
        const note = form.querySelector(".nb-alert") ||
                     form.appendChild(Object.assign(
                       document.createElement("div"),
                       { className:"nb-alert", style:"margin-top:.5em;font-size:.875em;" }));

        try {
          form.dataset.state = "working";
          note.textContent   = "Encrypting…";

          // collect fields
          const plain = txt.encode(JSON.stringify(
            Object.fromEntries(new FormData(form).entries())
          ));

          // seal
          const { kid, pubKey } = await getKey();
          const sender = await suite.createSenderContext({ recipientPublicKey: pubKey });
          const ct     = new Uint8Array(await sender.seal(plain));

          const blob = new Uint8Array(sender.enc.length + ct.length);
          blob.set(sender.enc, 0); blob.set(ct, sender.enc.length);

          // push
          const res = await fetch(`${API}/push`, {
            method:"POST",
            headers:{ "Content-Type":"application/json" },
            body:JSON.stringify({ appID:APP_ID, kid, blob:b64(blob) })
          });
          if (!res.ok) throw new Error(`push ${res.status}`);

          form.dataset.state = "success";
          note.style.color   = "#157347";
          note.textContent   = "Sent ✓";
          form.reset();
        } catch (e) {
          console.error("NoisyBuffer:", e);
          form.dataset.state = "error";
          note.style.color   = "#d6336c";
          note.textContent   = "Error — see console";
        } finally {
          delete form.dataset.state;
        }
      });
    });
  }
})(window);
