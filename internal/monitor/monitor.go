package monitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/logz899/github-issue-discord-notifier/internal/config"
	gh "github.com/logz899/github-issue-discord-notifier/internal/github"
	"github.com/logz899/github-issue-discord-notifier/internal/state"
)

const defaultOverlap = 10 * time.Minute

type IssueLister interface {
	ListIssues(ctx context.Context, repository string, since *time.Time) ([]gh.Issue, error)
}

type Notifier interface {
	Send(ctx context.Context, webhookURL, repository string, issue gh.Issue) error
}

type Runner struct {
	GitHub       IssueLister
	Discord      Notifier
	Destinations config.Destinations
	Now          func() time.Time
	Overlap      time.Duration
}

func (runner Runner) Run(ctx context.Context, cfg config.Config, current *state.State) error {
	if runner.GitHub == nil || runner.Discord == nil {
		return errors.New("monitor runner requires GitHub and Discord clients")
	}
	if current == nil || current.Monitors == nil {
		return errors.New("monitor runner requires initialized state")
	}
	now := time.Now().UTC()
	if runner.Now != nil {
		now = runner.Now().UTC()
	}
	overlap := runner.Overlap
	if overlap <= 0 {
		overlap = defaultOverlap
	}

	var runErrors []error
	for _, route := range cfg.Monitors {
		existing, initialized := current.Monitors[route.ID]
		var since *time.Time
		if initialized {
			value := existing.LastSuccessfulPoll.Add(-overlap)
			since = &value
		}

		issues, err := runner.GitHub.ListIssues(ctx, route.Repository, since)
		if err != nil {
			runErrors = append(runErrors, fmt.Errorf("monitor %q: list GitHub issues: %w", route.ID, err))
			continue
		}

		next := state.Monitor{
			LastSuccessfulPoll: existing.LastSuccessfulPoll,
			NotifiedIssueIDs:   append([]int64(nil), existing.NotifiedIssueIDs...),
		}
		notified := make(map[int64]struct{}, len(next.NotifiedIssueIDs))
		for _, issueID := range next.NotifiedIssueIDs {
			notified[issueID] = struct{}{}
		}

		if !initialized {
			for _, issue := range issues {
				if eligible(issue, route.Labels) {
					if _, exists := notified[issue.ID]; !exists {
						notified[issue.ID] = struct{}{}
						next.NotifiedIssueIDs = append(next.NotifiedIssueIDs, issue.ID)
					}
				}
			}
			next.LastSuccessfulPoll = now
			sort.Slice(next.NotifiedIssueIDs, func(i, j int) bool { return next.NotifiedIssueIDs[i] < next.NotifiedIssueIDs[j] })
			current.Monitors[route.ID] = next
			continue
		}

		webhookURL, destinationExists := runner.Destinations[route.Channel]
		routeFailed := false
		if !destinationExists {
			runErrors = append(runErrors, fmt.Errorf("monitor %q: Discord destination is missing", route.ID))
			routeFailed = true
		}
		for _, issue := range issues {
			if !eligible(issue, route.Labels) {
				continue
			}
			if _, exists := notified[issue.ID]; exists {
				continue
			}
			if !destinationExists {
				continue
			}
			if err := runner.Discord.Send(ctx, webhookURL, route.Repository, issue); err != nil {
				runErrors = append(runErrors, fmt.Errorf("monitor %q issue %d: %w", route.ID, issue.Number, err))
				routeFailed = true
				continue
			}
			notified[issue.ID] = struct{}{}
			next.NotifiedIssueIDs = append(next.NotifiedIssueIDs, issue.ID)
		}
		if !routeFailed {
			next.LastSuccessfulPoll = now
		}
		sort.Slice(next.NotifiedIssueIDs, func(i, j int) bool { return next.NotifiedIssueIDs[i] < next.NotifiedIssueIDs[j] })
		current.Monitors[route.ID] = next
	}
	return errors.Join(runErrors...)
}

func eligible(issue gh.Issue, wantedLabels []string) bool {
	if issue.IsPullRequest || !strings.EqualFold(strings.TrimSpace(issue.State), "open") {
		return false
	}
	if len(wantedLabels) == 0 {
		return true
	}
	for _, wanted := range wantedLabels {
		wanted = strings.TrimSpace(wanted)
		for _, actual := range issue.Labels {
			if strings.EqualFold(wanted, strings.TrimSpace(actual)) {
				return true
			}
		}
	}
	return false
}
