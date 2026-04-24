package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/pr-ai-teammate/internal/orchestrator"
)

// ── stubs ─────────────────────────────────────────────────────────────────────

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

type stubFeedbackRecorder struct {
	called   bool
	lastRepo string
	lastRule string
	lastAcc  bool
	err      error
}

func (s *stubFeedbackRecorder) RecordFeedback(_ context.Context, repo, ruleID string, accepted bool) error {
	s.called = true
	s.lastRepo = repo
	s.lastRule = ruleID
	s.lastAcc = accepted
	return s.err
}

// ── health ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
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

// ── webhook ───────────────────────────────────────────────────────────────────

func TestWebhookGitHubMissingHeader(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", nil)
	res := httptest.NewRecorder()

	handlers.WebhookGitHub(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestWebhookGitHubIgnoredEvent(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
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
	handlers := NewHandlers(stub, nil, "")

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
	handlers := NewHandlers(stub, nil, "")

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
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
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
	handlers := NewHandlers(&stubEnqueuer{}, nil, "secret")
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
	handlers := NewHandlers(stub, nil, "secret")

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

// ── analyze ───────────────────────────────────────────────────────────────────

func TestAnalyzePRInvalidJSON(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/analyze/pr", bytes.NewBufferString("not-json"))
	res := httptest.NewRecorder()

	handlers.AnalyzePR(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestAnalyzePRAcceptsRequest(t *testing.T) {
	stub := &stubEnqueuer{}
	handlers := NewHandlers(stub, nil, "")

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

// ── feedback ──────────────────────────────────────────────────────────────────

func TestFeedbackNotConfigured(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, nil, "")
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewBufferString(`{}`))
	res := httptest.NewRecorder()

	handlers.Feedback(res, req)

	if res.Code != http.StatusNotImplemented {
		t.Fatalf("expected status 501, got %d", res.Code)
	}
}

func TestFeedbackInvalidJSON(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, &stubFeedbackRecorder{}, "")
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewBufferString("not-json"))
	res := httptest.NewRecorder()

	handlers.Feedback(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestFeedbackMissingFields(t *testing.T) {
	handlers := NewHandlers(&stubEnqueuer{}, &stubFeedbackRecorder{}, "")

	cases := []string{
		`{"rule_id":"secrets","accepted":true}`,         // missing repository
		`{"repository":"acme/demo","accepted":false}`,   // missing rule_id
		`{}`,                                            // missing both
	}
	for _, body := range cases {
		req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewBufferString(body))
		res := httptest.NewRecorder()
		handlers.Feedback(res, req)
		if res.Code != http.StatusBadRequest {
			t.Errorf("body %q: expected 400, got %d", body, res.Code)
		}
	}
}

func TestFeedbackRecordsAccepted(t *testing.T) {
	stub := &stubFeedbackRecorder{}
	handlers := NewHandlers(&stubEnqueuer{}, stub, "")

	body, _ := json.Marshal(map[string]any{
		"repository": "acme/demo",
		"rule_id":    "secrets",
		"accepted":   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(body))
	res := httptest.NewRecorder()

	handlers.Feedback(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if !stub.called {
		t.Fatal("expected feedback recorder to be called")
	}
	if stub.lastRepo != "acme/demo" {
		t.Errorf("unexpected repo: %q", stub.lastRepo)
	}
	if stub.lastRule != "secrets" {
		t.Errorf("unexpected rule: %q", stub.lastRule)
	}
	if !stub.lastAcc {
		t.Error("expected accepted=true")
	}
}

func TestFeedbackRecordsRejected(t *testing.T) {
	stub := &stubFeedbackRecorder{}
	handlers := NewHandlers(&stubEnqueuer{}, stub, "")

	body, _ := json.Marshal(map[string]any{
		"repository": "acme/demo",
		"rule_id":    "todo",
		"accepted":   false,
	})
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(body))
	res := httptest.NewRecorder()

	handlers.Feedback(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.Code)
	}
	if stub.lastAcc {
		t.Error("expected accepted=false")
	}
}

func TestFeedbackStorageError(t *testing.T) {
	stub := &stubFeedbackRecorder{err: fmt.Errorf("db connection lost")}
	handlers := NewHandlers(&stubEnqueuer{}, stub, "")

	body, _ := json.Marshal(map[string]any{
		"repository": "acme/demo",
		"rule_id":    "secrets",
		"accepted":   true,
	})
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(body))
	res := httptest.NewRecorder()

	handlers.Feedback(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", res.Code)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}