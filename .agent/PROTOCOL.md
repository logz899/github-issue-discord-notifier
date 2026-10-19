# Agent Plan Protocol

This protocol is the shared workflow for coding agents in this repository.
Root entry files should point here rather than duplicate project instructions.

## Startup

Before planned work:

1. Run `git status --short` and preserve unrelated user changes.
2. Read `.agent/PROJECT.md`.
3. Read the selected plan's `CURRENT.md`.
4. Read only the active task and the files listed under `Read first`.
5. Inspect additional code only when the task is ambiguous or verified code
   contradicts the plan.
6. Keep changes within the active task boundary.

## Feature Lifecycle

Non-trivial work follows this sequence:

1. Clarify goal, constraints, risks, and success criteria.
2. Record durable decisions in `SUMMARY.md` and `BLUEPRINT.md`.
3. Obtain user review of the written blueprint.
4. Split approved work into small numbered tasks.
5. Detail only the active or next task.
6. Implement behavior changes test-first.
7. Run the task's verification commands.
8. Update the task snapshot and `CURRENT.md`.

## Plan Layout

```text
.agent/plans/<feature>/
  CURRENT.md
  SUMMARY.md
  BLUEPRINT.md
  tasks/
    README.md
    01-<task>.md
```

`CURRENT.md` is an index, not a second plan. It contains the current stage,
active task, next action, narrow read set, and known blockers.

## Task Status

Use exactly one task status:

- `ready`
- `active`
- `blocked`
- `completed`
- `scoped-out`

Each detailed task contains, in order:

1. Task Snapshot
2. Execution Context
3. Interfaces
4. Current Contract
5. Implementation Steps
6. Verification
7. Changelog
8. Historical Details

## Change Discipline

- Separate structural changes from behavior changes.
- Do not overwrite or revert unrelated user work.
- Never expose secret values in commands, logs, patches, or chat.
- Use placeholders only in examples.
- Production behavior changes require tests before implementation.
- Before claiming completion, run the exact verification commands and inspect
  their output.

## External and Destructive Actions

Local implementation does not authorize external changes. Get explicit user
approval before creating a remote repository, adding or rotating credentials,
changing repository visibility, pushing, enabling workflows, or sending a
production Discord message.

Resolve exact targets before deleting files. Prefer a recoverable move when
removing untracked material.

## Commit Policy

Never auto-commit implementation work. Show a proposed commit message in chat
after verification; the user performs `git add` and `git commit` when ready.
