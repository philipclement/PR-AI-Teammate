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
	mux.HandleFunc("/health", methodGuard(http.MethodGet, handlers.Health))
	mux.HandleFunc("/webhook/github", methodGuard(http.MethodPost, handlers.WebhookGitHub))
	mux.HandleFunc("/analyze/pr", methodGuard(http.MethodPost, handlers.AnalyzePR))

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

func methodGuard(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}
