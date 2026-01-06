package rules

import (
	"strings"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

type TodoRule struct{}

func (TodoRule) ID() string          { return "todo" }
func (TodoRule) Description() string { return "Flags TODO/FIXME markers in production code." }

func (TodoRule) Check(file analysis.FileDiff) []analysis.Issue {
	if file.Type == analysis.FileTypeTest {
		return nil
	}
	var issues []analysis.Issue
	for _, line := range file.AddedLines {
		if strings.Contains(line.Content, "TODO") || strings.Contains(line.Content, "FIXME") {
			issues = append(issues, analysis.Issue{
				File:     file.Path,
				Line:     line.Number,
				RuleID:   "todo",
				Severity: "medium",
				Message:  "TODO/FIXME marker added to production code.",
			})
		}
	}
	return issues
}

type SecretRule struct{}

func (SecretRule) ID() string          { return "secrets" }
func (SecretRule) Description() string { return "Flags potential secrets in added lines." }

func (SecretRule) Check(file analysis.FileDiff) []analysis.Issue {
	var issues []analysis.Issue
	for _, line := range file.AddedLines {
		lower := strings.ToLower(line.Content)
		if strings.Contains(lower, "password") || strings.Contains(lower, "secret") || strings.Contains(lower, "api_key") {
			issues = append(issues, analysis.Issue{
				File:     file.Path,
				Line:     line.Number,
				RuleID:   "secrets",
				Severity: "high",
				Message:  "Possible secret detected in added line.",
			})
		}
	}
	return issues
}

type LargeDiffRule struct {
	Threshold int
}

func (LargeDiffRule) ID() string          { return "large-diff" }
func (LargeDiffRule) Description() string { return "Flags files with a large number of added lines." }

func (r LargeDiffRule) Check(file analysis.FileDiff) []analysis.Issue {
	if r.Threshold <= 0 || len(file.AddedLines) <= r.Threshold {
		return nil
	}
	return []analysis.Issue{
		{
			File:     file.Path,
			Line:     0,
			RuleID:   "large-diff",
			Severity: "low",
			Message:  "Large diff detected; consider splitting into smaller changes.",
		},
	}
}
