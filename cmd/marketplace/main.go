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
	"syscall"
	"time"

	"AIxVuln/marketplace"
)

func main() {
	port := flag.Int("port", 8081, "HTTP listen port")
	flag.Parse()

	server := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", *port),
		Handler:           marketplace.NewRouter(marketplace.NewStore()),
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
