# GitHub Issue Discord Notifier

A small Go service that checks public GitHub repositories every 5 minutes and
sends newly eligible Issues to Discord. It starts with all open Issues in
`ray-project/kuberay` and supports multiple repository, label, and Discord
channel routes without Go code changes.

It uses polling because a fork has its own Issues and cannot receive upstream
Issue webhooks. On a public repository, standard GitHub-hosted Actions runners
are free.

## What a Discord notification looks like

```text
#42 [Bug] RayCluster autoscaling fails after restart     (clickable)

After upgrading to v1.x, the autoscaler...               (bounded excerpt)

Repository    ray-project/kuberay #42
Author        alice
Labels        bug, good first issue

2026-10-19 17:30                                         (localized by Discord)
```

The Issue title links to GitHub. Body, title, and fields are truncated on Unicode
boundaries to stay within Discord limits. `allowed_mentions` is disabled, so
Issue text such as `@everyone` or `<@123>` cannot ping Discord users or roles.

## What each poll requests

For every configured monitor route, the notifier calls:

```http
GET /repos/{owner}/{repo}/issues
  ?state=open
  &sort=updated
  &direction=asc
  &per_page=100
  &since={last successful poll minus 10 minutes}
```

It uses only these fields from each returned Issue:

- GitHub database ID, for durable deduplication;
- Issue number, title, body (truncated before Discord delivery), and GitHub URL;
- open/closed state and whether the item is actually a pull request;
- author login, labels, and creation time.

GitHub's Issues API also returns pull requests; those are explicitly removed.
The notifier does not fetch comments, email addresses, private repositories, or
GitHub user profiles. Label filtering happens locally so a label added after an
Issue was opened is still detected through the Issue's updated time.

The ten-minute overlap protects timestamp boundaries and delayed scheduled
runs. Stored Issue IDs prevent overlap from producing duplicate Discord
messages. A second API page is requested only when GitHub provides a `next`
pagination link; one page contains up to 100 Issues.

### First run

A route without saved state fetches its currently open Issues, records the
matching IDs, and sends nothing. This silent baseline prevents setup from
flooding Discord with historical Issues.

## State

Progress is stored in committed `state.json`, separately for each monitor:

```json
{
  "version": 1,
  "monitors": {
    "kuberay-all-issues": {
      "last_successful_poll": "2026-10-19T10:15:00Z",
      "notified_issue_ids": [123456789]
    }
  }
}
```

A route advances `last_successful_poll` only after its GitHub query and every
required Discord delivery succeed. If delivery partially fails, successful IDs
are still saved but the checkpoint stays unchanged. The next run retries failed
Issues and skips those already confirmed by Discord. State contains public Issue
IDs and timestamps only; it never contains webhook URLs or tokens.

GitHub Actions serializes runs and commits `state.json` after every content
change, including partial failures. Every run checks out the latest default
branch and writes state back to that same branch, including manual runs started
from another ref.

## Configuration

Public routes live in `config.json`:

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
      "id": "kubernetes-beginner-issues",
      "repository": "kubernetes/kubernetes",
      "labels": ["good first issue", "help wanted"],
      "channel": "kubernetes"
    }
  ]
}
```

- `id` must be unique and stable because it is the state key.
- `repository` uses `owner/name` syntax.
- `labels: []` matches every Issue.
- A non-empty list matches when any configured label is present. Matching is
  case-insensitive.
- `channel` refers to a secret destination alias, never a webhook URL.
- One repository may have several routes and several routes may share a channel.

Discord destinations are a single GitHub Actions Secret named
`DISCORD_WEBHOOKS`:

```json
{
  "kuberay": "https://discord.com/api/webhooks/...",
  "kubernetes": "https://discord.com/api/webhooks/..."
}
```

To add a repository or channel:

1. Add or update a monitor in `config.json`.
2. If the channel alias is new, add its Discord webhook URL to the
   `DISCORD_WEBHOOKS` secret.
3. Run the workflow manually once. The new route creates a silent baseline.

## GitHub setup

1. Create a new **public** GitHub repository for this project. It does not need
   to be a KubeRay fork.
2. Push this project to its default branch.
3. In Discord, open the destination channel's **Edit Channel → Integrations →
   Webhooks**, create a webhook, and copy its URL.
4. In GitHub, open **Settings → Secrets and variables → Actions**, create the
   `DISCORD_WEBHOOKS` repository secret, and paste the alias-to-URL JSON object.
5. Open **Actions → Monitor GitHub issues → Run workflow**. The first run should
   create only a state commit and no Discord messages.

The workflow is scheduled every five minutes. GitHub may delay a
scheduled workflow during high load. GitHub also automatically disables
scheduled workflows in inactive public repositories after 60 days; re-enable it
from the Actions page or make a small reviewed schedule commit.

The workflow uses `actions/checkout@v7` and `actions/setup-go@v7`, grants only
`contents: write`, and does not run on pull requests. Its write permission is
used solely to commit `state.json`.

## Local verification

No third-party Go packages are required.

```bash
gofmt -w cmd/notifier/*.go internal/*/*.go
go test -race ./...
go vet ./...
go build -o /tmp/github-issue-discord-notifier ./cmd/notifier
```

For a local run, use a test Discord webhook rather than a production channel:

```bash
DISCORD_WEBHOOKS='{"kuberay":"https://discord.com/api/webhooks/..."}' \
GITHUB_TOKEN='optional-token-for-higher-rate-limits' \
go run ./cmd/notifier
```

Optional overrides are `CONFIG_PATH`, `STATE_PATH`, and `GITHUB_API_URL`.
Webhook URLs and tokens must never be committed or printed.

## Operational behavior

- Invalid or unknown configuration fails before any network call.
- GitHub and Discord clients use 15-second HTTP timeouts.
- GitHub pagination is limited to 100 pages and rejects cycles or cross-origin
  pagination links.
- Discord and GitHub errors include a bounded response excerpt but never tokens
  or webhook URLs.
- One failed route does not prevent independent routes from running.
- Discord rate limits fail the affected route; the next scheduled run retries
  it instead of holding an Actions runner in a long sleep.
