package rules

import "github.com/example/pr-ai-teammate/internal/analysis"

type Rule interface {
	ID() string
	Description() string
	Check(file analysis.FileDiff) []analysis.Issue
}

type Engine struct {
	rules []Rule
}

func NewDefaultEngine() *Engine {
	return &Engine{
		rules: []Rule{
			TodoRule{},
			SecretRule{},
			LargeDiffRule{Threshold: 200},
		},
	}
}

func (e *Engine) Run(files []analysis.FileDiff) []analysis.Issue {
	var issues []analysis.Issue
	for _, file := range files {
		for _, rule := range e.rules {
			issues = append(issues, rule.Check(file)...)
		}
	}
	return issues
}
