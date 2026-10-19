package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gh "github.com/logz899/github-issue-discord-notifier/internal/github"
)

const (
	requestTimeout = 15 * time.Second
	maxErrorBody   = 512
	maxTitleRunes  = 256
	maxBodyRunes   = 3500
	maxRepoRunes   = 512
	maxAuthorRunes = 256
	maxLabelsRunes = 1024
)

type Client struct {
	HTTP *http.Client
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	} else if httpClient.Timeout == 0 {
		clone := *httpClient
		clone.Timeout = requestTimeout
		httpClient = &clone
	}
	return &Client{HTTP: httpClient}
}

func (client *Client) Send(ctx context.Context, webhookURL, repository string, issue gh.Issue) error {
	endpoint, err := url.Parse(webhookURL)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return errors.New("invalid Discord webhook URL")
	}
	query := endpoint.Query()
	query.Set("wait", "true")
	endpoint.RawQuery = query.Encode()

	payload := buildPayload(repository, issue)
	body, err := json.Marshal(payload)
	if err != nil {
		return errors.New("encode Discord webhook payload")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return errors.New("create Discord webhook request")
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.HTTP.Do(request)
	if err != nil {
		return errors.New("send Discord webhook request")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return fmt.Errorf("Discord webhook returned %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

type webhookPayload struct {
	AllowedMentions allowedMentions `json:"allowed_mentions"`
	Embeds          []embed         `json:"embeds"`
}

type allowedMentions struct {
	Parse []string `json:"parse"`
}

type embed struct {
	Title       string       `json:"title"`
	URL         string       `json:"url,omitempty"`
	Description string       `json:"description,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
	Fields      []embedField `json:"fields"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func buildPayload(repository string, issue gh.Issue) webhookPayload {
	timestamp := ""
	if !issue.CreatedAt.IsZero() {
		timestamp = issue.CreatedAt.UTC().Format(time.RFC3339)
	}
	labels := strings.Join(issue.Labels, ", ")
	if labels == "" {
		labels = "None"
	}
	author := issue.Author
	if strings.TrimSpace(author) == "" {
		author = "unknown"
	}
	return webhookPayload{
		AllowedMentions: allowedMentions{Parse: []string{}},
		Embeds: []embed{{
			Title:       truncateRunes(fmt.Sprintf("#%d %s", issue.Number, issue.Title), maxTitleRunes),
			URL:         issue.HTMLURL,
			Description: truncateRunes(issue.Body, maxBodyRunes),
			Timestamp:   timestamp,
			Fields: []embedField{
				{Name: "Repository", Value: truncateRunes(fmt.Sprintf("%s #%d", repository, issue.Number), maxRepoRunes), Inline: true},
				{Name: "Author", Value: truncateRunes(author, maxAuthorRunes), Inline: true},
				{Name: "Labels", Value: truncateRunes(labels, maxLabelsRunes), Inline: false},
			},
		}},
	}
}

func truncateRunes(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	if maximum <= 1 {
		return string(runes[:maximum])
	}
	return string(runes[:maximum-1]) + "…"
}
