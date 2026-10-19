# GitHub Issue Discord Notifier Project Guide

## Purpose

GitHub Issue Discord Notifier polls public GitHub repositories and sends each
newly eligible Issue to a configured Discord channel. Routes are configured as
repository, optional labels, and destination aliases so adding repositories or
channels does not require Go changes.

## Stack

- Application: Go standard library.
- Source: GitHub REST Issues API.
- Destination: Discord incoming webhooks.
- Scheduler: GitHub Actions every five minutes plus manual dispatch.
- Persistence: committed JSON state guarded by workflow concurrency.
- Tests: Go `testing` and `httptest`.

## Repository Layout

```text
cmd/notifier/       CLI composition and exit behavior
internal/config/    Public and secret configuration
internal/state/     Durable checkpoint and delivery state
internal/github/    GitHub Issues API client
internal/discord/   Discord payload and webhook client
internal/monitor/   Poll, filter, route, and state orchestration
.github/workflows/  Scheduled execution and state persistence
.agent/             Shared agent guidance and active plan
docs/               Approved design source
```

## Commands

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Run locally only with explicitly supplied non-production credentials:

```bash
GITHUB_TOKEN=... DISCORD_WEBHOOKS='{"alias":"https://discord.com/api/webhooks/..."}' \
  go run ./cmd/notifier
```

## Security Rules

- Never commit or print Discord webhook URLs, GitHub tokens, private keys, or
  complete secret JSON values.
- Keep destination aliases in `config.json`; keep actual URLs only in the
  `DISCORD_WEBHOOKS` environment variable or GitHub Actions Secret.
- Do not run unreviewed pull-request code with repository secrets.
- Disable Discord mentions for all user-controlled GitHub text.
- Error messages may include route IDs, repositories, HTTP status codes, and
  bounded response excerpts, but never request authorization or destination
  URLs.

## Working Tree and Commit Rules

Preserve unrelated user work. Never push, create a GitHub repository, change
repository visibility, add Actions Secrets, or activate a production Discord
webhook without explicit approval.

Do not auto-commit implementation work. After verification, show the proposed
commit message and let the user decide whether to commit. The approved design
commit predates this rule and remains part of repository history.
