package rules

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

// ── TodoRule ──────────────────────────────────────────────────────────────────

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

// ── SecretRule ────────────────────────────────────────────────────────────────

// secretQuoted matches lines where a sensitive keyword is to the LEFT of an
// assignment operator followed by a quoted string of 4+ characters.
// Examples caught:  password = "hunter2"   api_key: "sk-abc123def"
// Examples skipped: // password should be hashed   var passwordHasher = ...
var secretQuoted = regexp.MustCompile(
	`(?i)\b(password|passwd|secret|api[_-]?key|auth[_-]?token|private[_-]?key|access[_-]?token)\b\s*[:=]+\s*["']([^"']{4,})["']`,
)

// secretEnvVar matches env-var-style assignments at the start of a line where
// the value is 6+ non-whitespace characters (covers .env files and shell exports).
// Example: API_KEY=sk-1234567890abcdef
var secretEnvVar = regexp.MustCompile(
	`(?i)^(password|passwd|secret|api[_-]?key|auth[_-]?token|private[_-]?key|access[_-]?token)\s*=\s*([^\s"'#;]{6,})`,
)

// commentLine matches lines that are purely a comment (leading //, #, or *).
var commentLine = regexp.MustCompile(`^\s*(?://|#|\*)`)

// placeholderVal matches values that are clearly placeholders rather than real
// credentials, so they don't trigger false positives.
var placeholderVal = regexp.MustCompile(
	`(?i)^(change|changeme|your_|<|{|%s|%v|replace|example|test_|fake|mock|dummy|placeholder|todo|xxx|password|secret)`,
)

type SecretRule struct{}

func (SecretRule) ID() string          { return "secrets" }
func (SecretRule) Description() string { return "Flags potential hardcoded secrets in added lines." }

func (SecretRule) Check(file analysis.FileDiff) []analysis.Issue {
	var issues []analysis.Issue
	for _, line := range file.AddedLines {
		if commentLine.MatchString(line.Content) {
			continue
		}
		keyword, value, ok := matchSecret(line.Content)
		if !ok || isPlaceholder(value) {
			continue
		}
		issues = append(issues, analysis.Issue{
			File:     file.Path,
			Line:     line.Number,
			RuleID:   "secrets",
			Severity: "high",
			Message:  fmt.Sprintf("Possible hardcoded secret (matched keyword %q). Use environment variables or a secrets manager instead.", keyword),
		})
	}
	return issues
}

// matchSecret returns the keyword and value if the line contains a sensitive
// keyword assignment pattern.
func matchSecret(content string) (keyword, value string, ok bool) {
	if m := secretQuoted.FindStringSubmatch(content); m != nil {
		return m[1], m[2], true
	}
	if m := secretEnvVar.FindStringSubmatch(content); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}

// isPlaceholder reports whether a matched value is an obvious placeholder that
// should not trigger a secrets alert.
func isPlaceholder(value string) bool {
	return placeholderVal.MatchString(value)
}

// ── LargeDiffRule ─────────────────────────────────────────────────────────────

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
