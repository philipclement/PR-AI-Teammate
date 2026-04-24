package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// openAIResponse builds a minimal chat completion response whose content is the
// JSON-encoded value of inner.
func openAIResponse(t *testing.T, inner any) string {
	t.Helper()
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		t.Fatalf("marshal inner: %v", err)
	}
	outer := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"role":    "assistant",
					"content": string(innerJSON),
				},
			},
		},
	}
	b, err := json.Marshal(outer)
	if err != nil {
		t.Fatalf("marshal outer: %v", err)
	}
	return string(b)
}

func mockServer(t *testing.T, statusCode int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		fmt.Fprint(w, body)
	}))
}

func TestReviewEmptyAPIKey(t *testing.T) {
	r := NewReviewer("", "", "")
	issues, summary, err := r.Review(context.Background(), ReviewInput{Title: "t", Body: "b", Diff: "d"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues != nil || summary != "" {
		t.Fatalf("expected nil issues and empty summary, got %v / %q", issues, summary)
	}
}

func TestReviewParsesStructuredIssues(t *testing.T) {
	inner := aiReviewResponse{
		Summary: "Looks good overall with one concern.",
		Issues: []aiIssue{
			{File: "internal/auth/handler.go", Line: 42, Severity: "high", Message: "Missing error check on token parse."},
			{File: "internal/db/query.go", Line: 17, Severity: "medium", Message: "N+1 query pattern; consider batching."},
		},
	}
	srv := mockServer(t, http.StatusOK, openAIResponse(t, inner))
	defer srv.Close()

	reviewer := NewReviewer("test-key", srv.URL, "gpt-4o-mini")
	issues, summary, err := reviewer.Review(context.Background(), ReviewInput{
		Title: "Add auth middleware",
		Body:  "Adds JWT validation",
		Diff:  "--- a/internal/auth/handler.go\n+++ b/internal/auth/handler.go\n@@ -40,3 +40,5 @@",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != inner.Summary {
		t.Fatalf("summary: got %q, want %q", summary, inner.Summary)
	}
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}

	if issues[0].File != "internal/auth/handler.go" {
		t.Errorf("issue[0].File: got %q", issues[0].File)
	}
	if issues[0].Line != 42 {
		t.Errorf("issue[0].Line: got %d", issues[0].Line)
	}
	if issues[0].Severity != "high" {
		t.Errorf("issue[0].Severity: got %q", issues[0].Severity)
	}
	if issues[0].RuleID != "ai-review" {
		t.Errorf("issue[0].RuleID: got %q, want \"ai-review\"", issues[0].RuleID)
	}

	if issues[1].Severity != "medium" {
		t.Errorf("issue[1].Severity: got %q", issues[1].Severity)
	}
}

func TestReviewFiltersInvalidIssues(t *testing.T) {
	inner := aiReviewResponse{
		Summary: "Some issues.",
		Issues: []aiIssue{
			{File: "", Line: 5, Severity: "high", Message: "No file."},    // filtered: empty file
			{File: "foo.go", Line: 0, Severity: "low", Message: "No line."}, // filtered: line 0
			{File: "bar.go", Line: 3, Severity: "medium", Message: "Valid."},
		},
	}
	srv := mockServer(t, http.StatusOK, openAIResponse(t, inner))
	defer srv.Close()

	reviewer := NewReviewer("test-key", srv.URL, "gpt-4o-mini")
	issues, _, err := reviewer.Review(context.Background(), ReviewInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 valid issue, got %d", len(issues))
	}
	if issues[0].File != "bar.go" {
		t.Errorf("unexpected file: %q", issues[0].File)
	}
}

func TestReviewNormalizesUnknownSeverity(t *testing.T) {
	inner := aiReviewResponse{
		Summary: "ok",
		Issues:  []aiIssue{{File: "a.go", Line: 1, Severity: "critical", Message: "bad"}},
	}
	srv := mockServer(t, http.StatusOK, openAIResponse(t, inner))
	defer srv.Close()

	reviewer := NewReviewer("test-key", srv.URL, "gpt-4o-mini")
	issues, _, err := reviewer.Review(context.Background(), ReviewInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues[0].Severity != "medium" {
		t.Errorf("expected severity normalised to \"medium\", got %q", issues[0].Severity)
	}
}

func TestReviewFallsBackOnInvalidJSON(t *testing.T) {
	// Model returns plain text instead of JSON (e.g. non-OpenAI-compatible API).
	rawText := "This PR looks fine but watch out for N+1 queries on line 12."
	outer := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{"role": "assistant", "content": rawText},
			},
		},
	}
	b, _ := json.Marshal(outer)
	srv := mockServer(t, http.StatusOK, string(b))
	defer srv.Close()

	reviewer := NewReviewer("test-key", srv.URL, "gpt-4o-mini")
	issues, summary, err := reviewer.Review(context.Background(), ReviewInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issues != nil {
		t.Errorf("expected nil issues on JSON fallback, got %v", issues)
	}
	if summary != rawText {
		t.Errorf("summary: got %q, want %q", summary, rawText)
	}
}

func TestReviewAPIError(t *testing.T) {
	errorBody := `{"error":{"message":"invalid api key"}}`
	srv := mockServer(t, http.StatusUnauthorized, errorBody)
	defer srv.Close()

	reviewer := NewReviewer("bad-key", srv.URL, "gpt-4o-mini")
	_, _, err := reviewer.Review(context.Background(), ReviewInput{})
	if err == nil {
		t.Fatal("expected error for non-2xx status")
	}
}

func TestReviewNoChoices(t *testing.T) {
	srv := mockServer(t, http.StatusOK, `{"choices":[]}`)
	defer srv.Close()

	reviewer := NewReviewer("test-key", srv.URL, "gpt-4o-mini")
	_, _, err := reviewer.Review(context.Background(), ReviewInput{})
	if err == nil {
		t.Fatal("expected error when choices is empty")
	}
}

func TestNormalizeSeverity(t *testing.T) {
	cases := []struct{ in, want string }{
		{"high", "high"},
		{"HIGH", "high"},
		{"low", "low"},
		{"LOW", "low"},
		{"medium", "medium"},
		{"critical", "medium"},
		{"", "medium"},
		{"warning", "medium"},
	}
	for _, c := range cases {
		if got := normalizeSeverity(c.in); got != c.want {
			t.Errorf("normalizeSeverity(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
