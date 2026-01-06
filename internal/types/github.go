package types

import "strings"

type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
	Repository  Repository  `json:"repository"`
}

type PullRequest struct {
	Number int            `json:"number"`
	Head   PullRequestRef `json:"head"`
}

type PullRequestRef struct {
	SHA string `json:"sha"`
}

type Repository struct {
	FullName string `json:"full_name"`
}

func (e PullRequestEvent) IsActionSupported() bool {
	action := strings.ToLower(e.Action)
	return action == "opened" || action == "synchronize" || action == "reopened"
}
