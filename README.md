# NoisyBuffer — End-to-End Encrypted Forms API

NoisyBuffer is an end-to-end-encrypted (E2EE) form backend for static sites and Jamstack pages.
A drop-in `<script>` seals every field in the browser with post-quantum crypto (Kyber-768 × X25519 → AES-256-GCM) and streams an opaque blob to a lightweight Go API—so neither your server nor any third-party ever sees plaintext. Plug it in where you’d use Formspree or Netlify Forms and stay GDPR-proof and post-quantum ready.

> **Status:** early WIP — API surface will change

---

## ✨ Features
| Capability | Details |
|------------|---------|
| **True E2EE** | Form data is encrypted *in the browser*; the server only stores opaque blobs. |
| **Post‑quantum hybrid** | Kyber‑768 × X25519 → AES‑256‑GCM. With [hpke-js](https://github.com/dajiaji/hpke-js) and [WebCrypto API](https://developer.mozilla.org/en-US/docs/Web/API/Crypto) |
| **Static‑site friendly** | Works behind GitHub Pages, Netlify, S3, etc. — just drop the JS snippet. |
| **Owner export** | Stream `/nb/v1/pull` → decrypt locally → JSON / CSV. |

*XWING KEM and browser‑based exporter are on the roadmap.*

---

## 🗺️ Architecture

```
Static site          Go edge API             PostgreSQL
<form> --(HPKE)--> POST /nb/v1/push  --> blobs
               <--  GET  /nb/v1/pull  --< stream blobs
```
`blob = enc ‖ ct`, where `ct = AES‑GCM(plaintext)`.

---

## 🚀 Quick Start (dev)

```bash
git clone https://github.com/whitenoise/noisybuffer
cd noisybuffer
docker compose up -d            # Postgres + API
open http://localhost:1234      # demo Push/Pull page
```

---

## 🏗️ Embed on any page (Preview of the Functionality)

```html
<script defer
        src="https://cdn.noisybuffer.com/nb.js"
        data-app="YOUR_APP_ID">
</script>

<form data-noisybuffer>
  <input name="email" required>
  <button>Join wait‑list</button>
</form>
```

The snippet fetches your public key, encrypts fields, and calls `/nb/v1/push`.

---

## 📦 Project layout

```
cmd/noisybufferd/   main.go + embedded demo UI
handler/            HTTP handlers (push, pull, key)
service/            domain logic (validation, E2EE)
store/postgres/     SQL adapter (implements store.Store)
web/                index.html, app.js test harness
```

---

## 🔐 Security notes

* **Hybrid KEM**  
* **AEAD**
* **Server**: validates length & UUID only; never sees plaintext or private keys.  
---

> Contributions welcome!  Open issues or pull requests to discuss improvements.
