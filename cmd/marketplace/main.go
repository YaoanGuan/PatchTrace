// Command marketplace runs the PatchTrace AI marketplace HTTP API.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"AIxVuln/marketplace"
)

func main() {
	port := flag.Int("port", 8081, "HTTP listen port")
	dbPath := flag.String("db", "data/marketplace.db", "SQLite database path")
	flag.Parse()

	if dir := filepath.Dir(*dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("create marketplace db directory %s: %v", dir, err)
		}
	}
	store, err := marketplace.OpenStore(*dbPath)
	if err != nil {
		log.Fatalf("open marketplace store: %v", err)
	}
	defer func() { _ = store.Close() }()

	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", *port),
		Handler:           marketplace.NewRouter(store),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("marketplace API listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("marketplace API listen failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("marketplace API shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("marketplace API shutdown error: %v", err)
	}
	log.Println("marketplace API stopped")
}
