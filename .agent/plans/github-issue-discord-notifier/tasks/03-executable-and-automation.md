# Task 03: Executable and Automation

**Goal:** Ship a testable command, free serialized GitHub Actions schedule, and
clear setup/extension documentation without performing remote setup.

## Task Snapshot

Status: completed
Last updated: 2026-10-19

Current decision:

- Keep `main` thin and put testable composition in `internal/app`.
- Persist state after every runner attempt, including partial failure, then
  preserve the runner's failing exit result.
- Schedule at minutes 7 and 37 to avoid top-of-hour congestion.

Exit criteria:

- Command tests prove success and partial-failure state persistence, the
  workflow performs all local gates and state commits in the required order,
  the README covers setup and extension, and the complete verification passes.

## Execution Context

Read first:

- `.agent/plans/github-issue-discord-notifier/BLUEPRINT.md`
- `internal/config/config.go`
- `internal/state/store.go`
- `internal/github/client.go`
- `internal/discord/client.go`
- `internal/monitor/monitor.go`

Task boundary:

- May create `internal/app`, `cmd/notifier`, `.github/workflows/monitor.yml`,
  and `README.md`.
- Must not create/push a remote repository, set Secrets, enable Actions, or send
  to a real Discord webhook.

## Interfaces

- `app.Options` supplies config/state paths, secret JSON, GitHub token/API URL,
  HTTP client, clock, and output writer.
- `app.Run(ctx context.Context, options Options) error` loads/validates inputs,
  runs the real pipeline, saves mutated state even on partial failure, and
  returns the combined operational error.
- `cmd/notifier` maps environment variables into `app.Options` and exits nonzero
  on any returned error.

## Current Contract

- Defaults are `config.json`, `state.json`, and `https://api.github.com`;
  `CONFIG_PATH`, `STATE_PATH`, and `GITHUB_API_URL` may override them for local
  testing.
- `DISCORD_WEBHOOKS` is required; `GITHUB_TOKEN` is optional for public data but
  supplied automatically by Actions for authenticated rate limits.
- Errors and logs never include secret JSON, tokens, or webhook URLs.
- The workflow runs tests and vet, executes the notifier without swallowing its
  status, commits changed `state.json` even after partial failure, then restores
  the notifier's result as the job result.
- README documents exact API fields/query behavior, state layout, Discord
  appearance, initial silent baseline, 30-minute schedule, multi-route config,
  secret setup, manual dispatch, free-public-repo constraint, and 60-day public
  schedule inactivity behavior.

## Implementation Steps

- [x] Write failing app integration tests using local HTTP servers for a normal
  run and a partial Discord failure whose successful IDs are still saved.
- [x] Implement `internal/app` and thin `cmd/notifier`; make focused and full Go
  tests green without real network calls.
- [x] Add the serialized thirty-minute Actions workflow with test/vet gates,
  notifier status preservation, and conditional `state.json` commit/push.
- [x] Add README setup, configuration, polling/state/message explanations, and
  multi-repository/channel extension examples.
- [x] Run formatting, full tests, race tests, vet, build, diff checks, and inspect
  the workflow plus repository status.
- [x] Mark Task 03 complete and update `CURRENT.md` for final review.

## Verification

```bash
gofmt -w cmd/notifier/*.go internal/app/*.go
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/github-issue-discord-notifier ./cmd/notifier
git diff --check
```

## Changelog

- 2026-10-19: Expanded compact Task 03 after Task 02 verification.
- 2026-10-19: Implemented CLI composition, partial-failure state persistence,
  the thirty-minute workflow, and setup/operations documentation; full, race,
  vet, build, and diff verification passed.
- 2026-10-19: Final review pinned runs and state writes to the latest default
  branch and added a safe rebase before pushing state.

## Historical Details

No historical body yet.
