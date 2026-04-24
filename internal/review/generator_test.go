package review

import (
	"strings"
	"testing"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

func TestGenerateNoIssues(t *testing.T) {
	result := Generate(nil)
	if result.Summary != "✅ No issues detected by automated checks." {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
	if len(result.Comments) != 0 {
		t.Errorf("expected no comments, got %d", len(result.Comments))
	}
}

func TestGenerateInlineIssues(t *testing.T) {
	issues := []analysis.Issue{
		{File: "main.go", Line: 10, RuleID: "todo", Severity: "medium", Message: "TODO marker."},
		{File: "auth.go", Line: 5, RuleID: "secrets", Severity: "high", Message: "Possible secret."},
	}
	result := Generate(issues)

	if !strings.Contains(result.Summary, "High: 1") {
		t.Errorf("summary missing high count: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "Medium: 1") {
		t.Errorf("summary missing medium count: %q", result.Summary)
	}
	if strings.Contains(result.Summary, "File-level findings") {
		t.Errorf("summary should not contain file-level section for inline-only issues")
	}
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(result.Comments))
	}
}

func TestGenerateFileLevelIssues(t *testing.T) {
	issues := []analysis.Issue{
		{File: "service.go", Line: 0, RuleID: "large-diff", Severity: "low", Message: "Large diff detected."},
	}
	result := Generate(issues)

	if !strings.Contains(result.Summary, "Low: 1") {
		t.Errorf("summary missing low count: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "File-level findings") {
		t.Errorf("summary missing file-level section: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "`service.go`") {
		t.Errorf("summary missing file name: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "**large-diff**") {
		t.Errorf("summary missing rule ID: %q", result.Summary)
	}
	if len(result.Comments) != 0 {
		t.Errorf("expected no inline comments for file-level issues, got %d", len(result.Comments))
	}
}

func TestGenerateMixedIssues(t *testing.T) {
	issues := []analysis.Issue{
		{File: "main.go", Line: 42, RuleID: "todo", Severity: "medium", Message: "TODO marker."},
		{File: "big.go", Line: 0, RuleID: "large-diff", Severity: "low", Message: "Large diff."},
		{File: "auth.go", Line: 7, RuleID: "secrets", Severity: "high", Message: "Possible secret."},
	}
	result := Generate(issues)

	if !strings.Contains(result.Summary, "High: 1, Medium: 1, Low: 1") {
		t.Errorf("unexpected severity counts in summary: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "File-level findings") {
		t.Errorf("summary missing file-level section: %q", result.Summary)
	}
	if !strings.Contains(result.Summary, "`big.go`") {
		t.Errorf("summary missing large-diff file: %q", result.Summary)
	}
	if len(result.Comments) != 2 {
		t.Fatalf("expected 2 inline comments, got %d", len(result.Comments))
	}
}

func TestGenerateFileLevelIssuesSortedByFile(t *testing.T) {
	issues := []analysis.Issue{
		{File: "z.go", Line: 0, RuleID: "large-diff", Severity: "low", Message: "Z file large."},
		{File: "a.go", Line: 0, RuleID: "large-diff", Severity: "low", Message: "A file large."},
	}
	result := Generate(issues)

	idxA := strings.Index(result.Summary, "`a.go`")
	idxZ := strings.Index(result.Summary, "`z.go`")
	if idxA > idxZ {
		t.Errorf("file-level issues not sorted alphabetically: %q", result.Summary)
	}
}

func TestGenerateSkipsIssuesWithNoFile(t *testing.T) {
	issues := []analysis.Issue{
		{File: "", Line: 0, RuleID: "unknown", Severity: "high", Message: "No location."},
		{File: "real.go", Line: 5, RuleID: "todo", Severity: "medium", Message: "TODO."},
	}
	result := Generate(issues)

	// The no-file issue is counted in the severity totals but produces no comment and no file-level entry.
	if !strings.Contains(result.Summary, "High: 1") {
		t.Errorf("no-file issue should still be counted: %q", result.Summary)
	}
	if strings.Contains(result.Summary, "File-level findings") {
		t.Errorf("no-file issue should not create a file-level section: %q", result.Summary)
	}
	if len(result.Comments) != 1 {
		t.Fatalf("expected 1 inline comment, got %d", len(result.Comments))
	}
}

func TestGenerateInlineCommentsSortedByFileAndLine(t *testing.T) {
	issues := []analysis.Issue{
		{File: "b.go", Line: 3, RuleID: "r", Severity: "low", Message: "B3"},
		{File: "a.go", Line: 10, RuleID: "r", Severity: "low", Message: "A10"},
		{File: "a.go", Line: 2, RuleID: "r", Severity: "low", Message: "A2"},
	}
	result := Generate(issues)

	if len(result.Comments) != 3 {
		t.Fatalf("expected 3 comments, got %d", len(result.Comments))
	}
	if result.Comments[0].Path != "a.go" || result.Comments[0].Line != 2 {
		t.Errorf("first comment: got %s:%d", result.Comments[0].Path, result.Comments[0].Line)
	}
	if result.Comments[1].Path != "a.go" || result.Comments[1].Line != 10 {
		t.Errorf("second comment: got %s:%d", result.Comments[1].Path, result.Comments[1].Line)
	}
	if result.Comments[2].Path != "b.go" || result.Comments[2].Line != 3 {
		t.Errorf("third comment: got %s:%d", result.Comments[2].Path, result.Comments[2].Line)
	}
}
