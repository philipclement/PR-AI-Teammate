package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/pr-ai-teammate/internal/github"
)

type Service struct {
	githubClient *github.Client
}

func NewService(githubClient *github.Client) *Service {
	return &Service{githubClient: githubClient}
}

type AnalyzeInput struct {
	Repository string
	PullNumber int
	CommitSHA  string
}

type AnalyzeResult struct {
	Summary string
}

func (s *Service) AnalyzePR(ctx context.Context, input AnalyzeInput) (AnalyzeResult, error) {
	if input.Repository == "" {
		return AnalyzeResult{}, fmt.Errorf("repository is required")
	}
	if input.PullNumber == 0 {
		return AnalyzeResult{}, fmt.Errorf("pull number is required")
	}
	if input.CommitSHA == "" {
		return AnalyzeResult{}, fmt.Errorf("commit SHA is required")
	}

	if s.githubClient == nil {
		return AnalyzeResult{Summary: "analysis queued (no github client configured)"}, nil
	}

	pr, err := s.githubClient.FetchPullRequest(ctx, input.Repository, input.PullNumber)
	if err != nil {
		return AnalyzeResult{}, err
	}
	diff, err := s.githubClient.FetchPullRequestDiff(ctx, input.Repository, input.PullNumber)
	if err != nil {
		return AnalyzeResult{}, err
	}

	// TODO: split diff by file, run rule engine, static analysis, and AI reviewer.
	summary := fmt.Sprintf("analysis queued for %s#%d (%s) with %d diff bytes",
		input.Repository,
		input.PullNumber,
		strings.TrimSpace(pr.Title),
		len(diff),
	)
	return AnalyzeResult{Summary: summary}, nil
}
