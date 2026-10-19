package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	gh "github.com/logz899/github-issue-discord-notifier/internal/github"
)

func TestSendPostsBoundedEmbedWithMentionsDisabled(t *testing.T) {
	title := strings.Repeat("界", 300)
	body := "@everyone <@123> " + strings.Repeat("詳", 5000)
	issue := gh.Issue{
		ID:        101,
		Number:    42,
		Title:     title,
		Body:      body,
		HTMLURL:   "https://github.com/ray-project/kuberay/issues/42",
		State:     "open",
		Author:    "alice",
		Labels:    []string{"bug", "good first issue"},
		CreatedAt: time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Query().Get("wait") != "true" {
			t.Errorf("wait = %q, want true", r.URL.Query().Get("wait"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}

		var payload struct {
			AllowedMentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
			Embeds []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Timestamp   string `json:"timestamp"`
				Fields      []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"fields"`
			} `json:"embeds"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.AllowedMentions.Parse == nil || len(payload.AllowedMentions.Parse) != 0 {
			t.Errorf("allowed mentions = %#v, want explicit empty parse", payload.AllowedMentions.Parse)
		}
		if len(payload.Embeds) != 1 {
			t.Fatalf("embeds = %d, want 1", len(payload.Embeds))
		}
		embed := payload.Embeds[0]
		if utf8.RuneCountInString(embed.Title) > 256 || !utf8.ValidString(embed.Title) {
			t.Errorf("title is not Unicode-safe within limit: %d runes", utf8.RuneCountInString(embed.Title))
		}
		if utf8.RuneCountInString(embed.Description) > 4096 || !utf8.ValidString(embed.Description) {
			t.Errorf("description is not Unicode-safe within limit: %d runes", utf8.RuneCountInString(embed.Description))
		}
		if embed.URL != issue.HTMLURL || embed.Timestamp != "2026-10-19T09:30:00Z" {
			t.Errorf("embed URL/timestamp = %q/%q", embed.URL, embed.Timestamp)
		}
		if len(embed.Fields) != 3 || embed.Fields[0].Value != "ray-project/kuberay #42" || embed.Fields[1].Value != "alice" || embed.Fields[2].Value != "bug, good first issue" {
			t.Errorf("embed fields = %#v", embed.Fields)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.Client())
	if err := client.Send(context.Background(), server.URL, "ray-project/kuberay", issue); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSendReturnsBoundedErrorWithoutWebhookURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, strings.Repeat("rate-limited ", 1000))
	}))
	defer server.Close()

	client := NewClient(server.Client())
	err := client.Send(context.Background(), server.URL, "owner/repo", gh.Issue{ID: 1, Number: 1, Title: "Issue"})
	if err == nil {
		t.Fatal("Send() error = nil, want HTTP error")
	}
	if strings.Contains(err.Error(), server.URL) || len(err.Error()) > 700 {
		t.Fatalf("Send() returned unsafe error of length %d: %v", len(err.Error()), err)
	}
}
