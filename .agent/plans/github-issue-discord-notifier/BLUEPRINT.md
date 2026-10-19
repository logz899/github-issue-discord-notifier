# GitHub Issue Discord Notifier Blueprint

Status: approved
Last updated: 2026-10-19

## Architecture

A dependency-free Go command reads public route configuration, secret Discord
destinations, and committed delivery state. Focused GitHub and Discord clients
sit behind interfaces consumed by a monitor service, which applies route-local
filtering and durable deduplication. GitHub Actions runs the command every thirty
minutes under a single concurrency group and commits changed state.

The first route watches all Issues in `ray-project/kuberay`. Configuration is
general: one repository may feed multiple destinations, multiple repositories
may share a destination, and label lists may narrow a route without changing
application code.

## Data Flow

```text
GitHub Actions schedule/manual dispatch
  -> load config.json + DISCORD_WEBHOOKS + state.json
  -> for each monitor route
       -> GitHub Issues API, paginated by updated time
       -> remove pull requests and non-open Issues
       -> apply optional label-any filter
       -> remove IDs delivered for this route
       -> Discord webhook with wait=true and mentions disabled
       -> record each confirmed delivery
       -> advance checkpoint only after the route fully succeeds
  -> atomically replace state.json
  -> commit state.json when changed
  -> retain a failing workflow result when any route partially fails
```

On a route's first run, the monitor fetches all matching currently open Issues,
records them without delivery, and sets the initial checkpoint. Later queries
use a checkpoint overlap to tolerate boundary timing. Successful IDs are saved
even during partial failure; a failed route does not advance its checkpoint.

## Configuration Surface

Public `config.json`:

```json
{
  "monitors": [
    {
      "id": "kuberay-all-issues",
      "repository": "ray-project/kuberay",
      "labels": [],
      "channel": "kuberay"
    }
  ]
}
```

Secret `DISCORD_WEBHOOKS`:

```json
{
  "kuberay": "https://discord.com/api/webhooks/..."
}
```

State `state.json`:

```json
{
  "version": 1,
  "monitors": {}
}
```

Rules:

- Monitor IDs are unique stable state keys.
- Repositories use exactly `owner/name` syntax.
- Empty labels match all Issues; otherwise any configured label matches after
  whitespace trimming and case folding.
- Channels are aliases that must resolve in `DISCORD_WEBHOOKS`.
- Unknown fields, duplicate IDs, malformed repositories, blank aliases, and
  invalid state fail before network activity.

## Internal Interfaces

- `internal/config` owns file/env decoding, normalization, and cross-reference
  validation.
- `internal/state` owns versioned JSON decoding and atomic replacement.
- `internal/github` owns authenticated pagination and maps remote objects to a
  small internal `Issue` contract.
- `internal/discord` owns safe embed formatting and confirmed webhook delivery.
- `internal/monitor` owns first-run baseline, matching, deduplication,
  per-route failure isolation, checkpoints, and state mutation.
- `cmd/notifier` owns dependency construction, environment checks, logging, and
  exit status only.

## Discord Contract

Each delivery uses one embed with repository and Issue number, linked title,
author, labels, creation time, and a bounded body excerpt. All fields obey
Discord limits. `allowed_mentions.parse` is empty and `wait=true` is present so
user-controlled text cannot ping users or roles and a successful response
confirms message persistence.

## Failure Behavior

- HTTP clients have finite timeouts and bounded error bodies.
- Neither authorization values nor Discord URLs appear in logs or errors.
- GitHub pagination has a page maximum and rejects loops.
- A failing route does not prevent independent routes from being attempted.
- Each successful notification is recorded before state is saved after the run.
- State uses a temporary file in the target directory followed by rename.
- Corrupt state is an error and is never silently replaced with empty state.
- Discord rate limits fail the route and rely on the next scheduled run rather
  than holding a runner in a long retry loop.

## GitHub Actions

The workflow runs at a non-zero minute offset every thirty minutes and via manual
dispatch, uses a standard Linux runner, grants only `contents: write`, and sets
`concurrency.cancel-in-progress: false`. It runs formatting checks, tests, and
vet before the notifier. State persistence runs even after a partial notifier
failure, while the final workflow result remains failed.

The workflow never runs with secrets for pull-request events. Commits to
`state.json` do not recursively trigger it because its only triggers are the
schedule and manual dispatch.

## ADRs

- ADR-1: Poll from the operator's repository instead of using an upstream
  webhook, because upstream administration permission is unavailable.
- ADR-2: Use one configurable process instead of one workflow per source, so
  repository-to-channel routes scale without duplicated code.
- ADR-3: Use Go's standard library to avoid dependency download, update, and
  supply-chain overhead for a small HTTP/JSON command.
- ADR-4: Commit readable JSON state instead of provisioning a database, because
  the scheduler is serialized and expected Issue volume is small.
- ADR-5: Retain notified IDs indefinitely in v1 for durable at-most-once routing;
  measured growth, not speculation, will trigger compaction work.
- ADR-6: Match any configured label, which supports category alternatives while
  preserving the empty-list means all contract.

## Trade-offs

| Option | Pros | Cons | Decision |
|---|---|---|---|
| One scheduled configurable Go command | Free, simple, multi-repo/channel | Up to thirty-minute normal latency; committed state | Chosen |
| Workflow per repository | Easy initial setup | Duplicated logic and secrets wiring | Rejected |
| Hosted service and database | Near-real-time; richer operations | Hosting cost and ongoing maintenance | Deferred |
| Fork upstream and install webhook | Familiar webhook flow | Fork Issues are independent from upstream | Rejected |

## Risks

- GitHub may delay or drop a scheduled run. Checkpoint overlap plus durable
  deduplication allows a later run to recover.
- A public repository schedule may be disabled after prolonged inactivity. The
  README will document detection and manual re-enablement.
- A multi-delivery run can partially fail. Successful route-local IDs are saved
  while the failed checkpoint remains unchanged.
- User text can contain Markdown or mention syntax. Embed bounds and disabled
  mentions prevent pings and oversized payloads.
- Concurrent manual and scheduled runs can race. Workflow concurrency prevents
  overlap before either process reads state.

## Success Criteria

- The initial KubeRay execution creates a silent baseline.
- A later newly opened KubeRay Issue produces exactly one Discord notification
  within the normal polling interval.
- Reruns and partial retries do not duplicate successful notifications.
- A second repository and channel require configuration and Secrets changes
  only.
- Automated tests cover validation, pagination, filtering, first-run behavior,
  routing, deduplication, partial failure, state safety, and mention suppression.

## Changelog

- 2026-10-19: Converted the approved design into the repository plan format.

## Detailed Source

See `docs/superpowers/specs/2026-10-19-github-issue-discord-notifier-design.md`
for the original approved design.
