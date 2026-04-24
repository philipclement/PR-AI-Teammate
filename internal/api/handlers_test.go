package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/pr-ai-teammate/internal/orchestrator"
)

type stubEnqueuer struct {
	called bool
	input  orchestrator.AnalyzeInput
	err    error
}

func (s *stubEnqueuer) Enqueue(input orchestrator.AnalyzeInput) error {
	s.called = true
	s.input = input
	return s.err
}

func TestHealth(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()

	handlers.Health(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if got := res.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("unexpected body: %s", got)
	}
}

func TestWebhookGitHubMissingHeader(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", nil)
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestWebhookGitHubIgnoredEvent(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", nil)
	req.Header.Set("X-GitHub-Event", "ping")
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
}

func TestWebhookGitHubDispatchesAnalysis(t *testing.T) {
	stub := &stubEnqueuer{}
	handlers := NewHandlers(stub, "")

	payload := map[string]any{
		"action": "opened",
		"number": 7,
		"pull_request": map[string]any{
			"number": 7,
			"head":   map[string]any{"sha": "abc123"},
		},
		"repository": map[string]any{
			"full_name": "acme/demo",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if !stub.called {
		t.Fatalf("expected enqueuer to be called")
	}
	if stub.input.Repository != "acme/demo" {
		t.Fatalf("unexpected repository: %s", stub.input.Repository)
	}
	if stub.input.PullNumber != 7 {
		t.Fatalf("unexpected pull number: %d", stub.input.PullNumber)
	}
	if stub.input.CommitSHA != "abc123" {
		t.Fatalf("unexpected commit SHA: %s", stub.input.CommitSHA)
	}
}

func TestWebhookGitHubQueueFull(t *testing.T) {
	stub := &stubEnqueuer{err: errors.New("queue is full")}
	handlers := NewHandlers(stub, "")

	payload := map[string]any{
		"action": "opened",
		"number": 1,
		"pull_request": map[string]any{
			"number": 1,
			"head":   map[string]any{"sha": "abc123"},
		},
		"repository": map[string]any{
			"full_name": "acme/demo",
		},
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", res.Code)
	}
}

func TestWebhookGitHubPayloadTooLarge(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "")
	bigPayload := bytes.Repeat([]byte("a"), 1<<20+1) // 1 MB + 1 byte

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(bigPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", res.Code)
	}
}

func TestWebhookGitHubSignatureMissing(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewBufferString(`{"action":"opened"}`))
	req.Header.Set("X-GitHub-Event", "pull_request")
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", res.Code)
	}
}

func TestWebhookGitHubSignatureValid(t *testing.T) {
	stub := &stubEnqueuer{}
	handlers := NewHandlers(stub, "secret")

	payload := map[string]any{
		"action": "opened",
		"number": 7,
		"pull_request": map[string]any{
			"number": 7,
			"head":   map[string]any{"sha": "abc123"},
		},
		"repository": map[string]any{
			"full_name": "acme/demo",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewReader(body))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signPayload("secret", body))
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if !stub.called {
		t.Fatalf("expected enqueuer to be called")
	}
}

func TestAnalyzePRInvalidJSON(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, "")
	req := httptest.NewRequest(http.MethodPost, "/analyze/pr", bytes.NewBufferString("not-json"))
	res := httptest.NewRecorder()

	handlers.AnalyzePR(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestAnalyzePRAcceptsRequest(t *testing.T) {
	stub := &stubEnqueuer{}
	handlers := NewHandlers(stub, "")

	payload := map[string]any{
		"repository":  "acme/demo",
		"pull_number": 99,
		"commit_sha":  "deadbeef",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/analyze/pr", bytes.NewReader(body))
	res := httptest.NewRecorder()

	handlers.AnalyzePR(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if !stub.called {
		t.Fatalf("expected enqueuer to be called")
	}
}

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
