# CLAUDE.md

Follow [AGENTS.md](AGENTS.md) for repository guidance and [README.md](README.md)
for generator behavior, template maintenance, and verification commands.

The instruction files under `internal/scaffold/_assets/template/` ship to
generated projects. Keep their commit workflow aligned with the root guides
and the `golang-clean-architecture` Template Source.

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
