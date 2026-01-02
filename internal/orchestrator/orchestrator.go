package orchestrator

import (
	"context"
	"fmt"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
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

	// TODO: fetch PR diff, run rule engine, static analysis, and AI reviewer.
	return AnalyzeResult{Summary: "analysis queued"}, nil
}
