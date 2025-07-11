package kem_test

import (
	"bytes"
	"testing"

	"github.com/cloudflare/circl/kem/hybrid"
	kem "github.com/collapsinghierarchy/noisybuffer/pkc/kem"
)

// generateKeyPair creates a matching public/private key pair for the hybrid scheme.
func generateKeyPair(t *testing.T) (pubBytes, privBytes []byte) {
	t.Helper()
	scheme := hybrid.Kyber768X25519()
	// Current interface: GenerateKeyPair returns (kem.PublicKey, kem.PrivateKey, error)
	pk, sk, err := scheme.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}
	// MarshalBinary is implemented on both PublicKey and PrivateKey
	pubBytes, err = pk.MarshalBinary()
	if err != nil {
		t.Fatalf("PublicKey.MarshalBinary error: %v", err)
	}
	privBytes, err = sk.MarshalBinary()
	if err != nil {
		t.Fatalf("PrivateKey.MarshalBinary error: %v", err)
	}
	return pubBytes, privBytes
}

func TestEncapsulateDecapsulate(t *testing.T) {
	pub, priv := generateKeyPair(t)
	m1, m2 := []byte("ctx1"), []byte("ctx2")

	ct, key1, err := kem.Encapsulate(pub, m1, m2)
	if err != nil {
		t.Fatalf("Encapsulate failed: %v", err)
	}
	if len(ct) == 0 {
		t.Fatal("Encapsulate returned empty ciphertext")
	}
	if got, want := len(key1), 32; got != want {
		t.Fatalf("wrong key length: got %d, want %d", got, want)
	}

	key2, err := kem.Decapsulate(priv, ct, m1, m2)
	if err != nil {
		t.Fatalf("Decapsulate failed: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Error("mismatch: derived keys differ between Encapsulate and Decapsulate")
	}
}

func TestContextBinding(t *testing.T) {
	pub, _ := generateKeyPair(t)
	m1a, m2a := []byte("A"), []byte("B")
	m1b, m2b := []byte("X"), []byte("Y")

	// Encapsulate is randomized, so keys should normally differ even for same context:
	_, keyA1, _ := kem.Encapsulate(pub, m1a, m2a)
	_, keyA2, _ := kem.Encapsulate(pub, m1a, m2a)
	if bytes.Equal(keyA1, keyA2) {
		t.Error("randomized Encapsulate should produce different keys for same context")
	}

	// But different contexts should also produce different keys:
	_, keyB, _ := kem.Encapsulate(pub, m1b, m2b)
	if bytes.Equal(keyA1, keyB) {
		t.Error("different contexts should produce different derived keys")
	}
}
func TestTamperedCiphertextYieldsDifferentKey(t *testing.T) {
	pub, priv := generateKeyPair(t)
	m1, m2 := []byte("ctx1"), []byte("ctx2")

	// original
	ct, keyOrig, err := kem.Encapsulate(pub, m1, m2)
	if err != nil {
		t.Fatalf("Encapsulate failed: %v", err)
	}

	// tamper (but keep length identical)
	tampered := make([]byte, len(ct))
	copy(tampered, ct)
	tampered[0] ^= 0xFF

	// decapsulate should NOT error
	keyTam, err := kem.Decapsulate(priv, tampered, m1, m2)
	if err != nil {
		t.Fatalf("Decapsulate on tampered ciphertext should not error, got: %v", err)
	}

	// but key should almost certainly differ
	if bytes.Equal(keyOrig, keyTam) {
		t.Error("tampered ciphertext produced the same key (extremely unlikely)")
	}
}
