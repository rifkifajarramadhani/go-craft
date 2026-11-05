# Repository guidelines

`go-craft` is an offline generator for Clean Architecture Go services. Read
[README.md](README.md) for the project overview, generation options, and
template maintenance workflow.

This guide applies to work on the generator. The
[starter AGENTS.md](internal/scaffold/_assets/template/AGENTS.md) and
[starter CLAUDE.md](internal/scaffold/_assets/template/CLAUDE.md) ship to
generated projects. Keep their commit workflow consistent with this guide
and with the `golang-clean-architecture` Template Source.

## Commit workflow

Use concise, imperative Conventional Commit subjects in the form
`type(scope): summary`, for example `docs(agents): require incremental commits`.
Keep each commit focused on one logical change. Use a commit body when it
explains the reason or important implementation details.

Before implementation, split each task into small, independently reviewable
commit checkpoints. Record checkpoints in an existing task plan when one
applies. A simple change with one checkpoint needs only one commit.

For each checkpoint:

1. Implement one logical change with its required tests and documentation.
2. Run the relevant checks and fix any failures.
3. Review and stage only files or hunks for that checkpoint. Exclude
   pre-existing and unrelated worktree changes.
4. Commit immediately after the checks pass, before starting the next
   checkpoint. Do not wait until the parent task, feature, or phase is complete.

Update the matching task checkbox or status only in the commit that completes
that task.

## Verification

Review the diff and run `git diff --check` for documentation changes. Run
`make check` for generator or embedded-template changes.
