// register.js — minimal key‑registration helper for NoisyBuffer
import { CipherSuite, Aes128Gcm, HkdfSha256 }
  from "https://cdn.jsdelivr.net/npm/@hpke/core@1.7.2/+esm";
import { HybridkemX25519Kyber768 }
  from "https://cdn.jsdelivr.net/npm/@hpke/hybridkem-x25519-kyber768@1.6.1/+esm";

const out = document.getElementById("output");
const MY_ID_KEY = "nb:my-app-id";

/* -------------------------------------------------- persistent App ID */
let myAppId = localStorage.getItem(MY_ID_KEY);
if (!myAppId) {
  myAppId = crypto.randomUUID();
  localStorage.setItem(MY_ID_KEY, myAppId);
}
document.getElementById("regAppId").value = myAppId;

/* -------------------------------------------------- helpers */
function suiteFactory() {
  return new CipherSuite({
    kem:  new HybridkemX25519Kyber768(),
    kdf:  new HkdfSha256(),
    aead: new Aes128Gcm(),
  });
}
function toB64(u8) { return btoa(String.fromCharCode(...u8)); }
function fromB64(s) { return Uint8Array.from(atob(s), c => c.charCodeAt(0)); }

async function loadOrCreateKeypair(appID) {
  const key = `hpke:${appID}`;
  const suite = suiteFactory();
  const cached = localStorage.getItem(key);
  if (cached) {
    return JSON.parse(cached);
  }
  const kp = await suite.kem.generateKeyPair();
  const pub  = await suite.kem.serializePublicKey(kp.publicKey);
  const priv = await suite.kem.serializePrivateKey(kp.privateKey);
  const obj = { kid: 0, pubB64: toB64(pub), privB64: toB64(priv) };
  localStorage.setItem(key, JSON.stringify(obj));
  return obj;
}

function downloadJSON(obj, fname) {
  const url = URL.createObjectURL(new Blob([JSON.stringify(obj)], {type:"application/json"}));
  Object.assign(document.createElement("a"), { href:url, download:fname }).click();
  URL.revokeObjectURL(url);
}

/* -------------------------------------------------- UI bindings */
document.getElementById("exportBtn").addEventListener("click", async () => {
  const pair = await loadOrCreateKeypair(myAppId);
  downloadJSON({ appID: myAppId, ...pair }, `noisybuffer-keypair-${myAppId}.json`);
  out.textContent = "key‑pair downloaded ✓";
});

document.getElementById("regForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const { pubB64, kid } = await loadOrCreateKeypair(myAppId);
  const rsp = await fetch("/api/nb/v1/key", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ appID: myAppId, kid, pub: pubB64 }),
  });
  out.textContent = `register: ${rsp.status} ${rsp.statusText}`;
});
