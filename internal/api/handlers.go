package api

import (
	"encoding/json"
	"io"
<<<<<<< ours
=======
	"log"
>>>>>>> theirs
	"net/http"

	"github.com/example/pr-ai-teammate/internal/orchestrator"
	"github.com/example/pr-ai-teammate/internal/types"
)

type Handlers struct {
	orchestrator *orchestrator.Service
}

func NewHandlers(orchestrator *orchestrator.Service) *Handlers {
	return &Handlers{orchestrator: orchestrator}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, types.HealthResponse{Status: "ok"})
}

func (h *Handlers) WebhookGitHub(w http.ResponseWriter, r *http.Request) {
<<<<<<< ours
	_, _ = io.ReadAll(r.Body)
	respondJSON(w, http.StatusAccepted, types.WebhookResponse{Status: "received"})
=======
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
>>>>>>> theirs
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
