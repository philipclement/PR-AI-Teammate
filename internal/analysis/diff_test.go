package analysis

import (
	"strings"
	"testing"
)

func TestParseUnifiedDiff(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"index 111..222 100644",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,2 +1,3 @@",
		" package main",
		"+// TODO: add handler",
		" func main() {}",
		"diff --git a/config/app.yaml b/config/app.yaml",
		"index 333..444 100644",
		"--- a/config/app.yaml",
		"+++ b/config/app.yaml",
		"@@ -1 +1 @@",
		"-old: value",
		"+new: value",
	}, "\n")

	files, err := ParseUnifiedDiff(diff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != "main.go" {
		t.Fatalf("expected first file path main.go, got %q", files[0].Path)
	}
	if len(files[0].AddedLines) != 1 {
		t.Fatalf("expected 1 added line, got %d", len(files[0].AddedLines))
	}
	if files[1].Type != FileTypeConfig {
		t.Fatalf("expected config file type, got %s", files[1].Type)
	}
}
