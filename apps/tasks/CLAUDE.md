# apps/tasks

## Purpose

One Go binary with three modes. Bare `tasks` prints usage, `tasks <verb>` acts and exits, and `tasks api` serves the JSON `apps/web` reads and the compiled client's files beside it. `docs/adrs/0003-replace-the-tui-with-a-browser-client.md` records why the person's surface is a browser client this binary serves; `docs/adrs/0004-tasks-md-is-the-store.md` records why the store is a `tasks.md` file in each Repo rather than a database.

This slice (#119) reads only: `list`, `repos`, `lint` and `GET /api/state`. Writes come back with #121 and #122, onto the file rather than a store.

## Ownership

- `src/cmd/tasks/` — `package main`, nothing but the call into `cli`. It sits under `cmd/` because `scripts/package` names the artifact after the main package's import path.
- `src/cli/` — mode dispatch and the verbs. `ModeOf` decides which mode an invocation asked for; `Run` takes its streams as arguments so every mode is testable without a process.
- `src/taskfile/` — the `tasks.md` format: `Parse` reads a file into its Task tree plus every problem by line, and `Write` gives the tree back as canonical text. It reads no disk.
- `src/repos/` — the config file and Repo discovery: `ConfigPath`, `Roots`, `Find`.
- `src/board/` — the one read of every Repo: discover, parse each file afresh, work out Blocked and each Task's parents, and keep what a `Narrowing` asks for. Every verb and route reads through it.
- `src/api/` — `tasks api`: routing, JSON encoding, the status an error takes, and the client's files. `GET /api/files` is the one route holding a rule of its own.
- `src/schedule/` — Scheduling's rule arithmetic: parsing a recurrence rule, writing it back, and the dates it produces. Nothing calls it yet; Series come back onto the file in a later ticket.
- `src/ai/` — the Broker client: an OpenAI-compatible HTTP call out to `inference-runtime-broker`. Nothing calls it until #123 brings the Broker routes back.
- `deploy/systemd/` — `tasks-api.service`, the `systemd --user` unit that starts `tasks api` on boot. `.unit.json` declares `ships: executable`.

## Local Contracts

- **A verb and a request make the same in-process call.** `tasks list` and `GET /api/state` both call `board.Discover` then `board.Read` with a `board.Narrowing`, so a person and an Agent are answered by one read. A rule enforced in a handler is one an Agent at a terminal never gets.
- **Roots come from `~/.config/tasks/config`** (`$XDG_CONFIG_HOME/tasks/config` when set), a plain key file read with the standard library: one `root = <path>` a line, `#` comments and blank lines ignored, `~` expanded to the home directory, a path that is neither absolute nor under `~` refused. Any other key is an error naming the line. No file, or no `root` line, means one root, `~/code`.
- **A Repo is a folder directly inside a root that holds a `tasks.md`.** Only direct children are looked at, so a nested clone, a worktree or a `tmp/` inside a Repo is never one. Dot folders are skipped. Links are followed, and two paths to one real folder are one Repo. One folder name under two roots is a config error naming both paths, and the first is kept.
- **The Repos are rescanned and every file re-read on each request.** Nothing is cached between calls, so a new Repo or a pull shows without a restart.
- **A config error never hides the Repos that were found.** `tasks list` prints the read and then the error, exiting 1; `GET /api/state` answers `200` with the sentences in `errors`.
- **The format is #97 and #105's.** A preamble (`# Tasks`, `color: <name>`, a comment), then title lines `- [ ] <title> | <state> #tags`, each followed in order by `- id:` (four of `[a-z0-9]`) and `- created:`, then any other `- label: value` lines (unknown labels kept, in order), a blank line, and the description. A Subtask is indented two spaces a level, five levels at most.
- **The checkbox wins over the word.** `[x]` is Done and `[-]` is Declined whatever the word says; `[ ]` under an ended word is Backlog.
- **A parent's State is worked out from its Subtasks.** While any is open it is the first of Doing, Backlog, Inbox and Deferred among them; once all have ended it is Declined if every one was, otherwise Done. A parent line saying otherwise is a problem on that line, and the read carries the worked-out State.
- **Blocked is any same-Repo blocker that has not ended.** `- blocked by: <id>, <id>` names Tasks in the same file; a blocker naming no Task is a lint problem and blocks nothing.
- **Lint is `taskfile.Parse`'s problems and nothing else.** Parse errors, duplicate ids, unknown blockers, a missing or out-of-order `id`/`created`, an id of the wrong form, a parent line disagreeing with its Subtasks, and conflict markers, each by line. `tasks lint [file...]` prints `file:line: message` and exits 1 on any; with no files it lints every Repo found. The read carries the same list as each Repo's `problems`, so a broken Repo is flagged where it is drawn rather than hidden.
- **One wire shape for `tasks list -json` and `GET /api/state`.** `board.Of` builds it: `{"repos": [{name, path, color, tasks, problems}], "errors": [], "sorts": [...], "today": "YYYY-MM-DD"}`; a Task is flat, each Task before its Subtasks, carrying `id, title, state, tags, created, attrs, description, line, depth, parent, parents, leaf, blocked, rank`, with `parents` the titles above it, outermost first. An empty collection is `[]`, never `null`.
- **A narrowing is the same six things on both surfaces.** `-repo`/`repo`, `-state`/`state` and `-tag`/`tag` repeated (any of them), `-search`/`search` (case-insensitive over id, title and description), `-unblocked`/`unblocked=true`, `-sort`/`sort`. A State or a sort there is not is refused through `Narrowing.Check`.
- **A sort orders siblings and never flattens the tree.** `board.Sorts` is the one list (`file title deadline created priority estimate`), and the read serves it as `sorts`. Priority ranks high, med, low and estimate S, M, L; a Task missing the key sorts last; ties keep file order. `rank` is each Task's place across every Repo (sort key, then Repo, then file), so a surface drawing several Repos in one lane orders by it.
- **`GET /api/state`'s ETag hashes the host's date, every Repo's name, path, file mtime and size, the config errors and the query.** The date is there because `today` is in the body, so past midnight the same files are a different response. A different view or a changed file is never answered `304` for another. A bad query and an unknown Repo are refused before the ETag is looked at, so `If-None-Match: *` cannot be told nothing changed about a view it can never be shown. The tag goes only on a `200`. `api.matches` reads `If-None-Match` as RFC 9110 writes it.
- **The API's status codes are the CLI's exit statuses.** `200` for done, `400` for usage (exit 2), `404` for a Repo that is not there (exit 1), `500` for a failure (exit 1), with the body `{"error": "<the sentence the CLI would print>"}`. `api.fail` is the one place that mapping is written.
- **The client's files are served under `/api/`'s feet, not over them.** The fallback answers any path with `index.html`; `/api/` is registered ahead of it, with or without `-web`, so a route this package does not serve is `400` with a sentence rather than the client at `200`. One `http.Dir` both decides whether a path names a file and serves it.
- **`GET /api/files` lists one directory under the listener's home and opens nothing.** `within` judges a path on how it is spelled and then on where it lands, so a link out of the root is refused the way `..` is. Everything outside gets one sentence, there or not. `Options.Browse` is a test seam, not an operator knob.
- **`tasks api` has no authentication, deliberately, and is meant for the LAN.** Changing that is what ADR 0003's first re-check trigger covers. `:8080` is a wildcard bind, so the host's interfaces decide its reach.
- **Two artifacts are placed on the deploy host, and nothing enforces they match.** The binary `scripts/package tasks` builds goes to `~/.local/bin/tasks`, and `apps/web/dist/` goes to `~/.local/share/tasks/web/`, which the unit's `-web` names.
- **Dates are host-local.** `board.Today` is the host's local date, served as `today` so a surface marks a Deadline today or Overdue against the host rather than the browser; a write or the pass uses `time.Local` the same way.
- **Bare `tasks` is usage, exit 2.** `cli.verbs` is the one list of verbs; the dispatch and the usage sentence are both written from it.
- **The AI is an outbound HTTP call, never embedded inference.** `TASKS_AI_URL` and `TASKS_AI_MODEL` point it elsewhere; a 429 is the broker's memory budget and is reported as a wait.
- **A rule is written the way it is said, and `schedule.Parse` is the only reader of one.** `String` writes every form back exactly as `Parse` reads it.

## Work Guidance

- Tests first, at the seams: `taskfile` table tests of text to Tasks and problems plus the write-back round trip; a temp root of Repo folders driven through `cli.Run`; `GET /api/state` under `httptest`.
- A test owns its home: set `HOME` and `XDG_CONFIG_HOME` to a temp directory so discovery never reads the real `~/code`.
- The broker is shared and its chat model is large: a 429 saying `scheduler_busy` means the GPU budget is spoken for. Tests stand a broker in with `httptest` and never reach the LAN.
- The exported vocabulary is `CONTEXT.md`'s: Repo, Task, Subtask, State, Tag, Blocked, Series, Occurrence, Actor.

## Verification

`go test ./...` from this directory, and `scripts/check` from the repository root for the gate CI runs.

## Child Index

None.
