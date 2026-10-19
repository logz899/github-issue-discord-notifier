# GitHub Issue Discord Notifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use
> `superpowers:subagent-driven-development` (recommended) or
> `superpowers:executing-plans` to implement this plan task-by-task. Every
> behavior change uses `superpowers:test-driven-development`, and steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a free, scheduled Go notifier that routes newly eligible
public GitHub Issues to Discord without duplicates and expands through
configuration to multiple repositories and channels.

**Architecture:** A dependency-free Go command composes focused configuration,
state, GitHub, Discord, and monitoring packages. GitHub Actions serializes
five-minute polls and commits versioned JSON state; route IDs scope checkpoints
and delivered Issue IDs so partial failures retry safely.

**Tech Stack:** Go standard library, GitHub REST API, Discord incoming webhooks,
GitHub Actions, JSON, `testing`, and `httptest`.

**Spec:** `.agent/plans/github-issue-discord-notifier/BLUEPRINT.md`

## Global Constraints

- Use module path `github.com/logz899/github-issue-discord-notifier` and the Go
  version selected in Task 01 from the locally installed stable toolchain.
- Use only the Go standard library in v1.
- Keep webhook URLs only in `DISCORD_WEBHOOKS`; never print URLs, tokens, or raw
  secret JSON.
- Empty labels match all Issues; otherwise a route matches any configured label
  case-insensitively after trimming whitespace.
- Pull requests and non-open Issues never produce Discord notifications.
- First execution for a route silently records its matching open Issues.
- A successful Issue is delivered at most once per route.
- State is versioned JSON and is replaced atomically; corrupt state is fatal.
- Do not mutate any remote repository, secret, workflow, or Discord channel
  without separate explicit approval.

## Review Focus

- A corrupt or future-version `state.json` must stop execution without replacing
  user state; Task 01 owns these tests.
- Whitespace-only aliases, duplicate monitor IDs after trimming, malformed
  `owner/name`, and unknown destination aliases must fail before network calls;
  Task 01 owns these tests.
- Cyclic or excessive GitHub pagination must fail rather than loop or silently
  truncate; Task 02 owns these tests.
- Markdown mention syntax and oversized Unicode Issue fields must not ping users
  or exceed Discord limits; Task 02 owns these tests.
- A route with mixed successful and failed Discord deliveries must persist the
  successful IDs but retain its old checkpoint, while independent routes still
  run; Task 02 owns these tests.

## Ordered Tasks

| Task | Name | Depends on | Deliverable |
|---|---|---|---|
| 01 | [Configuration and durable state](01-configuration-and-state.md) | Approved blueprint | Go module, validated multi-route configuration, initial KubeRay route, versioned state, and atomic persistence |
| 02 | Notification pipeline | Task 01 | GitHub pagination, Discord delivery, silent baseline, label filtering, deduplication, partial-failure recovery, and focused tests |
| 03 | Executable and automation | Tasks 01–02 | CLI wiring, safe logs/exit behavior, serialized five-minute Actions workflow, setup/extension README, and full verification |

Tasks execute in order. This is intentionally a compact plan for a small
application: only the active task is expanded, and its checklist describes
behavioral milestones rather than individual functions. Completing local
implementation does not authorize remote setup, push, workflow activation, or
a real Discord send.
