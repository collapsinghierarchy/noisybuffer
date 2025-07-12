package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/collapsinghierarchy/noisybuffer/handler"
	"github.com/collapsinghierarchy/noisybuffer/service"
	"github.com/collapsinghierarchy/noisybuffer/store/postgres"
)

//
// ─── embed static assets ────────────────────────────────────────────────────────
//

//go:embed web/*
var content embed.FS

func main() {
	//----------------------------------------------------------------------
	// 1. env config
	//----------------------------------------------------------------------
	pgURL := mustEnv("DATABASE_URL")
	port := getenv("PORT", "1234")
	blobLimit := envInt("MAX_BLOB", 64*1024)

	//----------------------------------------------------------------------
	// 2. Postgres
	//----------------------------------------------------------------------
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Fatalf("pgxpool.New: %v", err)
	}
	defer pool.Close()

	//----------------------------------------------------------------------
	// 3. domain → service → API handlers
	//----------------------------------------------------------------------
	st := postgres.NewStore(pool)
	svc := service.New(st, int64(blobLimit))
	api := handler.SetupNBRoutes(svc) // /push, /pull, etc.

	//----------------------------------------------------------------------
	// 4. web UI (embed /web)
	//----------------------------------------------------------------------

	root := http.NewServeMux()
	root.Handle("/api/", http.StripPrefix("/api", api)) // API lives under /api/*
	root.Handle("/", staticHandler())                   // index.html & assets

	//----------------------------------------------------------------------
	// 5. HTTP server with graceful shutdown
	//----------------------------------------------------------------------
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      root,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("NoisyBuffer listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// CTRL-C → graceful stop
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("shutting down …")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown: %v", err)
	}
}

func staticHandler() http.Handler {
	dir := os.Getenv("WEB_DIR")
	if dir == "" {
		// In dev we expect a bind-mount. Fail fast if it’s missing.
		log.Fatal("WEB_DIR environment variable must point to your web assets directory")
	}
	return http.FileServer(http.Dir(dir))
}

// ─── helpers ────────────────────────────────────────────────────────────────────
func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("%s env var is required", key)
	}
	return v
}
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("%s must be int: %v", key, err)
	}
	return n
}
