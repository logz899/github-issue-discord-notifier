# GitHub Issue Discord Notifier Design

## Goal

Build a small Go application that watches one or more public GitHub repositories and sends newly eligible Issues to configured Discord channels. The first route watches every new Issue in `ray-project/kuberay`. The design must allow more repositories, label filters, and destination channels to be added through configuration rather than code changes.

The application runs on a scheduled GitHub Actions workflow in a public repository, so its standard hosted-runner usage is free. It polls because the operator does not administer the upstream repositories and therefore cannot install repository webhooks on them.

## Scope

The first release will:

- poll public GitHub repositories every five minutes;
- support multiple independent monitor routes;
- optionally filter a route by one or more labels;
- send each route to a named Discord destination;
- exclude pull requests returned by GitHub's Issues API;
- avoid sending the same Issue twice for the same route;
- initialize without sending historical Issues;
- persist state in the notifier repository; and
- support manual execution for setup and troubleshooting.

The first release will not provide a web UI, database, inbound HTTP server, GitHub App, Discord bot, real-time delivery, or monitoring of comments and pull requests.

## Repository Layout

```text
github-issue-discord-notifier/
├── .github/workflows/monitor.yml
├── cmd/notifier/main.go
├── internal/config/config.go
├── internal/github/client.go
├── internal/discord/client.go
├── internal/monitor/monitor.go
├── docs/superpowers/specs/
├── config.json
├── state.json
├── go.mod
└── README.md
```

Each internal package has one responsibility:

- `config` validates public routing configuration and secret destination configuration.
- `github` retrieves paginated Issue data from GitHub.
- `discord` formats and sends Discord webhook messages.
- `monitor` coordinates polling, filtering, delivery, and state transitions.
- `cmd/notifier` reads the environment and files, invokes the monitor, and returns a process exit status.

The implementation will prefer the Go standard library. No third-party dependency is required for HTTP, JSON, time handling, tests, or HMAC because this application does not receive webhook requests.

## Public Configuration

`config.json` is committed and contains no credentials:

```json
{
  "monitors": [
    {
      "id": "kuberay-all-issues",
      "repository": "ray-project/kuberay",
      "labels": [],
      "channel": "kuberay"
    },
    {
      "id": "kubernetes-good-first-issues",
      "repository": "kubernetes/kubernetes",
      "labels": ["good first issue"],
      "channel": "kubernetes"
    }
  ]
}
```

Rules:

- `id` is a unique, stable route identifier and is also the state key.
- `repository` must use `owner/name` syntax.
- `labels: []` matches every Issue.
- A non-empty `labels` list matches when the Issue has at least one configured label. Label comparison is case-insensitive after trimming whitespace.
- `channel` is an alias, not a webhook URL.
- Duplicate monitor IDs and references to unknown channel aliases are startup errors.

One repository may appear in several routes. This permits, for example, all KubeRay Issues to go to one channel while beginner-friendly KubeRay Issues also go to a focused channel.

## Secret Configuration

The repository Actions secret `DISCORD_WEBHOOKS` contains a JSON object mapping aliases to URLs:

```json
{
  "kuberay": "https://discord.com/api/webhooks/...",
  "kubernetes": "https://discord.com/api/webhooks/..."
}
```

The application will never print this value or webhook URLs. Adding a channel requires adding its alias to this JSON secret. Multiple routes may share one alias and therefore one Discord channel.

The built-in `GITHUB_TOKEN` is used for GitHub API authentication and state commits. The workflow grants only `contents: write`; no other elevated permissions are needed.

## Persistent State

`state.json` is committed to the notifier repository and starts as:

```json
{
  "version": 1,
  "monitors": {}
}
```

For each monitor ID, state records:

- the last successfully completed poll timestamp; and
- the GitHub database IDs of Issues already delivered for that route.

Issue IDs are route-specific because one Issue may legitimately be delivered to two channels. IDs remain recorded to provide durable deduplication even if an Issue is later closed, reopened, or relabeled. This state grows slowly and remains human-readable; compaction is deferred until actual repository volume demonstrates a need.

The workflow uses a concurrency group with cancellation disabled, ensuring two scheduled or manually triggered runs cannot update state simultaneously.

## Data Flow

### First Run

For a route with no state:

1. Fetch all currently open Issues that satisfy the route.
2. Exclude pull requests.
3. Record their IDs as already seen without contacting Discord.
4. Set the route checkpoint to the current time.

This creates a baseline and prevents setup from flooding a Discord channel with historical Issues.

### Subsequent Runs

For each route:

1. Query the repository Issues API using the route checkpoint as `since`, with a small overlap window to tolerate timestamp boundaries and delayed updates.
2. Follow pagination until all matching results are read.
3. Exclude pull requests and Issues that are not currently open.
4. Apply the optional label filter locally.
5. Skip IDs already delivered for this route.
6. Send each remaining Issue to the route's Discord destination.
7. Record each successful delivery immediately in in-memory state.
8. Advance the route checkpoint only when its GitHub query and every required Discord delivery succeed.
9. Atomically write `state.json` after processing all routes.

If one route fails, other routes continue. Successful deliveries are still persisted, but the process exits unsuccessfully so GitHub Actions clearly reports the partial failure. A failed route retains its earlier checkpoint; on retry, recorded successes are skipped and failed deliveries are attempted again.

## Discord Message

Each notification uses one Discord embed containing:

- repository and Issue number;
- Issue title linked to GitHub;
- author;
- labels;
- creation timestamp; and
- a bounded plain-text excerpt of the Issue body when present.

The request sets `allowed_mentions.parse` to an empty list so user-controlled Issue text cannot mention Discord users or roles. Message fields are truncated to Discord's documented limits, invalid empty values are omitted, and the request uses `wait=true` so a successful HTTP response confirms that Discord saved the message.

## Failure Handling

- Invalid files, missing required environment variables, duplicate IDs, invalid repository names, and unknown channel aliases fail before network calls.
- GitHub and Discord non-success responses include the service name, status code, and a bounded response excerpt, but never credentials.
- GitHub pagination has an explicit page limit to prevent accidental infinite loops.
- HTTP clients use timeouts.
- Discord rate-limit responses are treated as failures and retried by the next workflow run; no long-running retry loop is needed for a five-minute scheduler.
- State is written to a temporary file and renamed so interruption cannot leave partial JSON.
- The workflow commits and pushes state only when it changed. Its only triggers are `schedule` and `workflow_dispatch`, so a state commit cannot recursively trigger the workflow.

## GitHub Actions Workflow

The workflow will:

1. run every five minutes at a non-zero minute offset and support `workflow_dispatch`;
2. use a standard Linux runner;
3. check out the default branch;
4. set up the Go version declared by the project;
5. run `go test ./...` before execution;
6. run the notifier with `GITHUB_TOKEN` and `DISCORD_WEBHOOKS` in its environment; and
7. commit and push `state.json` if it changed, including after a partial route failure.

The state-persistence step must run after the notifier even when the notifier reports a partial error. The workflow then preserves the notifier's failing result so operational problems remain visible.

## Testing

Unit and integration-style tests will use `testing` and `httptest` and cover:

- configuration validation and secret redaction;
- label matching, including empty filters and case differences;
- pull-request exclusion;
- pagination;
- first-run baseline behavior;
- duplicate suppression per route;
- notification of an Issue that receives a matching label after creation;
- routing different repositories to different Discord endpoints;
- Discord payload formatting and mention suppression;
- checkpoint advancement on success;
- checkpoint retention and partial state persistence on failure; and
- atomic state file updates.

The workflow itself will be syntax-checked locally where practical and exercised manually with a test Discord webhook before production use.

## Security and Operations

- Credentials exist only in GitHub Actions Secrets and process environment variables.
- The workflow does not run on pull requests, so unreviewed contributions cannot execute with Discord credentials.
- Logs identify routes by monitor ID and repository, never by destination URL.
- The notifier requests only public repository data.
- `state.json` contains public Issue IDs and timestamps only.
- The README will document setup, adding routes, adding destinations, label behavior, manual testing, and the GitHub behavior that may disable scheduled workflows in inactive public repositories.

## Success Criteria

The release is complete when:

1. its initial KubeRay route establishes a silent baseline;
2. a newly opened KubeRay Issue produces exactly one message in the configured Discord channel within the normal polling interval;
3. rerunning the workflow does not duplicate that message;
4. a second repository and channel can be added through configuration and Secrets without Go code changes; and
5. automated tests verify routing, filtering, deduplication, failure recovery, and message safety.
