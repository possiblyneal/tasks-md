## Template Contract

These sections are identical in every repository built from the template. Do not edit them here: a repository whose shape needs to differ has found a gap in the template, so fix it there and let the next template update (`/repo-builder`) carry it. This repository's own instructions start at Purpose.

### Commands

Use these instead of per-language tools. `check` and `fix` run for every language present and report `pass`, `not-applicable`, `unavailable`, or `FAIL`.

- `scripts/check` — the full local gate, a superset of CI except the pull request's changelog check: doctor, script tests, structure, lint, format, type check, test, build, security audit, pre-commit over every file. Prints only the Result table, with the full run in `tmp/check.log`; `--verbose` streams everything
- `scripts/fix` — lint autofix, then formatting
- `scripts/summarize <command…>` — run a check and print only its Result table; read `ci` and other commands through this, since `check` already does
- `scripts/run [unit] [--entry <name>] [-- args…]` — start a unit; the unit name is required when `apps/` holds several

`ls scripts/` lists the rest, such as `doctor`, `integration`, and `package`.

### Layout

`scripts/check` fails on any of these; pre-commit runs `scripts/structure` over the placement rules on every commit.

- Root holds only `apps/`, `docs/`, `libs/`, `scripts/`, `tests/`, `tools/`, `deploy/`, `assets/`, `gradle/`, `tmp/`, and the dot-folders `.claude/`, `.github/`, `.devcontainer/`.
- Root files are permitted by name: the shipped dotfiles, `CLAUDE.md`, `GLOSSARY.md`, `GLOSSARY-MAP.md`, `.gitmodules`, `.structure-allow`, `.repo-template.json`, adopted addons, and each language's root manifests and lockfiles. Any other root file or folder needs an entry in `.structure-allow`.
- `apps/<unit>/` is one unit: the smallest piece delivered on its own. Its code goes in `apps/<unit>/src/`. Nothing sits directly in `apps/`.
- Split a unit into domains only for distinct business areas: `apps/<unit>/<domain>/src/`. A unit has `src/` or domains, never both. Domains do not nest.
- A root, unit, or domain may hold its own `libs/`, `tests/`, `scripts/`, `tools/`, `assets/`, `docs/`, or `deploy/`, scoped to it. These never contain `src/`.
- `tools/` holds one directory per program, never loose files.
- `deploy/` holds only `quadlet/`, `containerfile/`, `compose/`, `systemd/`, and `env/`.
- `docs/` takes Markdown; anything else there needs `.structure-allow`.
- Below a `src/`, the language decides the layout.
- A path ending in `/` in `.structure-allow` exempts that whole prefix, for vendored or fixture trees.
- A package whose language has no root manifest fails the gate; add the root workspace manifest (`go.work`, `settings.gradle.kts`, and so on) that lists it. Swift is excepted: it has no root manifest.

These are conventions `scripts/structure` does not check:

- Root-level `libs/`, `tests/`, `scripts/`, `tools/`, `deploy/`, and `assets/` hold only what spans several units.
- Files a program reads go in `assets/`. Files it writes while running go in `apps/<unit>/state/`, which is gitignored.
- `tests/integration/` is the integration tier, run by `scripts/integration` and left out of the gate.

Every unit has a `.unit.json` at its root, never at a domain's:

```json
{ "schema_version": 1, "run": "oneshot", "ships": { "kind": "executable", "targets": ["linux-amd64"] } }
```

- `run`: `oneshot` (exits on its own), `longlived` (runs until stopped), or `none`.
- `ships.kind`: `executable`, `quadlet`, or `none`. `ships.targets` (`linux-amd64`, `macos-arm64`) is required for `executable` and rejected otherwise.
- Any pairing of `run` and `ships` is valid.

## Purpose

`tasks` is a task tracker with two kinds of consumer: a person at a keyboard, and agents that add, edit, and delete while nobody is watching. Both reach the same store through the same calls; neither gets a weaker or a stronger contract than the other. `GLOSSARY.md` holds the language that keeps those two from meaning different things by the same word, and `docs/features.md` records the operator's wish list verbatim as source material rather than as a specification.

The repository holds **two deployables**. Three bounded contexts — Tracking, Scheduling, Change History — collapse into a single Go binary, `apps/tasks`, with three modes: bare `tasks` prints usage, `tasks <verb>` acts and exits, and `tasks api` serves the JSON the browser client reads. `apps/web` is that client, and it builds to static files `tasks api` serves beside the JSON. The store is a `TASKS.md` file in each Repo, a folder directly inside a configured root (`docs/adrs/0004-tasks-md-is-the-store.md`), so it is not a deployable of its own.

`docs/adrs/0003-replace-the-tui-with-a-browser-client.md` supersedes ADR 0001: the person's surface is a TypeScript browser client in `apps/web` over a LAN-only JSON API served by the same Go binary. The TUI and `tasks serve` are deleted, so `apps/web` is the only thing a person looks at and `tasks api` is what serves it. `docs/plans/browser-client.md` is the order that work landed in, and every stage of it is done. The two are started on a host by `apps/tasks/deploy/systemd/tasks-api.service`, a `systemd --user` unit running the built binary against the compiled client's files; `apps/web` declares `ships: none` because those files are placed rather than executed, and the README's Install section is the sequence.

**Three things are what this tracker is for, and the rest is support for them.**

- **A dump becomes a Task, and the Broker is what reads it.** Somebody says a Task the way they think of it, most often typed one-handed into a phone, and the Broker turns the sentence into the attributes; the ordinary add form opens filled in, and a person corrects and submits it. That is the usual way a Task is created rather than a shortcut past the form, and it is the single most frequent thing anybody does here. On a surface a person is at, the form stays the gate: nothing the Broker read is written until they submit it. `tasks capture` is the one verb that writes what the Broker said without one, because a dump is a person's own words and there is nobody left to ask.
- **A question about the list is answered by the Broker.** The question goes with the Tasks in view and comes back as prose. It is a read: nothing written, nothing stored between calls.
- **A Task is broken into Subtasks by the Broker.** Proposals are shown, ticked or unticked, and only submitting writes. There is no Assistance aggregate and no artifact: an approved proposal is the same entry in the Change History a hand-written Subtask is.

Those three are first-class, and a surface that reaches them in fewer taps than it reaches anything else is the surface doing its job. So is **an Agent acting unattended**: adding, editing and completing through the same calls a person's surface uses, into the same Change History. Neither consumer gets the weaker contract, and that symmetry is the property worth protecting when a surface is added. What an Agent did unattended is visible afterwards: the Change History says which Actor wrote what, and a person's surface draws it rather than leaving the writes to have merely appeared.

**The list is the one thing the Broker does not own.** A person browses it, filters it, sorts it and searches it themselves, and opens a Task to see its detail and what has happened to it. That part is looked at rather than spoken to, and it stays that way.

## Ownership

## Local Contracts

There is a root manifest per toolchain, and each is what makes its nested package visible to every check: `go.work` for the module under `apps/tasks/`, and `package.json` naming `apps/web` as an npm workspace. A nested manifest with no root one above it is an orphan `scripts/doctor` fails on rather than passing over, so a new package gets its root entry in the same commit that creates it.

The node checks dispatch off the root `package.json`'s scripts — `lint`, `format:check`, `format`, `typecheck`, `test`, `build` — each delegating to `apps/web`. A missing script reports `unavailable`, which fails the gate the same way a failure does, so renaming one here is renaming a check. `dev` is the seventh script and the one no check reads: it is there so the client's dev server starts from the root the other six run from.

### Agent skills

The engineering skills (`/wayfinder`, `/triage`, `/ship`) carry the GitHub Issues, `gh`, and seven-role label defaults. A repository that differs records a departure in `.repo-template.json` `generation.features`; this repository records none. ADRs are in `docs/adrs/`.

### Session launcher

Orca, on this machine. See `docs/agents/session-launcher.md`.

## Work Guidance

## Verification

## Child Index

- `apps/tasks/CLAUDE.md` — the Go deployable: its package layout, the store's enforced rules, and what each mode owns
- `apps/web/CLAUDE.md` — the browser client: what it draws, what it refuses to work out for itself, and the npm scripts the checks dispatch on

`docs/` owns a `CLAUDE.md` once it holds enough to need a local contract.
