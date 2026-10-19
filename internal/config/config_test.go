package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadRejectsUnknownAndTrailingJSON(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "unknown field",
			body: `{"monitors": [], "unexpected": true}`,
		},
		{
			name: "trailing document",
			body: `{"monitors": []} {"monitors": []}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Load(writeConfig(t, tt.body)); err == nil {
				t.Fatal("Load() error = nil, want rejection")
			}
		})
	}
}

func TestParseDestinationsRejectsInvalidOrBlankValuesWithoutLeakingSecrets(t *testing.T) {
	const secretURL = "https://discord.com/api/webhooks/sentinel-secret"
	tests := []struct {
		name string
		raw  string
	}{
		{name: "empty object", raw: `{}`},
		{name: "invalid json", raw: `{"kuberay":`},
		{name: "blank alias", raw: `{"   ":"` + secretURL + `"}`},
		{name: "blank url", raw: `{"kuberay":"   "}`},
		{name: "malformed url", raw: `{"kuberay":"not-a-webhook"}`},
		{name: "url without host", raw: `{"kuberay":"https:///missing-host"}`},
		{name: "trailing document", raw: `{"kuberay":"` + secretURL + `"} {}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDestinations(tt.raw)
			if err == nil {
				t.Fatal("ParseDestinations() error = nil, want rejection")
			}
			if strings.Contains(err.Error(), secretURL) {
				t.Fatalf("error leaked secret URL: %v", err)
			}
		})
	}
}

func TestValidateNormalizesValidRoutes(t *testing.T) {
	cfg := Config{Monitors: []Monitor{{
		ID:         "  KubeRay-All  ",
		Repository: " Ray-Project/KubeRay ",
		Labels:     []string{" Good First Issue ", "BUG"},
		Channel:    " KubeRay ",
	}}}
	destinations := Destinations{"kuberay": "https://discord.com/api/webhooks/example"}

	got, err := Validate(cfg, destinations)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	want := Monitor{
		ID:         "kuberay-all",
		Repository: "ray-project/kuberay",
		Labels:     []string{"good first issue", "bug"},
		Channel:    "kuberay",
	}
	if len(got.Monitors) != 1 || got.Monitors[0].ID != want.ID ||
		got.Monitors[0].Repository != want.Repository || got.Monitors[0].Channel != want.Channel {
		t.Fatalf("Validate() monitor = %#v, want %#v", got.Monitors, want)
	}
	if strings.Join(got.Monitors[0].Labels, ",") != strings.Join(want.Labels, ",") {
		t.Fatalf("Validate() labels = %#v, want %#v", got.Monitors[0].Labels, want.Labels)
	}
}

func TestValidateRejectsInvalidRoutes(t *testing.T) {
	valid := Monitor{ID: "route", Repository: "owner/repo", Channel: "channel"}
	destinations := Destinations{"channel": "https://discord.com/api/webhooks/example"}
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "empty monitors", cfg: Config{}},
		{name: "duplicate normalized ids", cfg: Config{Monitors: []Monitor{
			valid,
			{ID: " ROUTE ", Repository: "owner/other", Channel: "channel"},
		}}},
		{name: "blank id", cfg: Config{Monitors: []Monitor{{Repository: "owner/repo", Channel: "channel"}}}},
		{name: "missing owner", cfg: Config{Monitors: []Monitor{{ID: "route", Repository: "/repo", Channel: "channel"}}}},
		{name: "missing repo", cfg: Config{Monitors: []Monitor{{ID: "route", Repository: "owner/", Channel: "channel"}}}},
		{name: "extra slash", cfg: Config{Monitors: []Monitor{{ID: "route", Repository: "owner/repo/more", Channel: "channel"}}}},
		{name: "invalid repository characters", cfg: Config{Monitors: []Monitor{{ID: "route", Repository: "owner/repo name", Channel: "channel"}}}},
		{name: "duplicate normalized labels", cfg: Config{Monitors: []Monitor{{
			ID: "route", Repository: "owner/repo", Labels: []string{"Bug", " bug "}, Channel: "channel",
		}}}},
		{name: "duplicate Unicode-folded labels", cfg: Config{Monitors: []Monitor{{
			ID: "route", Repository: "owner/repo", Labels: []string{"\u03c3", "\u03c2"}, Channel: "channel",
		}}}},
		{name: "blank label", cfg: Config{Monitors: []Monitor{{
			ID: "route", Repository: "owner/repo", Labels: []string{" "}, Channel: "channel",
		}}}},
		{name: "unknown channel", cfg: Config{Monitors: []Monitor{{
			ID: "route", Repository: "owner/repo", Channel: "missing",
		}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Validate(tt.cfg, destinations); err == nil {
				t.Fatal("Validate() error = nil, want rejection")
			}
		})
	}
}
