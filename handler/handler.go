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

// Server bundles dependencies for HTTP handlers.
type Server struct {
	svc *service.Service
}

// New constructs a ready-to-use Server instance.
func New(svc *service.Service) *Server { return &Server{svc: svc} }

// ------------------------------------------------------------
// Request & Response Structs
// ------------------------------------------------------------

type registerKeyReq struct {
	AppID string `json:"appID"` // UUID
	Kid   uint8  `json:"kid"`
	Pub   string `json:"pub"` // base64
}

type registerKeyResp struct {
	Message string `json:"message"`
}

type publicKeyReq struct {
	AppID string `json:"appID"`
}

type publicKeyResp struct {
	Kid uint8  `json:"kid"`
	Pub string `json:"pub"` // base64
}

type pushRequest struct {
	AppID string `json:"appID"` // UUID (base‑36)
	Kid   uint8  `json:"kid"`   // Key‑ID used for envelope encryption
	Blob  string `json:"blob"`  // base64(ciphertext)
}

type pushResp struct {
	Message string `json:"message"`
}

type pullRequest struct {
	AppID string `json:"appID"`
}

// ------------------------------------------------------------
// Router
// ------------------------------------------------------------

func SetupNBRoutes(svc *service.Service) http.Handler {
	srv := New(svc)

	mux := http.NewServeMux()
	mux.Handle("POST /nb/v1/key", http.HandlerFunc(srv.RegisterKey))
	mux.Handle("GET /nb/v1/pub", http.HandlerFunc(srv.PublicKey))
	mux.Handle("POST /nb/v1/push", http.HandlerFunc(srv.Push))
	mux.Handle("GET /nb/v1/pull", http.HandlerFunc(srv.Pull))

	chain := alice.New(logRequest)
	return chain.Then(mux)
}

// logRequest is a tiny middleware printing request method & path.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// ------------------------------------------------------------
// Handlers
// ------------------------------------------------------------

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
	println("RegisterKey: decoded JSON, AppID =", req.AppID, "Kid =", req.Kid)
	appID, err := uuid.Parse(req.AppID)
	if err != nil {
		println("RegisterKey: invalid app id:", req.AppID)
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}
	// Efficiently check if AppID already exists
	exists, err := s.svc.Store.AppExists(r.Context(), appID)
	if err != nil {
		println("RegisterKey: error checking app existence:", err.Error())
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if exists {
		println("RegisterKey: appID already exists:", req.AppID)
		http.Error(w, "appID already registered", http.StatusConflict) // 409 Conflict
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
	if err := json.NewEncoder(w).Encode(registerKeyResp{Message: "key registered successfully"}); err != nil {
		println("RegisterKey: failed to encode JSON response:", err.Error())
	}
}

func (s *Server) PublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Extract the public key request from query parameters.
	appIDStr := r.URL.Query().Get("appID")
	if appIDStr == "" {
		http.Error(w, "missing appID", http.StatusBadRequest)
		return
	}
	appID, err := uuid.Parse(appIDStr)
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
	resp := publicKeyResp{
		Kid: kid,
		Pub: base64.StdEncoding.EncodeToString(pub),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Push ingests one encrypted blob: {appID, kid, blob (base64)}.
func (s *Server) Push(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// ----- decode JSON ------------------------------------------------
	var req pushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appID, err := uuid.Parse(req.AppID)
	if err != nil {
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}

	// ----- pre-flight: does this App exist? ---------------------------
	exists, err := s.svc.Store.AppExists(r.Context(), appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !exists {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	// ----- decode blob ------------------------------------------------
	blobBytes, err := base64.StdEncoding.DecodeString(req.Blob)
	if err != nil {
		http.Error(w, "invalid blob", http.StatusBadRequest)
		return
	}

	// ----- persist ----------------------------------------------------
	if err := s.svc.Push(r.Context(), appID, req.Kid, blobBytes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// ----- done -------------------------------------------------------
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(pushResp{Message: "push successful"})
}

// Pull streams every pending submission for the given app.
// Response: text/plain; each line = base64(blob)\n
func (s *Server) Pull(w http.ResponseWriter, r *http.Request) {
	// 1. method guard --------------------------------------------------
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 2. extract & validate appID -------------------------------------
	appIDStr := r.URL.Query().Get("appID")
	if appIDStr == "" {
		http.Error(w, "missing appID", http.StatusBadRequest)
		return
	}
	appID, err := uuid.Parse(appIDStr)
	if err != nil {
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}

	// 3. stream blobs --------------------------------------------------
	w.Header().Set("Content-Type", "text/plain")

	err = s.svc.Pull(r.Context(), appID, func(sub *model.Submission) error {
		line := base64.StdEncoding.EncodeToString(sub.Blob)
		_, err := w.Write(append([]byte(line), '\n'))
		return err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
