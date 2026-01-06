package analysis

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

func RunStaticAnalysis(files []FileDiff, contents map[string]string) []Issue {
	var issues []Issue
	for _, file := range files {
		if filepath.Ext(file.Path) != ".go" {
			continue
		}
		source, ok := contents[file.Path]
		if !ok || strings.TrimSpace(source) == "" {
			continue
		}
		issues = append(issues, analyzeGoFile(file.Path, source)...)
	}
	return issues
}

func analyzeGoFile(path string, source string) []Issue {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return []Issue{
			{
				File:     path,
				Line:     0,
				RuleID:   "go-parse",
				Severity: "high",
				Message:  "Failed to parse Go file for static analysis.",
			},
		}
	}

	var issues []Issue
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncDecl:
			start := fset.Position(n.Pos()).Line
			end := fset.Position(n.End()).Line
			if end-start+1 > 50 {
				issues = append(issues, Issue{
					File:     path,
					Line:     start,
					RuleID:   "func-length",
					Severity: "medium",
					Message:  "Function exceeds 50 lines; consider refactoring.",
				})
			}
		case *ast.IfStmt:
			if isErrNilCheck(n.Cond) && len(n.Body.List) == 0 {
				line := fset.Position(n.Pos()).Line
				issues = append(issues, Issue{
					File:     path,
					Line:     line,
					RuleID:   "empty-error-check",
					Severity: "high",
					Message:  "Empty error handling block detected.",
				})
			}
		case *ast.CallExpr:
			if ident, ok := n.Fun.(*ast.Ident); ok && ident.Name == "panic" {
				line := fset.Position(n.Pos()).Line
				issues = append(issues, Issue{
					File:     path,
					Line:     line,
					RuleID:   "panic",
					Severity: "medium",
					Message:  "panic call detected; consider returning an error instead.",
				})
			}
		}
		return true
	})

	return issues
}

func isErrNilCheck(expr ast.Expr) bool {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok || binary.Op != token.NEQ {
		return false
	}
	left, ok := binary.X.(*ast.Ident)
	if !ok || left.Name != "err" {
		return false
	}
	right, ok := binary.Y.(*ast.Ident)
	if !ok || right.Name != "nil" {
		return false
	}
	return true
}
