import { CipherSuite, Aes128Gcm, HkdfSha256 }
  from "https://cdn.jsdelivr.net/npm/@hpke/core@1.7.2/+esm";

import { HybridkemX25519Kyber768 }
  from "https://cdn.jsdelivr.net/npm/@hpke/hybridkem-x25519-kyber768@1.6.1/+esm";

const out = document.getElementById("output");
const enc = new TextEncoder(), dec = new TextDecoder();

// --------------------------------------------------------------------
// Persistent "my App ID" 
// --------------------------------------------------------------------
const MY_ID_KEY = "nb:my-app-id";

let myAppId = localStorage.getItem(MY_ID_KEY);
if (!myAppId) {
    myAppId = crypto.randomUUID();          // Web-standard UUIDv4
    localStorage.setItem(MY_ID_KEY, myAppId);
}

// Prefill the two inputs that refer to *your* app ID
document.addEventListener("DOMContentLoaded", () => {
  document.getElementById("regAppId").value  = myAppId;
  document.getElementById("pullAppId").value = myAppId;
});

// ---------- Persistent keypair helpers --------------------------------
async function loadOrCreateKeypair(appId) {
  const dbKey = `hpke:${appId}`;
  const suite = suiteFactory();

  // ---- hit in localStorage? ----------------------------------------
  const cached = localStorage.getItem(dbKey);
  if (cached) {
    const { pubB64, privB64, kid } = JSON.parse(cached);
    const pubKey  = await suite.kem.deserializePublicKey(b64ToArray(pubB64));
    const privKey = await suite.kem.deserializePrivateKey(b64ToArray(privB64));
    return { pubB64, privB64, pubKey, privKey, kid };
  }

  // ---- first run → generate & persist ------------------------------
  const kp = await suite.kem.generateKeyPair();
  const pubBytes  = await suite.kem.serializePublicKey(kp.publicKey);
  const privBytes = await suite.kem.serializePrivateKey(kp.privateKey);

  const obj = {
    pubB64: arrayToB64(pubBytes),
    privB64: arrayToB64(privBytes),
    kid: 0,
  };
  localStorage.setItem(dbKey, JSON.stringify(obj));
  return { ...obj, pubKey: kp.publicKey, privKey: kp.privateKey };
}

function suiteFactory() {
  return new CipherSuite({
    kem:  new HybridkemX25519Kyber768(),
    kdf:  new HkdfSha256(),
    aead: new Aes128Gcm(),
  });
}

function arrayToB64(buf) {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  return btoa(String.fromCharCode(...bytes));
}

function b64ToArray(str)  {
  return Uint8Array.from(atob(str), c => c.charCodeAt(0));
}

function downloadJSON(obj, filename) {
  const blob = new Blob([JSON.stringify(obj)], {type: "application/json"});
  const url  = URL.createObjectURL(blob);
  const a    = document.createElement("a");
  a.href = url; a.download = filename; a.click();
  URL.revokeObjectURL(url);
}

// ---------- Register / rotate ----------------------------------------
// --- Download key-pair -----------------------------------------------
document.getElementById("exportBtn").addEventListener("click", async () => {
  const appId = document.getElementById("regAppId").value.trim();
  if (!appId) { out.textContent = "enter App ID first"; return; }

  const pair = await loadOrCreateKeypair(appId);
  downloadJSON(pair, `noisybuffer-keypair-${appId}.json`);
  out.textContent = "key-pair downloaded";
});

document.getElementById("regForm").addEventListener("submit", async ev => {
  ev.preventDefault();
  const appId = document.getElementById("regAppId").value.trim();
  const { pubB64, kid } = await loadOrCreateKeypair(appId);

  const res = await fetch("/api/nb/v1/key", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ appID: appId, kid, pub: pubB64 }),
  });
  out.textContent = `register: ${res.status} ${res.statusText}`;
});

// ---------- PUSH ------------------------------------------------------
document.getElementById("pushForm").addEventListener("submit", async ev => {
  ev.preventDefault();
  const recipId = document.getElementById("appId").value.trim();
  const message  = document.getElementById("msg").value;

  // 1. fetch recipient public key from the new endpoint '/api/nb/v1/pub'
  const r = await fetch(`/api/nb/v1/pub?appID=${encodeURIComponent(recipId)}`);
  if (!r.ok) { 
    out.textContent = "no public key registered"; 
    return; 
  }
  const { kid, pub } = await r.json();

  // 2. seal with HPKE
  const suite = suiteFactory();
  const pubKey = await suite.kem.deserializePublicKey(b64ToArray(pub));
  const sender = await suite.createSenderContext({ recipientPublicKey: pubKey });
  const ciphertextBuf = await sender.seal(enc.encode(message));
  const ciphertext = new Uint8Array(ciphertextBuf);
  console.log("ct length:", ciphertext.length);

  const blob = new Uint8Array(sender.enc.length + ciphertext.length);
  if (blob.length !== sender.enc.length + ciphertext.length) {
    out.textContent = "Internal error: blob size mismatch";
    return;
  }
  blob.set(sender.enc, 0);
  blob.set(ciphertext, sender.enc.length);

  // 4. push the sealed message using POST to the push handler
  const pushRes = await fetch("/api/nb/v1/push", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      appID: recipId,
      kid,
      blob: arrayToB64(blob),
    }),
  });
  out.textContent = `push: ${pushRes.status} ${pushRes.statusText}`;
});

// ---------- PULL ------------------------------------------------------
document.getElementById("pullForm").addEventListener("submit", async ev => {
  ev.preventDefault();
  const appId = document.getElementById("pullAppId").value.trim();
  const { privKey } = await loadOrCreateKeypair(appId); // <-- use privKey

  const suite = suiteFactory();

  const res = await fetch(`/api/nb/v1/pull?appID=${encodeURIComponent(appId)}`);
  if (!res.ok) { out.textContent = `${res.status}`; return; }

  const lines = (await res.text()).trim().split("\n");
  const msgs = [];
  for (const line of lines) {
    if (!line.trim()) continue; // skip empty lines
    try {
      const blob = b64ToArray(line);
      const encPart = blob.slice(0, suite.kem.encSize);
      const ctPart  = blob.slice(suite.kem.encSize);

      const recipCtx = await suite.createRecipientContext({
        recipientKey: privKey,
        enc: encPart,
      });
      const pt = await recipCtx.open(ctPart);
      msgs.push(dec.decode(pt));
    } catch (e) {
      console.warn("Skipping invalid base64 line:", line, e);
      continue;
    }
  }
  out.textContent = msgs.join("\n");
});