package config_test

import (
	"path/filepath"
	"testing"

	"github.com/logz899/github-issue-discord-notifier/internal/config"
	"github.com/logz899/github-issue-discord-notifier/internal/state"
)

func TestRepositorySeedFilesValidate(t *testing.T) {
	root := filepath.Join("..", "..")
	cfg, err := config.Load(filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatalf("load repository config: %v", err)
	}
	destinations, err := config.ParseDestinations(`{"kuberay":"https://discord.com/api/webhooks/example"}`)
	if err != nil {
		t.Fatalf("parse placeholder destinations: %v", err)
	}
	validated, err := config.Validate(cfg, destinations)
	if err != nil {
		t.Fatalf("validate repository config: %v", err)
	}
	if len(validated.Monitors) != 1 || validated.Monitors[0].Repository != "ray-project/kuberay" {
		t.Fatalf("repository config monitors = %#v, want KubeRay route", validated.Monitors)
	}

	loadedState, err := state.Load(filepath.Join(root, "state.json"))
	if err != nil {
		t.Fatalf("load repository state: %v", err)
	}
	if loadedState.Version != 1 || len(loadedState.Monitors) != 0 {
		t.Fatalf("repository state = %#v, want empty version 1 state", loadedState)
	}
}
