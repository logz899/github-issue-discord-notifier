package monitor

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/logz899/github-issue-discord-notifier/internal/config"
	gh "github.com/logz899/github-issue-discord-notifier/internal/github"
	"github.com/logz899/github-issue-discord-notifier/internal/state"
)

type listCall struct {
	repository string
	since      *time.Time
}

type fakeLister struct {
	issuesByRepository map[string][]gh.Issue
	errorByRepository  map[string]error
	calls              []listCall
}

func (fake *fakeLister) ListIssues(_ context.Context, repository string, since *time.Time) ([]gh.Issue, error) {
	var copiedSince *time.Time
	if since != nil {
		value := *since
		copiedSince = &value
	}
	fake.calls = append(fake.calls, listCall{repository: repository, since: copiedSince})
	if err := fake.errorByRepository[repository]; err != nil {
		return nil, err
	}
	return append([]gh.Issue(nil), fake.issuesByRepository[repository]...), nil
}

type sendCall struct {
	webhookURL string
	repository string
	issueID    int64
}

type fakeNotifier struct {
	failures map[string]error
	calls    []sendCall
}

func (fake *fakeNotifier) Send(_ context.Context, webhookURL, repository string, issue gh.Issue) error {
	fake.calls = append(fake.calls, sendCall{webhookURL: webhookURL, repository: repository, issueID: issue.ID})
	return fake.failures[fmt.Sprintf("%s:%d", webhookURL, issue.ID)]
}

func TestRunCreatesSilentBaselineForEligibleOpenIssues(t *testing.T) {
	now := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	lister := &fakeLister{issuesByRepository: map[string][]gh.Issue{
		"ray-project/kuberay": {
			{ID: 3, State: "closed"},
			{ID: 2, State: "open", IsPullRequest: true},
			{ID: 1, State: "open"},
		},
	}}
	notifier := &fakeNotifier{}
	current := &state.State{Version: 1, Monitors: map[string]state.Monitor{}}
	runner := Runner{
		GitHub:       lister,
		Discord:      notifier,
		Destinations: config.Destinations{"kuberay": "https://discord.test/kuberay"},
		Now:          func() time.Time { return now },
	}
	cfg := config.Config{Monitors: []config.Monitor{{
		ID: "kuberay-all", Repository: "ray-project/kuberay", Channel: "kuberay",
	}}}

	if err := runner.Run(context.Background(), cfg, current); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(lister.calls) != 1 || lister.calls[0].since != nil {
		t.Fatalf("ListIssues calls = %#v, want one baseline call without since", lister.calls)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("Discord calls = %#v, want silent baseline", notifier.calls)
	}
	got := current.Monitors["kuberay-all"]
	if !got.LastSuccessfulPoll.Equal(now) || !slices.Equal(got.NotifiedIssueIDs, []int64{1}) {
		t.Fatalf("baseline state = %#v, want now and [1]", got)
	}
}

func TestRunMatchesLabelsAndDeduplicatesPerRoute(t *testing.T) {
	checkpoint := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	issues := []gh.Issue{
		{ID: 1, State: "open", Labels: []string{"bug"}},
		{ID: 2, State: "open", Labels: []string{"Good First Issue"}},
		{ID: 3, State: "open", Labels: []string{"bug"}},
		{ID: 4, State: "open", IsPullRequest: true, Labels: []string{"good first issue"}},
		{ID: 5, State: "closed", Labels: []string{"good first issue"}},
	}
	lister := &fakeLister{issuesByRepository: map[string][]gh.Issue{"owner/repo": issues}}
	notifier := &fakeNotifier{}
	current := &state.State{Version: 1, Monitors: map[string]state.Monitor{
		"beginner": {LastSuccessfulPoll: checkpoint, NotifiedIssueIDs: []int64{1}},
		"all":      {LastSuccessfulPoll: checkpoint, NotifiedIssueIDs: []int64{}},
	}}
	runner := Runner{
		GitHub:  lister,
		Discord: notifier,
		Destinations: config.Destinations{
			"beginners": "https://discord.test/beginners",
			"general":   "https://discord.test/general",
		},
		Now: func() time.Time { return now },
	}
	cfg := config.Config{Monitors: []config.Monitor{
		{ID: "beginner", Repository: "owner/repo", Labels: []string{"good first issue"}, Channel: "beginners"},
		{ID: "all", Repository: "owner/repo", Channel: "general"},
	}}

	if err := runner.Run(context.Background(), cfg, current); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	wantSince := checkpoint.Add(-10 * time.Minute)
	if len(lister.calls) != 2 {
		t.Fatalf("ListIssues calls = %d, want 2", len(lister.calls))
	}
	for _, call := range lister.calls {
		if call.since == nil || !call.since.Equal(wantSince) {
			t.Fatalf("ListIssues since = %v, want %s", call.since, wantSince)
		}
	}
	wantCalls := []sendCall{
		{webhookURL: "https://discord.test/beginners", repository: "owner/repo", issueID: 2},
		{webhookURL: "https://discord.test/general", repository: "owner/repo", issueID: 1},
		{webhookURL: "https://discord.test/general", repository: "owner/repo", issueID: 2},
		{webhookURL: "https://discord.test/general", repository: "owner/repo", issueID: 3},
	}
	if !slices.Equal(notifier.calls, wantCalls) {
		t.Fatalf("Discord calls = %#v, want %#v", notifier.calls, wantCalls)
	}
	if got := current.Monitors["beginner"]; !got.LastSuccessfulPoll.Equal(now) || !slices.Equal(got.NotifiedIssueIDs, []int64{1, 2}) {
		t.Fatalf("beginner state = %#v", got)
	}
	if got := current.Monitors["all"]; !got.LastSuccessfulPoll.Equal(now) || !slices.Equal(got.NotifiedIssueIDs, []int64{1, 2, 3}) {
		t.Fatalf("all state = %#v", got)
	}
}

func TestRunMatchesLabelsWithUnicodeCaseFolding(t *testing.T) {
	checkpoint := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	lister := &fakeLister{issuesByRepository: map[string][]gh.Issue{
		"owner/repo": {{ID: 1, State: "open", Labels: []string{"\u03c2"}}},
	}}
	notifier := &fakeNotifier{}
	current := &state.State{Version: 1, Monitors: map[string]state.Monitor{
		"route": {LastSuccessfulPoll: checkpoint},
	}}
	runner := Runner{
		GitHub:       lister,
		Discord:      notifier,
		Destinations: config.Destinations{"alerts": "https://discord.test/alerts"},
	}
	cfg := config.Config{Monitors: []config.Monitor{{
		ID: "route", Repository: "owner/repo", Labels: []string{"\u03c3"}, Channel: "alerts",
	}}}

	if err := runner.Run(context.Background(), cfg, current); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(notifier.calls) != 1 || notifier.calls[0].issueID != 1 {
		t.Fatalf("Discord calls = %#v, want Unicode-folded label match", notifier.calls)
	}
}

func TestRunPersistsSuccessesWithoutAdvancingFailedRouteAndContinues(t *testing.T) {
	checkpoint := time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 19, 11, 0, 0, 0, time.UTC)
	lister := &fakeLister{issuesByRepository: map[string][]gh.Issue{
		"owner/failing": {{ID: 1, State: "open"}, {ID: 2, State: "open"}},
		"owner/healthy": {{ID: 3, State: "open"}},
	}}
	notifier := &fakeNotifier{failures: map[string]error{
		"https://discord.test/failing:2": errors.New("rate limited"),
	}}
	current := &state.State{Version: 1, Monitors: map[string]state.Monitor{
		"failing": {LastSuccessfulPoll: checkpoint, NotifiedIssueIDs: []int64{}},
		"healthy": {LastSuccessfulPoll: checkpoint, NotifiedIssueIDs: []int64{}},
	}}
	runner := Runner{
		GitHub:  lister,
		Discord: notifier,
		Destinations: config.Destinations{
			"failing": "https://discord.test/failing",
			"healthy": "https://discord.test/healthy",
		},
		Now: func() time.Time { return now },
	}
	cfg := config.Config{Monitors: []config.Monitor{
		{ID: "failing", Repository: "owner/failing", Channel: "failing"},
		{ID: "healthy", Repository: "owner/healthy", Channel: "healthy"},
	}}

	err := runner.Run(context.Background(), cfg, current)
	if err == nil || !strings.Contains(err.Error(), "failing") {
		t.Fatalf("Run() error = %v, want route failure", err)
	}
	failed := current.Monitors["failing"]
	if !failed.LastSuccessfulPoll.Equal(checkpoint) || !slices.Equal(failed.NotifiedIssueIDs, []int64{1}) {
		t.Fatalf("failed route state = %#v, want old checkpoint and successful ID", failed)
	}
	healthy := current.Monitors["healthy"]
	if !healthy.LastSuccessfulPoll.Equal(now) || !slices.Equal(healthy.NotifiedIssueIDs, []int64{3}) {
		t.Fatalf("healthy route state = %#v, want advanced checkpoint and ID", healthy)
	}
}
