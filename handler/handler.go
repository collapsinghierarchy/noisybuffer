package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/justinas/alice"

	"github.com/collapsinghierarchy/noisybuffer/model"
	"github.com/collapsinghierarchy/noisybuffer/service"
)

// Server bundles dependencies for HTTP handlers and exposes concrete
// HTTP endpoints for the NoisyBuffer API.
//
// It keeps its dependency surface intentionally narrow: the only thing
// it needs is a *service.Service implementation that performs the core
// business logic (validation, persistence, streaming, …).
type Server struct {
	svc *service.Service
}

// New constructs a ready-to-use Server instance.
func New(svc *service.Service) *Server { return &Server{svc: svc} }

// ------------------------------------------------------------
// Types
// ------------------------------------------------------------

type pushRequest struct {
	App  string `json:"app"`  // UUID (base‑36) identifying the client application
	Kid  uint8  `json:"kid"`  // Key‑ID used for envelope encryption
	Blob string `json:"blob"` // base64(ct_kem|iv|ct_aes)
}

// ------------------------------------------------------------
// Handlers
// ------------------------------------------------------------

type registerKeyReq struct {
	App string `json:"app"` // UUID
	Kid uint8  `json:"kid"`
	Pub string `json:"pub"` // base64
}

func (s *Server) RegisterKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	println("RegisterKey: received request")
	var req registerKeyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		println("RegisterKey: failed to decode JSON:", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	println("RegisterKey: decoded JSON, App =", req.App, "Kid =", req.Kid)
	appID, err := uuid.Parse(req.App)
	if err != nil {
		println("RegisterKey: invalid app id:", req.App)
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}
	pub, err := base64.StdEncoding.DecodeString(req.Pub)
	if err != nil {
		println("RegisterKey: invalid pub base64:", req.Pub)
		http.Error(w, "invalid pub", http.StatusBadRequest)
		return
	}
	println("RegisterKey: calling service.RegisterKey")
	if err := s.svc.RegisterKey(r.Context(), appID, req.Kid, pub); err != nil {
		println("RegisterKey: service.RegisterKey failed:", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	println("RegisterKey: key registered successfully")
	w.WriteHeader(http.StatusCreated)
}

type getKeyResp struct {
	Kid uint8  `json:"kid"`
	Pub string `json:"pub"` // base64
}

func (s *Server) PublicKey(w http.ResponseWriter, r *http.Request) {
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
	kid, pub, err := s.svc.GetKey(r.Context(), appID)
	if err == service.ErrKeyNotFound {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(getKeyResp{
		Kid: kid,
		Pub: base64.StdEncoding.EncodeToString(pub),
	})
}

// Push ingests a single encrypted submission sent by a client.
// The binary cipher‑text is provided using base64 so it survives
// JSON marshalling and transport intact.
func (s *Server) Push(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req pushRequest
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

	if err := s.svc.Push(r.Context(), appID, req.Kid, blobBytes); err != nil {
		if err == service.ErrAppNotFound {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// Pull streams—newline‑delimited—every pending submission for the given
// application. The connection is held open until the service returns EOF
// (i.e. the app queue is empty) or an unrecoverable error occurs.
func (s *Server) Pull(w http.ResponseWriter, r *http.Request) {
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
	err = s.svc.Pull(r.Context(), appID, func(sub *model.Submission) error {
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

// ------------------------------------------------------------
// Router
// ------------------------------------------------------------

// SetupNBRoutes returns an http.Handler that exposes the two public
// endpoints (push & pull) wrapped with minimal logging middleware.
func SetupNBRoutes(svc *service.Service) http.Handler {
	srv := New(svc)

	mux := http.NewServeMux()
	mux.Handle("/nb/v1/key", http.HandlerFunc(srv.RegisterKey))
	mux.Handle("/nb/v1/push", http.HandlerFunc(srv.Push))
	mux.Handle("/nb/v1/pull", http.HandlerFunc(srv.Pull))

	// You can extend the alice chain with additional middleware (tracing,
	// metrics, …) in a single spot without touching the handlers.
	chain := alice.New(logRequest)
	return chain.Then(mux)
}

// logRequest is a tiny middleware printing request method & path to stdout.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
