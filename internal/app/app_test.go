package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/logz899/github-issue-discord-notifier/internal/state"
)

func writeAppFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestRunDeliversIssueAndAdvancesCheckpoint(t *testing.T) {
	checkpoint := "2026-10-19T10:00:00Z"
	now := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	configPath := writeAppFile(t, "config.json", `{"monitors":[{"id":"route","repository":"owner/repo","labels":[],"channel":"alerts"}]}`)
	statePath := writeAppFile(t, "state.json", `{"version":1,"monitors":{"route":{"last_successful_poll":"`+checkpoint+`","notified_issue_ids":[]}}}`)

	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer github-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		fmt.Fprint(w, `[{
          "id": 101, "number": 7, "title": "Issue", "body": "Details",
          "html_url": "https://github.com/owner/repo/issues/7", "state": "open",
          "user": {"login": "alice"}, "labels": [], "created_at": "2026-10-19T10:30:00Z"
        }]`)
	}))
	defer githubServer.Close()
	discordCalls := 0
	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		discordCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer discordServer.Close()

	destinations, err := json.Marshal(map[string]string{"alerts": discordServer.URL})
	if err != nil {
		t.Fatalf("marshal destinations: %v", err)
	}
	err = Run(context.Background(), Options{
		ConfigPath:      configPath,
		StatePath:       statePath,
		DiscordWebhooks: string(destinations),
		GitHubToken:     "github-token",
		GitHubAPIURL:    githubServer.URL,
		HTTPClient:      &http.Client{Timeout: time.Second},
		Now:             func() time.Time { return now },
		Output:          io.Discard,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if discordCalls != 1 {
		t.Fatalf("Discord calls = %d, want 1", discordCalls)
	}
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("load saved state: %v", err)
	}
	got := loaded.Monitors["route"]
	if !got.LastSuccessfulPoll.Equal(now) || len(got.NotifiedIssueIDs) != 1 || got.NotifiedIssueIDs[0] != 101 {
		t.Fatalf("saved route = %#v, want advanced checkpoint and issue 101", got)
	}
}

func TestRunSavesSuccessfulIDsWhenLaterDiscordDeliveryFails(t *testing.T) {
	checkpointTime := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	configPath := writeAppFile(t, "config.json", `{"monitors":[{"id":"route","repository":"owner/repo","labels":[],"channel":"alerts"}]}`)
	statePath := writeAppFile(t, "state.json", `{"version":1,"monitors":{"route":{"last_successful_poll":"2026-10-19T10:00:00Z","notified_issue_ids":[]}}}`)

	githubServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `[
          {"id":101,"number":1,"title":"First","body":"","html_url":"https://example/1","state":"open","user":{"login":"a"},"labels":[],"created_at":"2026-10-19T10:10:00Z"},
          {"id":102,"number":2,"title":"Second","body":"","html_url":"https://example/2","state":"open","user":{"login":"b"},"labels":[],"created_at":"2026-10-19T10:11:00Z"}
        ]`)
	}))
	defer githubServer.Close()
	discordCalls := 0
	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		discordCalls++
		if discordCalls == 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "retry later")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer discordServer.Close()

	destinations, _ := json.Marshal(map[string]string{"alerts": discordServer.URL})
	err := Run(context.Background(), Options{
		ConfigPath:      configPath,
		StatePath:       statePath,
		DiscordWebhooks: string(destinations),
		GitHubAPIURL:    githubServer.URL,
		HTTPClient:      &http.Client{Timeout: time.Second},
		Now:             func() time.Time { return checkpointTime.Add(time.Hour) },
		Output:          io.Discard,
	})
	if err == nil || strings.Contains(err.Error(), discordServer.URL) {
		t.Fatalf("Run() error = %v, want redacted partial failure", err)
	}
	loaded, loadErr := state.Load(statePath)
	if loadErr != nil {
		t.Fatalf("load state after partial failure: %v", loadErr)
	}
	got := loaded.Monitors["route"]
	if !got.LastSuccessfulPoll.Equal(checkpointTime) || len(got.NotifiedIssueIDs) != 1 || got.NotifiedIssueIDs[0] != 101 {
		t.Fatalf("saved route = %#v, want old checkpoint and successful issue 101", got)
	}
}
