package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	handlers := api.NewHandlers(orchestratorService, webhookSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", methodGuard(handlers.Health, http.MethodGet, http.MethodHead))
	mux.HandleFunc("/health/", methodGuard(handlers.Health, http.MethodGet, http.MethodHead))
	mux.HandleFunc("/webhook/github", methodGuard(handlers.WebhookGitHub, http.MethodPost))
	mux.HandleFunc("/analyze/pr", methodGuard(handlers.AnalyzePR, http.MethodPost))
	mux.HandleFunc("/", notFoundHandler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           requestLogger(mux),
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

func methodGuard(handler http.HandlerFunc, methods ...string) http.HandlerFunc {
	allowed := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		allowed[method] = struct{}{}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := allowed[r.Method]; !ok {
			w.Header().Set("Allow", allowHeader(methods))
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handler(w, r)
	}
}

func allowHeader(methods []string) string {
	return strings.Join(methods, ", ")
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
