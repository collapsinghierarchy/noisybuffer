package kem

import (
	"io"

	"golang.org/x/crypto/sha3"

	"github.com/cloudflare/circl/kem/hybrid"
	"golang.org/x/crypto/hkdf"
)

// note from hybrid.go
// Package hybrid defines several hybrid classical/quantum KEMs for use in TLS.
//
// Hybrid KEMs in TLS are created by simple concatenation
// of shared secrets, cipher texts, public keys, etc.
// This is safe for TLS, see eg.
//
//	https://datatracker.ietf.org/doc/draft-ietf-tls-hybrid-design/
//	https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-56Cr2.pdf
//
// Note that this approach is not proven secure in broader context.
//
// [...]

// ----------------- Our approach -->
// Encapsulate executes the hybrid KEM and derives a 32‑byte symmetric key using
// the CatKDF‐style construction from ETSI TS 103 744 (§8.2.3) and the
// security analysis of Campagna–Petcher (ePrint 2020/1364).
//
// deriveKey implements CatKDF(context, secret) where
//
//	context = SHA‑256(m1 || m2)
//	secret  = ss_dh || ss_pq (output of hybrid.Encapsulate)
//
// ANOTHER Alternative Approach is XWING. This will be integrated later.

// scheme selects the hybrid PQC/classical KEM we use everywhere.
var scheme = hybrid.Kyber768X25519()

// Inputs:
//
//	pub    – recipient public key (binary‑encoded)
//	m1,m2  – context values (application‑defined, e.g. HTTP headers)
//
// Output:
//
//	ct     – wire ciphertext (ct_dh || ct_pq)
//	key    – HKDF‑SHA3-256(context, secret)[0:32]
func Encapsulate(pub, m1, m2 []byte) (ct, key []byte, err error) {
	pk, err := scheme.UnmarshalBinaryPublicKey(pub)
	if err != nil {
		return nil, nil, err
	}
	ct, secret, err := scheme.Encapsulate(pk)
	if err != nil {
		return nil, nil, err
	}
	return ct, deriveKey(secret, m1, m2), nil
}

// Decapsulate mirrors Encapsulate for the holder of the private key.
func Decapsulate(priv, ct, m1, m2 []byte) (key []byte, err error) {
	sk, err := scheme.UnmarshalBinaryPrivateKey(priv)
	if err != nil {
		return nil, err
	}
	secret, err := scheme.Decapsulate(sk, ct)
	if err != nil {
		return nil, err
	}
	return deriveKey(secret, m1, m2), nil
}

// Encapsulate executes the hybrid KEM and derives a 32‑byte symmetric key using
// the CatKDF‐style construction from ETSI TS 103 744 (§8.2.3) and the
// security analysis of Campagna–Petcher (ePrint 2020/1364).
//
// deriveKey implements CatKDF(context, secret) where
//
//	context = SHA‑256(m1 || m2)
//	secret  = ss_dh || ss_pq (output of hybrid.Encapsulate)
//
// TODO: Compare in detail with ETSI TS 103 744 (§8.2.3)
func deriveKey(secret, m1, m2 []byte) []byte {
	h := sha3.New256()
	h.Write(m1)
	h.Write(m2)
	context := h.Sum(nil)

	hk := hkdf.New(sha3.New256, secret, nil, context)
	key := make([]byte, 32) // AES‑256‑GCM
	if _, err := io.ReadFull(hk, key); err != nil {
		panic(err)
	}
	return key
}
