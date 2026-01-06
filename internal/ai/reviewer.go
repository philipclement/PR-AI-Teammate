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

	prompt := buildPrompt(input.Title, input.Body, diff)
	request := chatCompletionRequest{
		Model: r.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a senior software engineer performing a code review."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
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

	return nil, strings.TrimSpace(response.Choices[0].Message.Content), nil
}

type ReviewInput struct {
	Title string
	Body  string
	Diff  string
}

type chatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
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

func buildPrompt(title string, body string, diff string) string {
	return fmt.Sprintf(`Review this PR for architectural concerns, performance risks, security issues, maintainability, and API design.

For each issue:
- Explain why it matters
- Suggest a concrete improvement
- Reference specific lines when possible

PR Title: %s
PR Description: %s

Diff:
%s`, title, body, diff)
}
