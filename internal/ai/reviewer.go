package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

type Reviewer struct {
	apiKey string
}

func NewReviewer(apiKey string) *Reviewer {
	return &Reviewer{apiKey: strings.TrimSpace(apiKey)}
}

func (r *Reviewer) Review(ctx context.Context, input ReviewInput) ([]analysis.Issue, string, error) {
	if r.apiKey == "" {
		return nil, "AI reviewer not configured", nil
	}
	return nil, "AI review placeholder (not yet implemented)", nil
}

type ReviewInput struct {
	Title string
	Body  string
	Diff  string
}
