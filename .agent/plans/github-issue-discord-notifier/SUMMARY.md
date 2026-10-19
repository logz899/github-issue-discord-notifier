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
  the normal thirty-minute polling interval.
- Each route delivers an Issue at most once, including after retries, relabels,
  closes, and reopens.
- A second repository or channel can be added without changing Go code.
- A route whose Discord delivery fails retries later without duplicating the
  route's already successful deliveries.

## Current Status

- Product design and all three implementation tasks are complete.
- The Go packages, CLI, serialized thirty-minute workflow, sample configuration,
  durable state, tests, and setup documentation are implemented locally.
- Whole-branch review found no critical issues; its default-branch workflow
  finding and smaller validation/documentation findings were addressed.
- No remote repository, Actions Secret, workflow, or Discord channel has been
  created or changed.

## Next Action

Choose whether to keep the local branch, create/push a GitHub repository, or
integrate it into the local main branch.

## Changelog

- 2026-10-19: Created the implementation plan from the approved design.
- 2026-10-19: Completed implementation, verification, and final review fixes.
