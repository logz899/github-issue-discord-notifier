package github

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListIssuesSendsExpectedRequestAndFollowsPagination(t *testing.T) {
	since := time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC)
	requests := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/repos/ray-project/kuberay/issues" {
			t.Errorf("path = %q, want repository issues path", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer github-token" {
			t.Errorf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		query := r.URL.Query()
		if query.Get("state") != "open" || query.Get("sort") != "updated" || query.Get("direction") != "asc" || query.Get("per_page") != "100" {
			t.Errorf("query = %v, want state/sort/direction/per_page", query)
		}
		if query.Get("since") != "2026-10-19T09:30:00Z" {
			t.Errorf("since = %q", query.Get("since"))
		}

		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") == "2" {
			fmt.Fprint(w, `[{
                  "id": 202,
                  "number": 12,
                  "title": "Pull request",
                  "body": null,
                  "html_url": "https://github.com/ray-project/kuberay/pull/12",
                  "state": "open",
                  "user": {"login": "bob"},
                  "labels": [],
                  "created_at": "2026-10-19T09:05:00Z",
                  "pull_request": {"url": "https://api.github.com/pulls/12"}
                }]`)
			return
		}
		w.Header().Set("Link", fmt.Sprintf(`<%s/repos/ray-project/kuberay/issues?page=2&state=open&sort=updated&direction=asc&per_page=100&since=2026-10-19T09%%3A30%%3A00Z>; rel="next"`, server.URL))
		fmt.Fprint(w, `[{
              "id": 101,
              "number": 11,
              "title": "New issue",
              "body": "Details",
              "html_url": "https://github.com/ray-project/kuberay/issues/11",
              "state": "open",
              "user": {"login": "alice"},
              "labels": [{"name": "bug"}],
              "created_at": "2026-10-19T09:00:00Z"
            }]`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "github-token", server.Client())
	issues, err := client.ListIssues(context.Background(), "ray-project/kuberay", &since)
	if err != nil {
		t.Fatalf("ListIssues() error = %v", err)
	}
	if requests != 2 || len(issues) != 2 {
		t.Fatalf("requests/issues = %d/%d, want 2/2", requests, len(issues))
	}
	if issues[0].ID != 101 || issues[0].Author != "alice" || strings.Join(issues[0].Labels, ",") != "bug" || issues[0].IsPullRequest {
		t.Fatalf("first issue = %#v", issues[0])
	}
	if issues[1].ID != 202 || !issues[1].IsPullRequest || issues[1].Body != "" {
		t.Fatalf("second issue = %#v", issues[1])
	}
}

func TestListIssuesRejectsUnsafePagination(t *testing.T) {
	tests := []struct {
		name      string
		maxPages  int
		link      func(serverURL string) string
		wantError string
	}{
		{
			name:      "cycle",
			maxPages:  10,
			link:      func(serverURL string) string { return `<` + serverURL + `/repos/o/r/issues>; rel="next"` },
			wantError: "cycle",
		},
		{
			name:      "page limit",
			maxPages:  1,
			link:      func(serverURL string) string { return `<` + serverURL + `/repos/o/r/issues?page=2>; rel="next"` },
			wantError: "page limit",
		},
		{
			name:      "cross origin",
			maxPages:  10,
			link:      func(string) string { return `<https://example.com/issues?page=2>; rel="next"` },
			wantError: "origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Link", tt.link(server.URL))
				fmt.Fprint(w, `[]`)
			}))
			defer server.Close()

			client := NewClient(server.URL, "token", server.Client())
			client.MaxPages = tt.maxPages
			_, err := client.ListIssues(context.Background(), "o/r", nil)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.wantError) {
				t.Fatalf("ListIssues() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestListIssuesReturnsBoundedRedactedHTTPError(t *testing.T) {
	const token = "sentinel-github-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, strings.Repeat("x", 5000))
	}))
	defer server.Close()

	client := NewClient(server.URL, token, server.Client())
	_, err := client.ListIssues(context.Background(), "o/r", nil)
	if err == nil {
		t.Fatal("ListIssues() error = nil, want HTTP error")
	}
	if strings.Contains(err.Error(), token) || len(err.Error()) > 700 {
		t.Fatalf("ListIssues() returned unsafe error of length %d: %v", len(err.Error()), err)
	}
}
