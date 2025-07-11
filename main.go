package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/collapsinghierarchy/noisybuffer/routes"
	"github.com/collapsinghierarchy/noisybuffer/service"
	"github.com/collapsinghierarchy/noisybuffer/store/postgres"
)

func main() {

	pgURL := os.Getenv("DATABASE_URL")
	if pgURL == "" {
		log.Fatal("DATABASE_URL env var is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, pgURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	st := postgres.NewStore(pool)
	svc := service.New(st, 64*1024) // 64 KB max blob

	mux := routes.SetupRoutes(svc) // from routes.go

	addr := ":8080"
	log.Printf("NoisyBuffer listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
