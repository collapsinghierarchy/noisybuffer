package handler_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/collapsinghierarchy/noisybuffer/handler"
	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/routes"
	"github.com/collapsinghierarchy/noisybuffer/service"
)

//--- minimal fake store (same pattern as service tests) --------------------

type fakeStore struct {
	exists      bool
	inserted    *model.Submission
	submissions []*model.Submission
}

func (f *fakeStore) AppExists(ctx context.Context, id uuid.UUID) (bool, error) {
	return f.exists, nil
}
func (f *fakeStore) InsertSubmission(ctx context.Context, s *model.Submission) error {
	copy := *s
	f.inserted = &copy
	return nil
}
func (f *fakeStore) StreamSubmissions(ctx context.Context, id uuid.UUID, fn func(*model.Submission) error) error {
	for _, s := range f.submissions {
		if err := fn(s); err != nil {
			return err
		}
	}
	return nil
}

// -------------------------------------------------------------------------
func TestSubmitHandler_Success(t *testing.T) {
	fs := &fakeStore{exists: true}
	svc := service.New(fs, 1024)
	mux := routes.SetupRoutes(svc)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	appID := uuid.New()
	rawBlob := []byte("abc123")
	reqBody, _ := json.Marshal(map[string]interface{}{
		"app":  appID.String(), // <-- fix here
		"kid":  1,
		"blob": base64.StdEncoding.EncodeToString(rawBlob),
	})

	resp, err := http.Post(srv.URL+"/nb/v1/submit", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("POST submit error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: got %d", resp.StatusCode)
	}
	if fs.inserted == nil {
		t.Fatal("InsertSubmission was not called")
	}
	if !bytes.Equal(fs.inserted.Blob, rawBlob) {
		t.Errorf("stored blob mismatch: %q vs %q", fs.inserted.Blob, rawBlob)
	}
}

func TestSubmitHandler_InvalidJSON(t *testing.T) {
	fs := &fakeStore{exists: true}
	svc := service.New(fs, 1024)
	h := handler.New(svc)
	mux := http.NewServeMux()
	mux.Handle("/nb/v1/submit", http.HandlerFunc(h.Submit))
	mux.Handle("/nb/v1/export", http.HandlerFunc(h.Export))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, _ := http.Post(srv.URL+"/nb/v1/submit", "application/json", bytes.NewReader([]byte("notjson")))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestExportHandler_Success(t *testing.T) {
	appID := uuid.New()
	fs := &fakeStore{
		exists: true,
		submissions: []*model.Submission{
			{ID: uuid.New(), AppID: appID, Blob: []byte("a")},
			{ID: uuid.New(), AppID: appID, Blob: []byte("b")},
		},
	}
	svc := service.New(fs, 1024)
	h := handler.New(svc)
	mux := http.NewServeMux()
	mux.Handle("/nb/v1/submit", http.HandlerFunc(h.Submit))
	mux.Handle("/nb/v1/export", http.HandlerFunc(h.Export))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/nb/v1/export?app=" + appID.String()) // <-- fix here
	if err != nil {
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	body, _ := ioutil.ReadAll(resp.Body)
	if !bytes.Equal(body, []byte("a\nb\n")) {
		t.Errorf("unexpected body: %q", body)
	}
}
