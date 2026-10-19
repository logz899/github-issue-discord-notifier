package main

import (
	"context"
	"log"
	"os"

	"github.com/logz899/github-issue-discord-notifier/internal/app"
)

func main() {
	err := app.Run(context.Background(), app.Options{
		ConfigPath:      os.Getenv("CONFIG_PATH"),
		StatePath:       os.Getenv("STATE_PATH"),
		DiscordWebhooks: os.Getenv("DISCORD_WEBHOOKS"),
		GitHubToken:     os.Getenv("GITHUB_TOKEN"),
		GitHubAPIURL:    os.Getenv("GITHUB_API_URL"),
		Output:          os.Stdout,
	})
	if err != nil {
		log.Printf("notifier failed: %v", err)
		os.Exit(1)
	}
}
