package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/example/pr-ai-teammate/internal/ai"
	"github.com/example/pr-ai-teammate/internal/analysis"
	"github.com/example/pr-ai-teammate/internal/github"
	"github.com/example/pr-ai-teammate/internal/review"
	"github.com/example/pr-ai-teammate/internal/rules"
)

type Service struct {
	githubClient GitHubClient
	reviewer     Reviewer
	rulesEngine  *rules.Engine
	store        Store
}

type GitHubClient interface {
	FetchPullRequest(ctx context.Context, repo string, number int) (github.PullRequest, error)
	FetchPullRequestDiff(ctx context.Context, repo string, number int) (string, error)
	FetchFileContent(ctx context.Context, repo string, path string, ref string) (string, error)
	CreatePullRequestReview(ctx context.Context, repo string, number int, commitSHA string, body string, comments []github.ReviewComment) error
}

type Reviewer interface {
	Review(ctx context.Context, input ai.ReviewInput) ([]analysis.Issue, string, error)
}

type Store interface {
	UpsertPullRequest(ctx context.Context, repo string, number int, sha string, title string, status string) (int64, error)
	UpdatePullRequestStatus(ctx context.Context, id int64, status string) error
	SaveAnalysisResults(ctx context.Context, prID int64, issues []analysis.Issue) error
}

func NewService(githubClient GitHubClient, reviewer Reviewer, store Store) *Service {
	return &Service{
		githubClient: githubClient,
		reviewer:     reviewer,
		rulesEngine:  rules.NewDefaultEngine(),
		store:        store,
	}
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

	files, err := analysis.ParseUnifiedDiff(diff)
	if err != nil {
		return AnalyzeResult{}, err
	}

	prID := int64(0)
	if s.store != nil {
		storedID, err := s.store.UpsertPullRequest(ctx, input.Repository, input.PullNumber, input.CommitSHA, pr.Title, "processing")
		if err != nil {
			return AnalyzeResult{}, err
		}
		prID = storedID
	}

	contents := map[string]string{}
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Path), ".go") {
			body, err := s.githubClient.FetchFileContent(ctx, input.Repository, file.Path, input.CommitSHA)
			if err != nil {
				return AnalyzeResult{}, err
			}
			contents[file.Path] = body
		}
	}

	issues := s.rulesEngine.Run(files)
	issues = append(issues, analysis.RunStaticAnalysis(files, contents)...)

	aiSummary := ""
	if s.reviewer != nil {
		aiIssues, summary, err := s.reviewer.Review(ctx, ai.ReviewInput{
			Title: pr.Title,
			Body:  pr.Body,
			Diff:  diff,
		})
		if err != nil {
			return AnalyzeResult{}, err
		}
		issues = append(issues, aiIssues...)
		aiSummary = summary
	}

	reviewResult := review.Generate(issues)
	if aiSummary != "" {
		reviewResult.Summary = fmt.Sprintf("%s\n\n%s", reviewResult.Summary, aiSummary)
	}

	var comments []github.ReviewComment
	for _, comment := range reviewResult.Comments {
		comments = append(comments, github.ReviewComment{
			Path: comment.Path,
			Line: comment.Line,
			Body: comment.Body,
			Side: "RIGHT",
		})
	}

	if s.store != nil && prID != 0 {
		if err := s.store.SaveAnalysisResults(ctx, prID, issues); err != nil {
			return AnalyzeResult{}, err
		}
	}

	if err := s.githubClient.CreatePullRequestReview(ctx, input.Repository, input.PullNumber, input.CommitSHA, reviewResult.Summary, comments); err != nil {
		return AnalyzeResult{}, err
	}

	if s.store != nil && prID != 0 {
		if err := s.store.UpdatePullRequestStatus(ctx, prID, "reviewed"); err != nil {
			return AnalyzeResult{}, err
		}
	}

	summary := fmt.Sprintf("analysis completed for %s#%d (%s) with %d diff bytes",
		input.Repository,
		input.PullNumber,
		strings.TrimSpace(pr.Title),
		len(diff),
	)
	return AnalyzeResult{Summary: summary}, nil
}
