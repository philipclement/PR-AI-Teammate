package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/pr-ai-teammate/internal/api"
	"github.com/example/pr-ai-teammate/internal/orchestrator"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	orchestratorService := orchestrator.NewService()
	handlers := api.NewHandlers(orchestratorService)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("POST /webhook/github", handlers.WebhookGitHub)
	mux.HandleFunc("POST /analyze/pr", handlers.AnalyzePR)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	shutdownErr := make(chan error, 1)
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
		<-stop

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr <- server.Shutdown(ctx)
	}()

	log.Printf("server listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	if err := <-shutdownErr; err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
