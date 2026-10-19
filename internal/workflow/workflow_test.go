package workflow

import (
	"os"
	"strings"
	"testing"
)

func TestWorkflowAlwaysUsesLatestDefaultBranchState(t *testing.T) {
	contents, err := os.ReadFile("../../.github/workflows/monitor.yml")
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	workflow := string(contents)
	wants := []string{
		"ref: ${{ github.event.repository.default_branch }}",
		"TARGET_BRANCH: ${{ github.event.repository.default_branch }}",
		"git pull --rebase origin \"${TARGET_BRANCH}\"",
		"go build -o /tmp/github-issue-discord-notifier ./cmd/notifier",
	}
	for _, want := range wants {
		if !strings.Contains(workflow, want) {
			t.Errorf("workflow missing %q", want)
		}
	}
}
