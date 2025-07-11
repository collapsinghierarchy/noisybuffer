# NoisyBuffer

**End-to-End Encrypted Form Data API**

NoisyBuffer lets you embed encrypted forms (such as contact pages or wait-lists) on any static site and securely collect responses. Data is encrypted client-side with a post-quantum hybrid KEM (Kyber768 + X25519) and AES-256-GCM, so the server never sees plaintext.

## Features (Work in Progress)

- **End-to-End Encryption**: Browser encrypts data before submission; server stores opaque blobs.
- **Post-Quantum Security**: Hybrid X25519‖Kyber768 for KEM, as per ETSI TS 103 744 and ePrint 2020/1364 (CatKDF). XWING will be supported as well.
- **Static Site Compatible**: Integrate with GitHub Pages, Hugo, Jekyll, etc.—no backend required.
- **Encrypted Form Data Export**: `/nb/v1/export` streams blobs; owner CLI decrypts to JSON/CSV without high memory use. Alternatively you will be able to do it within a Browser-App.

## Architecture

```
Static Site + JS Snippet      Go API Edge           PostgreSQL
<form data-noisybuffer> ──► POST /nb/v1/submit ──► store(blob)
                           GET /nb/v1/export   ──► stream blobs
```

- **Form Submission**: Client uses HPKE "hybrid" + AES-GCM to encrypt form JSON into a blob.
- **Blob Format**: `[4-byte ctLen][ct][1-byte nonceLen][nonce][ciphertext]`
- **Export & Decrypt**: Owner local CLI calls decrypt → outputs plaintext entries.

## Quick Start (Work in Progress)

1. **Clone & Run**  
   ```bash
   git clone https://github.com/whitenoise/noisybuffer.git
   cd noisybuffer
   docker compose up -d db
   go run ./cmd/noisybufferd
   ```
2. OR **Integrate into your Go Back-End**

TBD
```
import "github.com/whitenoise/noisybuffer"
```

    
3. OR **Use an existing instance**  
  TBD
4. **Embed Snippet**  
   ```html
   <script defer
     src="https://cdn.noisybuffer.com/nb.js"
     data-app="YOUR_APP_ID"
     data-kid="1">
   </script>
   <form data-noisybuffer>…</form>
   ```
5. **Collect & Export**  
   - Users submit encrypted blobs.  
   - Owner: `curl /nb/v1/export?project=… | nbctl decrypt --sk sk.pem > outputs.json`

## Project Layout

```
handler/          # HTTP handler methods (Submit, Export)
service/          # business rules (Submit, Export)
store/postgres/   # Postgres store implementation
pkc/              # cryptographic utilities (kem, aes, blob helpers)
routes.go         # HTTP routing & middleware
```

## Security Notes

- **CatKDF** construction per ETSI TS 103 744: `secret = ss_dh||ss_pq`, `context = SHA3-256(m1||m2)`, then HKDF-SHA3-256 to 32 bytes.
- **AES-GCM** encryption with random nonces, 64 KB max blob size to limit reuse.
- **Server**: validates blob size & UUID only; no plaintext ever stored.
