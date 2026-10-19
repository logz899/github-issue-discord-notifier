# GitHub Issue Discord Notifier Summary

Status: approved
Last updated: 2026-10-19

## Goal

Notify Discord when a public GitHub repository gains a newly eligible Issue,
starting with every new Issue in `ray-project/kuberay` and expanding through
configuration to additional repositories, label filters, and channels.

## Current Decision

- Poll public repositories from a free scheduled GitHub Actions workflow because
  the operator cannot install webhooks on upstream repositories.
- Implement the notifier in Go using only the standard library.
- Keep public routes in `config.json` and Discord URLs in one encrypted
  `DISCORD_WEBHOOKS` JSON secret keyed by channel alias.
- Persist per-route checkpoints and delivered GitHub Issue IDs in committed
  `state.json`.
- Treat an empty label list as all Issues and a non-empty list as matching any
  configured label, case-insensitively.
- Establish a silent baseline on a route's first run rather than sending its
  historical Issues.
- Exclude pull requests and disable Discord mentions in Issue-controlled text.

## User-Facing Contract

- A new matching open Issue appears in the configured Discord channel within
  the normal five-minute polling interval.
- Each route delivers an Issue at most once, including after retries, relabels,
  closes, and reopens.
- A second repository or channel can be added without changing Go code.
- A route whose Discord delivery fails retries later without duplicating the
  route's already successful deliveries.

## Current Status

- Product design is approved and retained in both the original design document
  and this plan's `BLUEPRINT.md`.
- The shared `.agent` planning base and three-task implementation index exist.
- Task 01 is the only expanded task and no product code has been written.

## Next Action

Review Task 01, select an execution method, and implement the configuration and
durable-state foundation test-first.

## Changelog

- 2026-10-19: Created the implementation plan from the approved design.
