package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://api.github.com"

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type PullRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	User  struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type ReviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Body string `json:"body"`
	Side string `json:"side"`
}

func NewClient(token string) *Client {
	if token == "" {
		return nil
	}
	return &Client{
		baseURL: defaultBaseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) FetchPullRequest(ctx context.Context, repo string, number int) (PullRequest, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return PullRequest{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	body, status, err := c.do(req)
	if err != nil {
		return PullRequest{}, err
	}
	if status >= 300 {
		return PullRequest{}, fmt.Errorf("github pull request fetch failed: %s", body)
	}

	var pr PullRequest
	if err := json.Unmarshal([]byte(body), &pr); err != nil {
		return PullRequest{}, err
	}
	return pr, nil
}

func (c *Client) FetchPullRequestDiff(ctx context.Context, repo string, number int) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, repo, number)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	req.Header.Set("Authorization", "Bearer "+c.token)

	body, status, err := c.do(req)
	if err != nil {
		return "", err
	}
	if status >= 300 {
		return "", fmt.Errorf("github pull request diff fetch failed: %s", body)
	}
	return body, nil
}

func (c *Client) CreatePullRequestReview(ctx context.Context, repo string, number int, commitSHA string, body string, comments []ReviewComment) error {
	if c == nil {
		return fmt.Errorf("github client is not configured")
	}
	url := fmt.Sprintf("%s/repos/%s/pulls/%d/reviews", c.baseURL, repo, number)
	payload := map[string]any{
		"body":      body,
		"event":     "COMMENT",
		"commit_id": commitSHA,
		"comments":  comments,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	response, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("github review creation failed: %s", response)
	}
	return nil
}

func (c *Client) do(req *http.Request) (string, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(payload), resp.StatusCode, nil
}
