package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/logz899/github-issue-discord-notifier/internal/config"
	"github.com/logz899/github-issue-discord-notifier/internal/discord"
	"github.com/logz899/github-issue-discord-notifier/internal/github"
	"github.com/logz899/github-issue-discord-notifier/internal/monitor"
	"github.com/logz899/github-issue-discord-notifier/internal/state"
)

const defaultGitHubAPIURL = "https://api.github.com"

type Options struct {
	ConfigPath      string
	StatePath       string
	DiscordWebhooks string
	GitHubToken     string
	GitHubAPIURL    string
	HTTPClient      *http.Client
	Now             func() time.Time
	Output          io.Writer
}

func Run(ctx context.Context, options Options) error {
	applyDefaults(&options)

	cfg, err := config.Load(options.ConfigPath)
	if err != nil {
		return err
	}
	destinations, err := config.ParseDestinations(options.DiscordWebhooks)
	if err != nil {
		return err
	}
	cfg, err = config.Validate(cfg, destinations)
	if err != nil {
		return err
	}
	current, err := state.Load(options.StatePath)
	if err != nil {
		return err
	}

	runner := monitor.Runner{
		GitHub:       github.NewClient(options.GitHubAPIURL, options.GitHubToken, options.HTTPClient),
		Discord:      discord.NewClient(options.HTTPClient),
		Destinations: destinations,
		Now:          options.Now,
	}
	runErr := runner.Run(ctx, cfg, &current)
	saveErr := state.Save(options.StatePath, current)
	if saveErr == nil {
		fmt.Fprintln(options.Output, "notifier state saved")
	}
	if runErr == nil && saveErr == nil {
		fmt.Fprintln(options.Output, "notifier run completed")
	}
	if saveErr != nil {
		saveErr = fmt.Errorf("save notifier state: %w", saveErr)
	}
	return errors.Join(runErr, saveErr)
}

func applyDefaults(options *Options) {
	if options.ConfigPath == "" {
		options.ConfigPath = "config.json"
	}
	if options.StatePath == "" {
		options.StatePath = "state.json"
	}
	if options.GitHubAPIURL == "" {
		options.GitHubAPIURL = defaultGitHubAPIURL
	}
	if options.Output == nil {
		options.Output = io.Discard
	}
}
