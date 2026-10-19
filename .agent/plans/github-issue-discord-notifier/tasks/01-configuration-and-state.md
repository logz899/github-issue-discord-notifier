# Task 01: Configuration and Durable State

**Goal:** Establish the dependency-free Go module and strict, safe contracts for
multi-repository routing, secret destination lookup, and durable per-route
delivery state.

## Task Snapshot

Status: ready
Last updated: 2026-10-19

Current decision:

- Use separate `config` and `state` packages so network code never owns file or
  environment decoding.
- Normalize IDs, repositories, labels, and channel aliases during validation;
  persist the normalized values used by later packages.
- Reject unknown JSON fields and unsupported state versions.
- Save state through a same-directory temporary file and atomic rename.

Current focus:

- Create the module, tests, configuration/state implementations, and initial
  KubeRay/sample state files without making network calls.

Exit criteria:

- Focused tests prove all parsing, validation, normalization, state-version,
  corruption, and atomic-save contracts; `go test ./...` and `go vet ./...`
  pass.

## Execution Context

Read first:

- `.agent/PROJECT.md`
- `.agent/plans/github-issue-discord-notifier/BLUEPRINT.md`
- `.agent/plans/github-issue-discord-notifier/tasks/README.md`
- `docs/superpowers/specs/2026-10-19-github-issue-discord-notifier-design.md`

Read only if needed:

- `.agent/templates/task.md`

Do not read:

- unrelated repositories under `/Users/kevinlin/program`
- future task files until they are created
- GitHub or Discord credentials

Task boundary:

- May create `go.mod`, `config.json`, `state.json`, `internal/config`, and
  `internal/state`.
- Must not add HTTP clients, monitoring orchestration, CLI execution, workflow
  YAML, or contact external services.

## Interfaces

- Consumes: `config.json`, the raw `DISCORD_WEBHOOKS` JSON string, and
  `state.json` using the blueprint schemas.
- Produces:
  - `config.Monitor { ID string; Repository string; Labels []string; Channel string }`
  - `config.Config { Monitors []Monitor }`
  - `config.Destinations map[string]string`
  - `config.Load(path string) (Config, error)`
  - `config.ParseDestinations(raw string) (Destinations, error)`
  - `config.Validate(cfg Config, destinations Destinations) (Config, error)`
  - `state.Monitor { LastSuccessfulPoll time.Time; NotifiedIssueIDs []int64 }`
  - `state.State { Version int; Monitors map[string]Monitor }`
  - `state.Load(path string) (State, error)`
  - `state.Save(path string, value State) error`

Later tasks consume only these exported types and functions; JSON field names
remain `monitors`, `id`, `repository`, `labels`, `channel`, `version`,
`last_successful_poll`, and `notified_issue_ids`.

## Current Contract

- `config.Load` accepts one JSON document, rejects trailing documents and
  unknown fields, and returns descriptive errors without file content.
- `config.ParseDestinations` requires a non-empty JSON object whose keys and
  values are non-blank; errors never include webhook URLs or the raw input.
- `config.Validate` trims values, lowercases repository owner/name only for
  comparison while retaining GitHub-compatible normalized text, folds labels
  for matching, rejects duplicate IDs and labels after normalization, enforces
  exactly one non-empty `/`-separated owner and repository, and requires every
  channel alias to exist.
- A config with zero monitors is rejected.
- `state.Load` accepts version `1`, rejects unknown fields, future or missing
  versions, negative Issue IDs, duplicate Issue IDs per route, zero checkpoint
  values for initialized routes, corrupt JSON, and trailing documents.
- Missing `state.json` is not silently treated as empty; the committed initial
  file is required.
- `state.Save` writes deterministic indented JSON plus a trailing newline,
  creates a temporary file in the destination directory, syncs and closes it,
  and renames it over the target without exposing a partially written target.
- Initial `config.json` contains only `kuberay-all-issues` for
  `ray-project/kuberay`, empty labels, and the `kuberay` channel alias.
- Initial `state.json` is version `1` with an empty `monitors` object.

## Implementation Steps

- [ ] Create `go.mod` with module
  `github.com/logz899/github-issue-discord-notifier` and no third-party
  dependencies.
- [ ] Write failing table tests for valid normalization and all configuration
  rejection cases in Current Contract, including verification that secret
  values never appear in errors.
- [ ] Implement the exact `internal/config` interfaces, run
  `go test ./internal/config -v`, and confirm all configuration tests pass.
- [ ] Write failing state tests for valid load, corrupt/unsupported state,
  duplicate or negative IDs, deterministic save, and preservation of the old
  file when saving fails.
- [ ] Implement the exact `internal/state` interfaces with strict JSON and
  same-directory atomic replacement, then run `go test ./internal/state -v`.
- [ ] Add the initial `config.json` and `state.json`, plus a repository-seed
  test that validates both using a placeholder destination.
- [ ] Run the complete Task 01 verification block and inspect its output.
- [ ] Update the plan state, expand compact Task 02 details, and propose commit
  message `feat: add notifier configuration and state foundation`; do not
  commit automatically.

## Verification

```bash
gofmt -w internal/config/*.go internal/state/*.go
go test ./...
go vet ./...
git diff --check
```

Expected result: all tests pass, vet and diff checks are silent, no dependency
appears outside the standard library, and neither package performs a network
request.

## Changelog

- 2026-10-19: Initial Task 01 plan created from the approved blueprint.

## Historical Details

No historical body yet.
