package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeState(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	return path
}

func TestLoadValidState(t *testing.T) {
	path := writeState(t, `{
  "version": 1,
  "monitors": {
    "kuberay": {
      "last_successful_poll": "2026-10-19T09:30:00Z",
      "notified_issue_ids": [101, 102]
    }
  }
}`)

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantTime := time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC)
	monitor := got.Monitors["kuberay"]
	if got.Version != 1 || !monitor.LastSuccessfulPoll.Equal(wantTime) {
		t.Fatalf("Load() = %#v, want version 1 at %s", got, wantTime)
	}
	if len(monitor.NotifiedIssueIDs) != 2 || monitor.NotifiedIssueIDs[0] != 101 || monitor.NotifiedIssueIDs[1] != 102 {
		t.Fatalf("Load() issue IDs = %#v, want [101 102]", monitor.NotifiedIssueIDs)
	}
}

func TestLoadRejectsUnsafeState(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "corrupt json", body: `{"version":`},
		{name: "trailing document", body: `{"version":1,"monitors":{}} {}`},
		{name: "unknown field", body: `{"version":1,"monitors":{},"other":true}`},
		{name: "missing version", body: `{"monitors":{}}`},
		{name: "future version", body: `{"version":2,"monitors":{}}`},
		{name: "nil monitors", body: `{"version":1,"monitors":null}`},
		{name: "zero checkpoint", body: `{"version":1,"monitors":{"route":{"last_successful_poll":"0001-01-01T00:00:00Z","notified_issue_ids":[]}}}`},
		{name: "negative issue id", body: `{"version":1,"monitors":{"route":{"last_successful_poll":"2026-10-19T09:30:00Z","notified_issue_ids":[-1]}}}`},
		{name: "duplicate issue id", body: `{"version":1,"monitors":{"route":{"last_successful_poll":"2026-10-19T09:30:00Z","notified_issue_ids":[42,42]}}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeState(t, tt.body)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read before: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load() error = nil, want rejection")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read after: %v", err)
			}
			if string(after) != string(before) {
				t.Fatalf("Load() modified state file: got %q, want %q", after, before)
			}
		})
	}
}

func TestSaveWritesDeterministicVersionedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	value := State{
		Version: 1,
		Monitors: map[string]Monitor{
			"kuberay": {
				LastSuccessfulPoll: time.Date(2026, 10, 19, 9, 30, 0, 0, time.UTC),
				NotifiedIssueIDs:   []int64{101, 102},
			},
		},
	}

	if err := Save(path, value); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved state: %v", err)
	}
	if !strings.HasSuffix(string(body), "\n") || strings.HasSuffix(string(body), "\n\n") {
		t.Fatalf("saved state must end in exactly one newline: %q", body)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(saved state) error = %v", err)
	}
	monitor := loaded.Monitors["kuberay"]
	if loaded.Version != 1 || len(monitor.NotifiedIssueIDs) != 2 || monitor.NotifiedIssueIDs[1] != 102 {
		t.Fatalf("Load(saved state) = %#v, want original value", loaded)
	}
}

func TestSaveFailurePreservesExistingFile(t *testing.T) {
	path := writeState(t, "original state\n")
	invalid := State{Version: 2, Monitors: map[string]Monitor{}}

	if err := Save(path, invalid); err == nil {
		t.Fatal("Save() error = nil, want invalid-state rejection")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original state: %v", err)
	}
	if string(body) != "original state\n" {
		t.Fatalf("Save() changed original after failure: %q", body)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".state-*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("Save() left temporary files: %v", matches)
	}
}
