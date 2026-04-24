package rules

import (
	"testing"

	"github.com/example/pr-ai-teammate/internal/analysis"
)

func fileDiff(path string, fileType analysis.FileType, lines ...string) analysis.FileDiff {
	added := make([]analysis.Line, len(lines))
	for i, content := range lines {
		added[i] = analysis.Line{Number: i + 1, Content: content}
	}
	return analysis.FileDiff{Path: path, Type: fileType, AddedLines: added}
}

// ── SecretRule ────────────────────────────────────────────────────────────────

func TestSecretRuleQuotedAssignment(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("config.go", analysis.FileTypeProd,
		`password = "hunter2secret"`,
		`apiKey := "sk-1234567890abcdef"`,
		`secret: "real-secret-value"`,
	)
	issues := rule.Check(diff)
	if len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %d: %v", len(issues), issues)
	}
	for _, issue := range issues {
		if issue.RuleID != "secrets" || issue.Severity != "high" {
			t.Errorf("unexpected issue: %+v", issue)
		}
	}
}

func TestSecretRuleEnvVarStyle(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff(".env", analysis.FileTypeProd,
		`API_KEY=sk-abcdefghij1234567890`,
		`PASSWORD=mysecretpassword123`,
	)
	issues := rule.Check(diff)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d: %v", len(issues), issues)
	}
}

func TestSecretRuleSkipsCommentLines(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("doc.go", analysis.FileTypeProd,
		`// password = "should_not_flag_this"`,
		`# api_key: "also_not_flagged_here"`,
		`* secret = "star_comment_not_flagged"`,
	)
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for comment lines, got %d: %v", len(issues), issues)
	}
}

func TestSecretRuleSkipsKeywordWithoutAssignment(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("auth.go", analysis.FileTypeProd,
		`func validatePassword(password string) error {`,
		`var apiKeyStore = newStore()`,
		`if err := checkSecret(ctx); err != nil {`,
		`type PasswordHasher struct{}`,
	)
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for keyword-without-assignment, got %d: %v", len(issues), issues)
	}
}

func TestSecretRuleSkipsPlaceholderValues(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("config.go", analysis.FileTypeProd,
		`password = "changeme"`,
		`api_key = "your_api_key_here"`,
		`secret = "replace_with_real_secret"`,
		`token = "example_token_value"`,
		`password = "test_password_123"`,
	)
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for placeholder values, got %d: %v", len(issues), issues)
	}
}

func TestSecretRuleSkipsShortValues(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("config.go", analysis.FileTypeProd,
		`password = ""`,    // empty
		`password = "ab"`,  // 2 chars — below threshold
		`api_key = "key"`,  // 3 chars — below threshold
	)
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues for short/empty values, got %d: %v", len(issues), issues)
	}
}

func TestSecretRuleMessageContainsKeyword(t *testing.T) {
	rule := SecretRule{}
	diff := fileDiff("app.go", analysis.FileTypeProd,
		`api_key = "sk-realvalue1234567"`,
	)
	issues := rule.Check(diff)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	// The message should include the matched keyword so reviewers know what triggered it.
	if issues[0].Message == "" {
		t.Error("issue message should not be empty")
	}
}

func TestSecretRuleChecksTestFiles(t *testing.T) {
	// Unlike TodoRule, SecretRule checks test files too — hardcoded secrets
	// in tests are still a problem if they're real credentials.
	rule := SecretRule{}
	diff := fileDiff("auth_test.go", analysis.FileTypeTest,
		`password = "realpassword1234"`,
	)
	issues := rule.Check(diff)
	if len(issues) != 1 {
		t.Fatalf("expected SecretRule to check test files, got %d issues", len(issues))
	}
}

// ── TodoRule ──────────────────────────────────────────────────────────────────

func TestTodoRuleFlagsProductionCode(t *testing.T) {
	rule := TodoRule{}
	diff := fileDiff("main.go", analysis.FileTypeProd,
		`// TODO: clean this up later`,
		`x := 1 // FIXME: this is broken`,
	)
	issues := rule.Check(diff)
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
}

func TestTodoRuleSkipsTestFiles(t *testing.T) {
	rule := TodoRule{}
	diff := fileDiff("main_test.go", analysis.FileTypeTest,
		`// TODO: add more test cases`,
	)
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues in test files, got %d", len(issues))
	}
}

// ── LargeDiffRule ─────────────────────────────────────────────────────────────

func TestLargeDiffRuleTriggersAboveThreshold(t *testing.T) {
	rule := LargeDiffRule{Threshold: 3}
	lines := make([]string, 4)
	for i := range lines {
		lines[i] = "line"
	}
	diff := fileDiff("big.go", analysis.FileTypeProd, lines...)
	issues := rule.Check(diff)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Line != 0 {
		t.Errorf("large-diff issue should have Line 0 (file-level), got %d", issues[0].Line)
	}
}

func TestLargeDiffRuleNoTriggerAtThreshold(t *testing.T) {
	rule := LargeDiffRule{Threshold: 3}
	diff := fileDiff("small.go", analysis.FileTypeProd, "a", "b", "c")
	issues := rule.Check(diff)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues at threshold, got %d", len(issues))
	}
}

// ── matchSecret / isPlaceholder helpers ──────────────────────────────────────

func TestMatchSecretQuoted(t *testing.T) {
	cases := []struct {
		line    string
		wantOK  bool
		wantKey string
	}{
		{`password = "hunter2secret"`, true, "password"},
		{`api_key: "sk-real1234567890"`, true, "api_key"},
		{`private_key = "-----BEGIN RSA"`, true, "private_key"},
		{`var passwordHasher = bcrypt.New()`, false, ""},
		{`// secret = "should_be_skipped"`, true, "secret"}, // matchSecret matches; Check() caller skips comment lines
		{`password = ""`, false, ""},
		{`password = "abc"`, false, ""},
	}
	for _, c := range cases {
		key, _, ok := matchSecret(c.line)
		if ok != c.wantOK {
			t.Errorf("matchSecret(%q): got ok=%v, want %v", c.line, ok, c.wantOK)
			continue
		}
		if c.wantOK && key != c.wantKey {
			t.Errorf("matchSecret(%q): got keyword=%q, want %q", c.line, key, c.wantKey)
		}
	}
}

func TestIsPlaceholder(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"changeme", true},
		{"your_api_key", true},
		{"<your_secret>", true},
		{"example_value", true},
		{"test_password", true},
		{"sk-1234567890abcdef", false},
		{"hunter2password!", false},
		{"realSecretValue99", false},
	}
	for _, c := range cases {
		if got := isPlaceholder(c.value); got != c.want {
			t.Errorf("isPlaceholder(%q) = %v, want %v", c.value, got, c.want)
		}
	}
}
