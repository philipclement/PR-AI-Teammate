package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

type Reviewer struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewReviewer(apiKey string, baseURL string, model string) *Reviewer {
	trimmedKey := strings.TrimSpace(apiKey)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Reviewer{
		apiKey:  trimmedKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (r *Reviewer) Review(ctx context.Context, input ReviewInput) ([]analysis.Issue, string, error) {
	if r.apiKey == "" {
		return nil, "", nil
	}

	diff := input.Diff
	if len(diff) > 8000 {
		diff = diff[:8000] + "\n...diff truncated..."
	}

	request := chatCompletionRequest{
		Model: r.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a senior software engineer performing a code review. You must respond with valid JSON."},
			{Role: "user", Content: buildPrompt(input.Title, input.Body, diff)},
		},
		Temperature:    0.2,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, "", err
	}

	url := fmt.Sprintf("%s/chat/completions", r.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	var response chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, "", err
	}
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("ai reviewer request failed: %s", response.Error.Message)
	}
	if len(response.Choices) == 0 {
		return nil, "", fmt.Errorf("ai reviewer returned no choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)

	var parsed aiReviewResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// Model didn't return valid JSON; treat the whole response as a summary with no inline issues.
		return nil, content, nil
	}

	var issues []analysis.Issue
	for _, item := range parsed.Issues {
		if item.File == "" || item.Line <= 0 {
			continue
		}
		issues = append(issues, analysis.Issue{
			File:     item.File,
			Line:     item.Line,
			RuleID:   "ai-review",
			Severity: normalizeSeverity(item.Severity),
			Message:  item.Message,
		})
	}

	return issues, parsed.Summary, nil
}

type ReviewInput struct {
	Title string
	Body  string
	Diff  string
}

// aiReviewResponse is the JSON schema we ask the model to return.
type aiReviewResponse struct {
	Summary string    `json:"summary"`
	Issues  []aiIssue `json:"issues"`
}

type aiIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float32         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func buildPrompt(title, body, diff string) string {
	return fmt.Sprintf(`Review this pull request and return your findings as JSON.

For each inline issue, provide:
- "file": exact file path from the diff header (e.g. "internal/foo/bar.go")
- "line": line number in the new file — use the +N value from the @@ hunk header as your starting point
- "severity": one of "high", "medium", or "low"
- "message": concise explanation of the issue and a concrete improvement suggestion

Also provide a "summary" covering architectural concerns, performance risks, security issues,
maintainability, and API design.

Respond with ONLY this JSON (no markdown fences, no extra text):
{
  "summary": "...",
  "issues": [
    {"file": "...", "line": 0, "severity": "...", "message": "..."}
  ]
}

PR Title: %s
PR Description: %s

Diff:
%s`, title, body, diff)
}

func normalizeSeverity(s string) string {
	switch strings.ToLower(s) {
	case "high":
		return "high"
	case "low":
		return "low"
	default:
		return "medium"
	}
}
