package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/example/pr-ai-teammate/internal/orchestrator"
	"github.com/example/pr-ai-teammate/internal/types"
)

type Handlers struct {
	orchestrator  Analyzer
	webhookSecret string
}

type Analyzer interface {
	AnalyzePR(ctx context.Context, input orchestrator.AnalyzeInput) (orchestrator.AnalyzeResult, error)
}

var _ Analyzer = (*orchestrator.Service)(nil)

func NewHandlers(orchestrator Analyzer, webhookSecret string) *Handlers {
	return &Handlers{
		orchestrator:  orchestrator,
		webhookSecret: webhookSecret,
	}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, types.HealthResponse{Status: "ok"})
}

func (h *Handlers) WebhookGitHub(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		respondError(w, http.StatusBadRequest, "missing X-GitHub-Event header")
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, http.StatusBadRequest, "unable to read request body")
		return
	}

	if err := h.verifySignature(r.Header.Get("X-Hub-Signature-256"), payload); err != nil {
		respondError(w, http.StatusUnauthorized, err.Error())
		return
	}

	if event != "pull_request" {
		respondJSON(w, http.StatusOK, types.WebhookResponse{Status: "ignored"})
		return
	}

	var prEvent types.PullRequestEvent
	if err := json.Unmarshal(payload, &prEvent); err != nil {
		respondError(w, http.StatusBadRequest, "invalid pull_request payload")
		return
	}

	if !prEvent.IsActionSupported() {
		respondJSON(w, http.StatusOK, types.WebhookResponse{Status: "ignored"})
		return
	}

	if prEvent.PullRequest.Number == 0 || prEvent.Repository.FullName == "" || prEvent.PullRequest.Head.SHA == "" {
		respondError(w, http.StatusBadRequest, "pull_request payload missing required fields")
		return
	}

	result, err := h.orchestrator.AnalyzePR(r.Context(), orchestrator.AnalyzeInput{
		Repository: prEvent.Repository.FullName,
		PullNumber: prEvent.PullRequest.Number,
		CommitSHA:  prEvent.PullRequest.Head.SHA,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("queued analysis for %s#%d (%s)", prEvent.Repository.FullName, prEvent.PullRequest.Number, prEvent.PullRequest.Head.SHA)
	respondJSON(w, http.StatusAccepted, types.WebhookResponse{Status: result.Summary})
}

func (h *Handlers) AnalyzePR(w http.ResponseWriter, r *http.Request) {
	var req types.AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	result, err := h.orchestrator.AnalyzePR(r.Context(), orchestrator.AnalyzeInput{
		Repository: req.Repository,
		PullNumber: req.PullNumber,
		CommitSHA:  req.CommitSHA,
	})
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, types.AnalyzeResponse{
		Status:  "queued",
		Message: result.Summary,
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func (h *Handlers) verifySignature(signature string, payload []byte) error {
	if h.webhookSecret == "" {
		return nil
	}
	if signature == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}

	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return fmt.Errorf("invalid X-Hub-Signature-256 header")
	}

	expectedMAC := hmac.New(sha256.New, []byte(h.webhookSecret))
	expectedMAC.Write(payload)
	expected := expectedMAC.Sum(nil)

	receivedHex := strings.TrimPrefix(signature, prefix)
	received, err := hex.DecodeString(receivedHex)
	if err != nil {
		return fmt.Errorf("invalid X-Hub-Signature-256 header")
	}

	if !hmac.Equal(received, expected) {
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}
