package routes

// routes/routes.go
// HTTP routing setup for NoisyBuffer API endpoints.

import (
	"net/http"

	"github.com/justinas/alice"

	"github.com/collapsinghierarchy/noisybuffer/handler"
	"github.com/collapsinghierarchy/noisybuffer/service"
)

// SetupRoutes wires all HTTP endpoints (no Prometheus instrumentation).
func SetupRoutes(svc *service.Service) http.Handler {
	srv := handler.New(svc)

	mux := http.NewServeMux()

	// NoisyBuffer API endpoints
	mux.Handle("/nb/v1/submit", http.HandlerFunc(srv.Submit))
	mux.Handle("/nb/v1/export", http.HandlerFunc(srv.Export))

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Middleware chain (logging)
	chain := alice.New(logRequest)
	return chain.Then(mux)
}

// logRequest logs basic request information.
func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		println(r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
