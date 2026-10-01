---
type: Architecture Decision Record
title: TASKS.md Keeps Its Own Git History, Apart from the Code's Branches
description: Each Repo's TASKS.md is ignored by the Repo's code history and tracked by a second git history in the same folder, pushed after every write to a `tasks` branch that nothing merges.
scope: [global, apps/tasks]
tags: [storage, git, durability]
generated: { by: "agent/claude-opus-5-5", at: "2026-09-30T00:00:00Z" }
superseded_by:
status: accepted
---

# TASKS.md Keeps Its Own Git History, Apart from the Code's Branches

## Decision

This amends ADR 0004's **History** and **Discovery** points; the rest of ADR 0004 stands.

- A Repo's code history ignores `TASKS.md`. A second git history in the same folder (`.tasks.git`, also ignored) tracks `TASKS.md` alone, so the file sits at the Repo's root, the same on every code branch.
- Every write through `tasks` is one commit to that history, then a push to a `tasks` branch on the Repo's own remote. The `tasks` branch holds only `TASKS.md`; nothing merges into it and it merges into nothing.
- A push that fails or is refused does not fail the write: the commit stays local, the board marks the Repo not pushed, and the next write retries. A Repo with no remote, such as one that holds no code, stays local and the board marks it not backed up.
- The template and a one-time operator step put `/TASKS.md` and `/.tasks.git` in a Repo's `.gitignore`. A write that finds a Repo with a code history ignoring neither appends them itself, leaving one uncommitted `.gitignore` change rather than a tasks history a `git add -A` would commit into the code.
- A linked worktree has no `TASKS.md` of its own; `tasks` run there reads and writes the main checkout's.
- Writes are refused, and the Repo flagged, while `TASKS.md` holds conflict markers or its history is mid-merge, mid-rebase or on a detached HEAD.

## Context

Issue #108 first settled that a write commits to whatever branch the main checkout has out, and that `tasks` never pushes. The operator asked for a design that never loses information. With task commits on code branches, a deleted unmerged branch took its Task changes with it, a branch switch hid Tasks from the board, and two branches' edits collided at merge. With no push and no host backup, a dead disk on `dev` took every unpushed change. On 2026-09-29, three of the four main checkouts under `~/code` sat on feature branches.

## Alternatives Considered

- **Commit to the checked-out branch, push by hand** (#108 as first settled). Rejected for the losses above.
- **Always commit to the default branch.** Rejected: the `main` ruleset requires a pull request, so task commits could never be pushed without weakening it for every writer holding the operator's credentials.
- **One tasks repo for every Repo, linked into each folder.** It would push Repos that hold no code and replace scanning. The operator chose to keep each Repo's Tasks with that Repo's own remote.
- **Keep #108 and back up `dev` nightly.** Covers the disk, not stranded branches or branch switches.

## Consequences

- `git log TASKS.md` in the code checkout shows nothing; the history is `git --git-dir=.tasks.git log`, or the `tasks` branch on the remote. Code pull requests never carry task commits.
- Editor and TUI git panels do not show task changes.
- Another machine gets a Repo's Tasks by fetching its `tasks` branch into its own `.tasks.git`.
- The lint that #100 put in the template's pre-commit hook cannot run on code commits; it runs on `tasks`'s writes and on `tasks lint`.
- `TASKS.md` is added to the ignored paths of every existing Repo, and `.tasks.git` beside it.
- An editor holding an old copy can still save over a `tasks` write. The overwritten version remains in the history, so it is recoverable, but nothing announces it.
