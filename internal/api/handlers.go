package api

import (
	"encoding/json"
	"io"
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
	_, _ = io.ReadAll(r.Body)
	respondJSON(w, http.StatusAccepted, types.WebhookResponse{Status: "received"})
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
