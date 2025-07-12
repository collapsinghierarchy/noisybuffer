package service_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"

	"github.com/cloudflare/circl/kem/hybrid"
	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/pkc/kem"
	"github.com/collapsinghierarchy/noisybuffer/service"
	"github.com/google/uuid"
)

// fakeStore implements the minimal store.Store interface for tests.
type fakeStore struct {
	exists         bool
	existsErr      error
	inserted       *model.Submission
	insertErr      error
	submissions    []*model.Submission
	streamErr      error
	streamedCalled bool
}

func (f *fakeStore) AppExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeStore) InsertSubmission(ctx context.Context, s *model.Submission) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	// copy submission to inspect fields
	copy := *s
	f.inserted = &copy
	return nil
}

func (f *fakeStore) StreamSubmissions(ctx context.Context, appID uuid.UUID, fn func(*model.Submission) error) error {
	f.streamedCalled = true
	for _, s := range f.submissions {
		if err := fn(s); err != nil {
			return err
		}
	}
	return f.streamErr
}

func TestPush_Success(t *testing.T) {
	fs := &fakeStore{exists: true}
	svc := service.New(fs, 1024)

	id := uuid.New()
	blob := []byte("data")
	kid := uint8(5)

	err := svc.Push(context.Background(), id, kid, blob)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if fs.inserted == nil {
		t.Fatal("expected InsertSubmission to be called")
	}
	s := fs.inserted
	if s.AppID != id {
		t.Errorf("AppID: got %v want %v", s.AppID, id)
	}
	if s.Kid != kid {
		t.Errorf("Kid: got %d want %d", s.Kid, kid)
	}
	if string(s.Blob) != string(blob) {
		t.Errorf("Blob: got %q want %q", s.Blob, blob)
	}
	if time.Since(s.TS) > time.Second {
		t.Errorf("TS not set recently: %v", s.TS)
	}
	if s.ID == uuid.Nil {
		t.Error("ID should be generated, got Nil UUID")
	}
}

func TestPush_BlobTooLarge(t *testing.T) {
	fs := &fakeStore{exists: true}
	svc := service.New(fs, 2) // maxBlob = 2 bytes
	err := svc.Push(context.Background(), uuid.New(), 1, []byte("toolarge"))
	if err == nil || err.Error() != "blob too large" {
		t.Fatalf("expected blob too large error, got %v", err)
	}
	if fs.inserted != nil {
		t.Error("InsertSubmission should not be called on too-large blob")
	}
}

func TestPush_AppNotFound(t *testing.T) {
	fs := &fakeStore{exists: false}
	svc := service.New(fs, 1024)
	err := svc.Push(context.Background(), uuid.New(), 1, []byte("ok"))
	if !errors.Is(err, service.ErrAppNotFound) {
		t.Fatalf("expected ErrAppNotFound, got %v", err)
	}
}

func TestPush_AppExistsError(t *testing.T) {
	fs := &fakeStore{existsErr: errors.New("db down")}
	svc := service.New(fs, 1024)
	err := svc.Push(context.Background(), uuid.New(), 1, []byte("ok"))
	if err == nil || err.Error() != "db down" {
		t.Fatalf("expected db down error, got %v", err)
	}
}

func TestPull_StreamsAll(t *testing.T) {
	id := uuid.New()
	subs := []*model.Submission{
		{ID: uuid.New(), AppID: id, Blob: []byte("a")},
		{ID: uuid.New(), AppID: id, Blob: []byte("b")},
	}
	fs := &fakeStore{submissions: subs}
	svc := service.New(fs, 1024)

	var collected []*model.Submission
	err := svc.Pull(context.Background(), id, func(s *model.Submission) error {
		collected = append(collected, s)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !fs.streamedCalled {
		t.Error("StreamSubmissions was not called")
	}
	if len(collected) != len(subs) {
		t.Fatalf("expected %d submissions, got %d", len(subs), len(collected))
	}
	for i := range subs {
		if collected[i] != subs[i] {
			t.Errorf("submission[%d] mismatch: got %v want %v", i, collected[i], subs[i])
		}
	}
}

func TestPull_StreamError(t *testing.T) {
	fs := &fakeStore{streamErr: errors.New("stream fail")}
	svc := service.New(fs, 1024)
	err := svc.Pull(context.Background(), uuid.New(), func(s *model.Submission) error { return nil })
	if err == nil || err.Error() != "stream fail" {
		t.Errorf("expected stream fail error, got %v", err)
	}
}

// TestPush_EncryptedStream tests the full integration of the service
// with actual KEM+AES streaming: it encrypts sample plaintext, pushs it,
// then decrypts the stored blob and verifies the original payload.
func TestPush_EncryptedStream(t *testing.T) {
	// Generate KEM key pair
	scheme := hybrid.Kyber768X25519()
	pkObj, skObj, err := scheme.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair error: %v", err)
	}
	pubBytes, err := pkObj.MarshalBinary()
	if err != nil {
		t.Fatalf("Marshal public key error: %v", err)
	}
	privBytes, err := skObj.MarshalBinary()
	if err != nil {
		t.Fatalf("Marshal private key error: %v", err)
	}

	// Prepare plaintext and context
	plaintext := []byte("super secret data stream")
	m1, m2 := []byte("ctxA"), []byte("ctxB")

	/* --- encrypt plaintext into a single blob ---------------------------------- */
	blob, err := EncryptBlob(pubBytes, m1, m2, plaintext)
	if err != nil {
		t.Fatalf("encryptBlob error: %v", err)
	}

	// --- push via the service -------------------------------------------------
	fs := &fakeStore{exists: true}
	svc := service.New(fs, int64(len(blob)+10))
	appID := uuid.New()

	if err := svc.Push(context.Background(), appID, 1, blob); err != nil {
		t.Fatalf("Push error: %v", err)
	}
	stored := fs.inserted
	if stored == nil {
		t.Fatal("no blob stored")
	}

	// --- decrypt the stored blob ------------------------------------------------
	pt, err := DecryptBlob(privBytes, m1, m2, stored.Blob)
	if err != nil {
		t.Fatalf("decryptBlob error: %v", err)
	}
	if !bytes.Equal(pt, plaintext) {
		t.Errorf("payload mismatch: got %q want %q", pt, plaintext)
	}
}

// --- helpers ---------------------------------------------------------------
// EncryptBlob performs a single-shot hybrid encrypt+AES-GCM on the plaintext.
// Format: [4-byte big endian len(ct)] [ct bytes] [1-byte nonceLen] [nonce] [ciphertext]
func EncryptBlob(pubKey, m1, m2, plaintext []byte) ([]byte, error) {
	// 1. KEM
	ct, key, err := kem.Encapsulate(pubKey, m1, m2)
	if err != nil {
		return nil, fmt.Errorf("kem encapsulate: %w", err)
	}
	// 2. AES-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce gen: %w", err)
	}
	ctBlob := gcm.Seal(nil, nonce, plaintext, nil)

	// 3. pack
	buf := make([]byte, 4+len(ct)+1+len(nonce)+len(ctBlob))
	binary.BigEndian.PutUint32(buf[0:4], uint32(len(ct)))
	off := 4
	off += copy(buf[off:], ct)
	buf[off] = byte(len(nonce))
	off++
	off += copy(buf[off:], nonce)
	off += copy(buf[off:], ctBlob)
	return buf, nil
}

// DecryptBlob reverses EncryptBlob, returns plaintext.
func DecryptBlob(privKey, m1, m2, blob []byte) ([]byte, error) {
	// 1. unpack ct
	if len(blob) < 5 {
		return nil, fmt.Errorf("blob too short")
	}
	ctLen := binary.BigEndian.Uint32(blob[0:4])
	if len(blob) < int(4+ctLen+1) {
		return nil, fmt.Errorf("blob truncated ct")
	}
	ct := blob[4 : 4+ctLen]
	off := 4 + ctLen
	nonceLen := int(blob[off])
	off++
	if len(blob) < int(off)+nonceLen {
		return nil, fmt.Errorf("blob truncated nonce")
	}
	nonce := blob[int(off) : int(off)+nonceLen]
	off = uint32(int(off) + nonceLen)
	ctBlob := blob[off:]

	// 2. KEM decap
	key, err := kem.Decapsulate(privKey, ct, m1, m2)
	if err != nil {
		return nil, fmt.Errorf("kem decapsulate: %w", err)
	}
	// 3. AES-GCM open
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm new: %w", err)
	}
	pt, err := gcm.Open(nil, nonce, ctBlob, nil)
	if err != nil {
		return nil, fmt.Errorf("aes open: %w", err)
	}
	return pt, nil
}
