---
type: Architecture Decision Record
title: A tasks.md File in Each Repo Is the Store
description: Every Task lives in exactly one tasks.md at the root of one Repo, git history replaces the Change History, a stale write is refused instead of a Lease being held, and Google Tasks syncs both ways against the files.
scope: [global, domain]
tags: [storage, domain-model, concurrency, sync]
generated: { by: "agent/claude-opus-5-5", at: "2026-09-29T00:00:00Z" }
superseded_by:
status: accepted
---

# A tasks.md File in Each Repo Is the Store

## Decision

The store is a `tasks.md` file at the root of each **Repo**: any directory that
holds one, whether or not it holds code. A Task lives in exactly one Repo. The
SQLite store, the Lease and the Change History as a stored record are retired.

- **History** is git. Every write through the tracker is a commit to that
  `tasks.md` authored by the Actor, and the tracker runs `git init` on a Repo
  that is not already a git repository.
- **Concurrency** is a stale-write refusal: a write made against an older copy
  of the file is rejected and the writer re-reads. Nothing is silently lost.
- **Agents** prefer the `todo` verbs, which carry the refusal and the commit. A
  direct edit to `tasks.md` is still valid and is picked up.
- **Discovery** scans root folders on the host running `todo api`. That host's
  checkouts are the Tasks the web page and Google Tasks see; other clones
  converge through ordinary git pull and push.
- **Google Tasks** syncs both ways.

It does not decide the file's grammar, which Task attributes survive, the Google
Tasks mapping, or how existing SQLite data moves. The wayfinding map
"tasks.md as the store: v1.0" carries those.

## Context

On 2026-09-03 (issue #5) the operator withdrew "markdown is the store" in favor
of SQLite. On 2026-09-29 the operator reversed that. The tracker is now wanted
where the work is: one file per repo that a person reads in an editor, an Agent
working in that repo reads and edits like any other file, and `grep` answers
without the whole file being read. A central database outside the repo is the
thing that prevented all three.

## Alternatives Considered

- **Store wins, file mirrors.** This kept the Lease and the Change History and
  imported edits made to the file. It was rejected because two sources of truth
  for one Task are what the operator wanted gone.
- **The file is a read-only export.** It was rejected because an Agent editing
  its own repo's `tasks.md` is the workflow this exists for.
- **Last write wins.** It was rejected because three writers touch one file:
  the web page, an Agent, and Google sync.

## Consequences

- The Lease is gone, so ADR 0002's tree-scoped Lease no longer holds. Whether a
  parent may complete while its Subtasks are open has to be decided again, as a
  rule of the file.
- A Task can no longer sit in two Lists. Cross-cutting grouping is left to Tags.
- `CONTEXT.md` terms that assume a single store change: List becomes Repo, and
  Lease, Change History and Actor are redefined. Each term is rewritten when the
  map ticket that settles it closes.
- A direct edit bypasses the stale-write refusal, so a rare clash between an
  Agent's hand edit and a web edit can overwrite one of them.
