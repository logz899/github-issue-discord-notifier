package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://api.github.com"
	defaultMaxPages = 100
	requestTimeout  = 15 * time.Second
	maxErrorBody    = 512
)

type Issue struct {
	ID            int64
	Number        int
	Title         string
	Body          string
	HTMLURL       string
	State         string
	Author        string
	Labels        []string
	CreatedAt     time.Time
	IsPullRequest bool
}

type Client struct {
	BaseURL    string
	Token      string
	HTTP       *http.Client
	MaxPages   int
	baseOrigin string
}

func NewClient(baseURL, token string, httpClient *http.Client) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	} else if httpClient.Timeout == 0 {
		clone := *httpClient
		clone.Timeout = requestTimeout
		httpClient = &clone
	}
	parsed, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Token:      token,
		HTTP:       httpClient,
		MaxPages:   defaultMaxPages,
		baseOrigin: parsed.Scheme + "://" + parsed.Host,
	}
}

func (client *Client) ListIssues(ctx context.Context, repository string, since *time.Time) ([]Issue, error) {
	requestURL, err := url.Parse(client.BaseURL + "/repos/" + repository + "/issues")
	if err != nil {
		return nil, fmt.Errorf("build GitHub issues URL: %w", err)
	}
	query := requestURL.Query()
	query.Set("state", "open")
	query.Set("sort", "updated")
	query.Set("direction", "asc")
	query.Set("per_page", "100")
	if since != nil {
		query.Set("since", since.UTC().Format(time.RFC3339))
	}
	requestURL.RawQuery = query.Encode()

	maxPages := client.MaxPages
	if maxPages <= 0 {
		maxPages = defaultMaxPages
	}
	visited := make(map[string]struct{}, maxPages)
	issues := make([]Issue, 0)

	for page := 1; ; page++ {
		current := requestURL.String()
		if _, exists := visited[current]; exists {
			return nil, errors.New("GitHub pagination cycle detected")
		}
		visited[current] = struct{}{}

		remoteIssues, next, err := client.fetchPage(ctx, requestURL)
		if err != nil {
			return nil, err
		}
		for _, remote := range remoteIssues {
			issues = append(issues, remote.toIssue())
		}
		if next == "" {
			return issues, nil
		}
		if page >= maxPages {
			return nil, fmt.Errorf("GitHub pagination page limit %d exceeded", maxPages)
		}
		nextURL, err := url.Parse(next)
		if err != nil {
			return nil, fmt.Errorf("parse GitHub pagination URL: %w", err)
		}
		if nextURL.Scheme+"://"+nextURL.Host != client.baseOrigin {
			return nil, errors.New("GitHub pagination URL changed origin")
		}
		requestURL = nextURL
	}
}

func (client *Client) fetchPage(ctx context.Context, pageURL *url.URL) ([]remoteIssue, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create GitHub request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if client.Token != "" {
		request.Header.Set("Authorization", "Bearer "+client.Token)
	}

	response, err := client.HTTP.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("request GitHub issues: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return nil, "", fmt.Errorf("GitHub issues returned %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var issues []remoteIssue
	if err := json.NewDecoder(response.Body).Decode(&issues); err != nil {
		return nil, "", fmt.Errorf("decode GitHub issues response: %w", err)
	}
	return issues, nextLink(response.Header.Get("Link")), nil
}

type remoteIssue struct {
	ID        int64     `json:"id"`
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      *string   `json:"body"`
	HTMLURL   string    `json:"html_url"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	PullRequest *json.RawMessage `json:"pull_request"`
}

func (remote remoteIssue) toIssue() Issue {
	body := ""
	if remote.Body != nil {
		body = *remote.Body
	}
	labels := make([]string, 0, len(remote.Labels))
	for _, label := range remote.Labels {
		labels = append(labels, label.Name)
	}
	return Issue{
		ID:            remote.ID,
		Number:        remote.Number,
		Title:         remote.Title,
		Body:          body,
		HTMLURL:       remote.HTMLURL,
		State:         remote.State,
		Author:        remote.User.Login,
		Labels:        labels,
		CreatedAt:     remote.CreatedAt,
		IsPullRequest: remote.PullRequest != nil,
	}
}

func nextLink(header string) string {
	for _, part := range strings.Split(header, ",") {
		segments := strings.Split(part, ";")
		if len(segments) < 2 {
			continue
		}
		isNext := false
		for _, parameter := range segments[1:] {
			if strings.TrimSpace(parameter) == `rel="next"` {
				isNext = true
				break
			}
		}
		if !isNext {
			continue
		}
		candidate := strings.TrimSpace(segments[0])
		if strings.HasPrefix(candidate, "<") && strings.HasSuffix(candidate, ">") {
			return strings.TrimSuffix(strings.TrimPrefix(candidate, "<"), ">")
		}
	}
	return ""
}
