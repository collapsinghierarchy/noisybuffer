package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/service"
)

type Server struct {
	prefix string
	svc    *service.Service
}

type submitRequest struct {
	App  string `json:"app"`
	Kid  uint8  `json:"kid"`
	Blob string `json:"blob"` // base64(ct_kem|iv|ct_aes)
}

// New returns a ready Server instance.
func New(svc *service.Service) *Server { return &Server{svc: svc} }

func (s *Server) Submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	appID, err := uuid.Parse(req.App)
	if err != nil {
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}
	blobBytes, err := base64.StdEncoding.DecodeString(req.Blob)
	if err != nil {
		http.Error(w, "invalid blob", http.StatusBadRequest)
		return
	}
	if err := s.svc.Submit(r.Context(), appID, req.Kid, blobBytes); err != nil {
		if err == service.ErrAppNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	app := r.URL.Query().Get("app")
	if app == "" {
		http.Error(w, "missing app", http.StatusBadRequest)
		return
	}
	appID, err := uuid.Parse(app)
	if err != nil {
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	err = s.svc.Export(r.Context(), appID, func(sub *model.Submission) error {
		if _, err := w.Write(sub.Blob); err != nil {
			return err
		}
		_, err := w.Write([]byte{'\n'})
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
