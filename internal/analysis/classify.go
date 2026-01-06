package analysis

import "strings"

func ClassifyPath(path string) FileType {
	lower := strings.ToLower(path)
	switch {
	case strings.Contains(lower, "/test/") || strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, ".spec.ts") || strings.HasSuffix(lower, ".test.ts"):
		return FileTypeTest
	case strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".json") || strings.Contains(lower, "/config/"):
		return FileTypeConfig
	default:
		return FileTypeProd
	}
}
