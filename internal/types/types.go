package types

type AnalyzeRequest struct {
	Repository string `json:"repository"`
	PullNumber int    `json:"pull_number"`
	CommitSHA  string `json:"commit_sha"`
}

type AnalyzeResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type WebhookResponse struct {
	Status string `json:"status"`
}
