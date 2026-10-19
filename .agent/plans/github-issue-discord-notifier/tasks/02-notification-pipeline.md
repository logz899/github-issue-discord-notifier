# Task 02: Notification Pipeline

**Goal:** Poll GitHub Issues, safely deliver eligible Issues to Discord, and
apply durable route-local baseline, deduplication, and failure recovery rules.

## Task Snapshot

Status: completed
Last updated: 2026-10-19

Current decision:

- Keep GitHub and Discord as small HTTP adapters behind monitor interfaces.
- Use `Link` pagination with visited-URL and maximum-page guards.
- Record successful delivery IDs immediately; advance a route checkpoint only
  when its query and every required delivery succeed.

Exit criteria:

- Adapter and orchestration tests cover the Review Focus conditions, and the
  full Go suite plus vet pass without contacting real services.

## Execution Context

Read first:

- `.agent/plans/github-issue-discord-notifier/BLUEPRINT.md`
- `.agent/plans/github-issue-discord-notifier/tasks/README.md`
- `internal/config/config.go`
- `internal/state/store.go`

Do not read:

- unrelated repositories or archived plans
- real GitHub or Discord credentials

Task boundary:

- May create `internal/github`, `internal/discord`, and `internal/monitor`.
- Must not add the CLI, workflow, README, or contact external services.

## Interfaces

- `github.Issue` carries ID, number, title, body, HTML URL, state, author,
  labels, creation time, and pull-request status.
- `github.Client.ListIssues(ctx context.Context, repository string, since *time.Time) ([]Issue, error)`.
- `discord.Client.Send(ctx context.Context, webhookURL, repository string, issue github.Issue) error`.
- `monitor.IssueLister` and `monitor.Notifier` mirror those adapter methods.
- `monitor.Runner.Run(ctx context.Context, cfg config.Config, current *state.State) error`
  mutates only confirmed baseline/delivery/checkpoint state.

## Current Contract

- GitHub requests use token authentication when configured, `state=open`,
  `sort=updated`, `direction=asc`, `per_page=100`, optional RFC3339 `since`, a
  finite timeout, and bounded `Link` pagination.
- Pull requests are decoded distinctly so the monitor can exclude them.
- Discord uses one bounded embed, `allowed_mentions.parse: []`, and `wait=true`;
  errors contain a bounded response excerpt but never the webhook URL.
- First run records matching open Issue IDs and current time without delivery.
- Later runs query from checkpoint minus ten minutes, exclude pull requests and
  non-open Issues, match any configured label, and skip delivered IDs.
- Failed routes retain their old checkpoint; successful IDs remain recorded;
  independent routes still run; all route errors are returned together.

## Implementation Steps

- [x] Write failing `httptest` coverage for GitHub query/auth/decoding,
  pagination, cycles, page limits, and bounded safe errors; implement the
  minimal GitHub client and make the package green.
- [x] Write failing Discord payload tests for `wait=true`, embed bounds,
  Unicode-safe truncation, disabled mentions, confirmation, and redacted
  errors; implement the minimal Discord client and make the package green.
- [x] Write failing monitor tests for silent baseline, later label matching,
  PR/non-open exclusion, per-route deduplication, overlap time, multi-channel
  routing, and partial/independent failure semantics.
- [x] Implement the monitor runner with injected adapters and clock; keep route
  state deterministic by sorting notified IDs.
- [x] Run `gofmt`, `go test ./...`, `go vet ./...`, and `git diff --check`.
- [x] Mark Task 02 complete, expand compact Task 03, and update `CURRENT.md`.

## Verification

```bash
gofmt -w internal/github/*.go internal/discord/*.go internal/monitor/*.go
go test ./...
go vet ./...
git diff --check
```

## Changelog

- 2026-10-19: Expanded compact Task 02 after Task 01 verification.
- 2026-10-19: Implemented and verified the GitHub, Discord, and monitor
  pipeline; changed the user-confirmed default to open Issues every thirty
  minutes.

## Historical Details

No historical body yet.
