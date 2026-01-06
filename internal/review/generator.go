package review

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

type Comment struct {
	Path string
	Line int
	Body string
}

type Result struct {
	Summary  string
	Comments []Comment
}

func Generate(issues []analysis.Issue) Result {
	if len(issues) == 0 {
		return Result{
			Summary:  "✅ No issues detected by automated checks.",
			Comments: nil,
		}
	}

	severityCounts := map[string]int{}
	for _, issue := range issues {
		severityCounts[issue.Severity]++
	}

	var summaryParts []string
	for _, severity := range []string{"high", "medium", "low"} {
		if count := severityCounts[severity]; count > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("%s: %d", capitalize(severity), count))
		}
	}

	summary := fmt.Sprintf("## Automated Review Summary\n\nIssues detected: %s", strings.Join(summaryParts, ", "))

	comments := make([]Comment, 0, len(issues))
	for _, issue := range issues {
		if issue.File == "" || issue.Line == 0 {
			continue
		}
		body := fmt.Sprintf("**%s**: %s", issue.RuleID, issue.Message)
		comments = append(comments, Comment{
			Path: issue.File,
			Line: issue.Line,
			Body: body,
		})
	}

	sort.Slice(comments, func(i, j int) bool {
		if comments[i].Path == comments[j].Path {
			return comments[i].Line < comments[j].Line
		}
		return comments[i].Path < comments[j].Path
	})

	return Result{
		Summary:  summary,
		Comments: comments,
	}
}

func capitalize(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
