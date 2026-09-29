# Source features: five Markdown-task tools

Extracted 2026-09-29 for a feature-by-feature decision session. Source tags used in the catalog:

- **GT**: google/gtasks-md
- **BM**: BaldissaraMatheus/Tasks.md (kanban board)
- **TM**: tasksmd/tasks.md (TASKS.md agent-queue spec + tooling)
- **RD**: Reddit post "TASKS.md changed how I use Claude Code, less lies" (**unreachable**, see §4)
- **WEB**: https://tasks.md/ (TasksMD, a separate browser app, unrelated to TM despite the name)

Read from: READMEs, docs, and source (GT: `__main__.py`, `parser.py`, `googleapi.py`, `backup.py`, `editor.py`; BM: `server.js`, `App.jsx`, `card-content-utils.js`, `expanded-card.jsx`, `card.jsx`, locale `en.js`, `KEYBOARD_SHORTCUTS.md`, `migration-guide.md`; TM: `README.md`, `spec.md` v1.0, `VISION.md`, package READMEs for cli/mcp/lint, user story 06; WEB: landing HTML plus strings from the shipped WASM bundle, because it has no public docs or repo).

---

## 1. google/gtasks-md

**What it is:** a Python CLI that shows all of your Google Tasks as one Markdown document. You edit the document declaratively and the tool reconciles your edits back to the Google Tasks API.

### Features

1. **`view`**: downloads every task list, renders it as Markdown, and prints it to stdout.
2. **`edit`**: renders the Markdown, opens it in `--editor` (falling back to `$VISUAL`, then `$EDITOR`, then `vim`) as a temporary `.md` file, parses the result on exit, and reconciles it with the server. A non-zero editor exit aborts the run.
3. **`reconcile <file>`**: the same reconciliation, but it reads a Markdown file you supply instead of opening an editor. This makes an offline or scripted source of truth possible.
4. **`rollback`**: re-applies the most recent local backup through `reconcile`, which undoes the last change.
5. **Rotating backups**: before every `edit` or `reconcile`, the server state as it was is saved to `$XDG_CACHE_HOME/gtasks-md/<user>/<0-9>.bak.md`. It is a ring of 10 files with a `marker` file pointing at the newest, and `rollback` walks the marker backwards.
6. **`auth <credentials.json>`**: stores the OAuth desktop-client credentials in `$XDG_DATA_HOME/gtasks-md/<user>/credentials.json`. The first API call runs the browser OAuth flow (`run_local_server(port=0)`) and caches `token.json` in the cache dir, refreshing it automatically. The scope is `https://www.googleapis.com/auth/tasks`. The README walks you through creating your own GCP project.
7. **Multi-account**: `--user <name>` on every command (default `default`) namespaces the credentials, token, and backups for each Google account.
8. **Completed-task window**: `--completed-after YYYY-MM-DD` defaults to one week ago, so older completed tasks are hidden. There is also `--completed-before YYYY-MM-DD`.
9. **Status filter**: `--status needsAction|completed` fetches only pending tasks or only completed ones.
10. **Document format** (a CommonMark subset parsed with markdown-it plus the GFM tasklists option, and formatted by mdformat with consecutive numbering):
    - An `# H1` at the top is ignored on parse. The renderer emits `# Google Tasks`.
    - `## <Task list title>` maps to one Google Tasks list. Lists are sorted by title when fetched.
    - A task is an **ordered** list item with a checkbox: `1. [ ] Title` means `needsAction`, `1. [x] Title` means `completed`. With no checkbox the status is `unknown`.
    - **Notes**: every block in the list item after the title paragraph, except a trailing ordered list, becomes the task's `notes` verbatim (dedented). Unordered sublists count as notes.
    - **Subtasks**: an ordered list nested at the end of a task item. Google Tasks allows only one level, which maps to the API's `parent` field.
    - **Order**: the position in the ordered list maps to the API's `position`. Anything else at the top level, such as stray paragraphs, raises `SyntaxError`.
    - **Not mapped**: due dates, links, and attachments. The `Task` model carries only id, title, note, position, status, and subtasks.
11. **Reconciliation algorithm**: items are matched **by title** at every level. Old items start as DELETE, a new item with the same title becomes UPDATE, and an unmatched new title becomes INSERT. Consequences:
    - Renaming a task is a delete plus an insert, so its id and any Google-side fields are lost.
    - Duplicate titles collapse into one.
    - An UPDATE is sent only when title, notes, status, or subtasks differ. It recurses into subtasks.
12. **Reorder pass**: after the changes are applied, incomplete tasks are moved into document order with `tasks.move(parent, previous)`, one call at a time because concurrent moves can't point at the same predecessor. Completed tasks are not reordered, because Google rejects moving hidden or cleared ones.
13. **Concurrency and batching**: each task list is reconciled in its own thread. Task operations within a list go in one batched HTTP request. Per-item failures are logged, not fatal.
14. **Conflict handling**: there is none beyond what the design implies. It is last-writer-wins against a fetch taken at the start of the command, with no optimistic check. The backup ring plus `rollback` is the safety net.
15. **Logging**: `$XDG_CACHE_HOME/gtasks-md/log.txt` records every insert, update, delete, and move.
16. **Install**: `pip`/`pipx install gtasks-md`, or a Nix flake dev shell.

---

## 2. BaldissaraMatheus/Tasks.md

**What it is:** a self-hosted kanban board (SolidJS front end, Koa back end, a single Docker image) in which every lane is a directory and every card is a Markdown file on disk.

### Features

1. **Lanes are directories**: every lane you create is a folder under `TASKS_DIR` (`/tasks` in Docker). You can create, rename, and delete lanes. Deleting a lane removes the folder and its cards, and there is a "delete all cards in lane" action.
2. **Cards are files**: each card is `<lane>/<card name>.md`. The card title is the filename without `.md`, and the card body is the file content. Names are validated: required, no leading dot, unique, no forbidden characters, no `.md` suffix typed in.
3. **Sub-boards**: a subdirectory can be opened as its own project at URL `/<subdir>`, with its own lanes and cards. The nesting is arbitrary. Hidden `.`-prefixed files are ignored.
4. **Deep link to a card**: the URL `/<board>/<card>.md` opens that card expanded.
5. **Card content editor**: a Stacks-Editor with a **Markdown mode / rich-text mode** toggle. The last mode used is remembered in `localStorage` as `lastEditorModeUsed`. It autosaves on change and has a maximize/minimize dialog.
6. **Tags inline in content**: tags are written `[tag:name]` anywhere in the file. Adding a tag from the UI prepends `[tag:name] ` (with a blank line after the first one). Tags are case-insensitive for duplicate detection. The v2 syntax was `tags: a, b` and a migration guide covers the change.
7. **Tag colours**: a tag's colour is picked by hashing its name into the palette `--color-alt-1..7`. You can override it per tag, and overrides are stored in `CONFIG_DIR/tags.json`.
8. **Due dates inline**: written `[due:YYYY-MM-DD]`. Setting one from the UI replaces an existing date or prepends a new one. The card badge reads "Due Mon D", coloured with `color-alt-3` for today and `color-alt-1` for past due.
9. **Images**: you can upload or paste into the editor. Files are stored as `CONFIG_DIR/images/<uuid>.<ext>` and served at `/_api/image/<name>`. A periodic **orphan-image cleanup** deletes images that no card references (`LOCAL_IMAGES_CLEANUP_INTERVAL` minutes, default 1440, 0 disables it). Upload can be disabled.
10. **Drag and drop**: cards can be dragged within a lane and across lanes. Manual order is stored per board path in `CONFIG_DIR/sort.json`, as an array of card names per lane.
11. **Sort modes**: Manually, Name A-Z/Z-A, Tags A-Z/Z-A (by first tag), Due ascending/descending, Last updated (file mtime), and Created first (file birthtime). The sort and its direction persist in `localStorage`.
12. **Search**: a case-insensitive substring match over card name **and** content.
13. **Filter by tag**: pick one tag. The choice persists in `localStorage`.
14. **View modes (density)**: Extended, Regular, Compact, Tight. The choice persists.
15. **Bulk operations**: a "Select cards" mode, then add tags, remove tags, set a due date, or delete across all selected cards. There is a tag search, "Create 'x'" for a new tag, and a delete confirmation.
16. **Keyboard navigation**:
    - Move between cards with arrows or vim `hjkl`.
    - `Alt+↑/↓` reorders a card within its lane, and `Alt+←/→` moves it to the adjacent lane.
    - `Enter`/`e` opens a card, `n` creates a new one, `r` renames, and `d` deletes after a confirmation.
    - `Esc` clears focus or closes the card, and `?` opens the shortcut help.
17. **Mobile**: long-press opens card and lane menus, with vibration feedback. The layout is responsive.
18. **Themes**:
    - Light and dark follow the OS.
    - Three bundled colour themes: Adwaita (the default), Nord, and Catppuccin.
    - Full CSS override through `CONFIG_DIR/stylesheets/custom.css`, using documented variables (`color-accent`, `color-foreground`, `color-background-1..4`, `color-alt-1..7`).
19. **i18n**: English and Spanish. The locale is auto-detected from the browser and persisted per user.
20. **PWA**: installable. This does not work when `BASE_PATH` is not `/`.
21. **Config via environment variables**:
    - `PUID`/`PGID` set file ownership.
    - `TITLE` sets the header and tab title.
    - `BASE_PATH` serves the app under a reverse-proxy subpath.
    - `LOCAL_IMAGES_CLEANUP_INTERVAL` sets the cleanup interval described above.
    - `TASKS_DIR` and `CONFIG_DIR` apply when running from source.
22. **REST API** (internal, under `/_api`):
    - `GET/POST/PATCH/DELETE /resource/<path>` for lanes and cards. A PATCH with a new name renames the file or folder.
    - `GET/PATCH /tags/<path>` for tag colours.
    - `GET/PUT /sort/<path>` for manual order.
    - `POST /image` for uploads, and `GET /title`.
23. **Interop**: because cards are plain files, the same folder works in Obsidian or any editor. The README links an issue showing this. There is **no file watching**, so external edits appear on the next load.
24. **No auth, no multi-user**: it is a single shared board. Only UI preferences are per browser.
25. **Scope policy**: the project is deliberately low-maintenance, and PRs that widen its scope are discouraged.

---

## 3. tasksmd/tasks.md

**What it is:** "a lightweight spec for AI agent task queues, the companion to AGENTS.md". It is a single `TASKS.md` file format (spec v1.0) plus reference tooling: a parser, a linter, a CLI (`tasks`), an MCP server (`tasks-mcp`), per-agent `/next-task` commands, and pluggable backends (file, git-native event log, GitHub Issues).

### 3a. File format (spec v1.0)

1. **File**: named `TASKS.md` at the repo root, UTF-8. It carries no version line.
2. **Multiple files**: monorepos may add `TASKS.md` in subdirectories. Discovery works like this:
   - Find the git root.
   - Find every `TASKS.md`, excluding `.git/` and `node_modules/`.
   - Sort the paths lexicographically.
   - Treat all the tasks together, ranked by P-level.
   - Split a file at around 50 tasks.
3. **Header**: the first line must be `# Tasks`.
4. **Priority sections**: `## P0` means drop everything, `## P1` is core work (the default for planned work), `## P2` is valuable but not blocking (the default for work an agent discovers), and `## P3` is someday. Sections go in ascending order, and empty ones can be omitted. Within a section, the first task is the most important. P4 and above are errors.
5. **Task**: `- [ ] Short imperative description`. It should be completable in one agent session.
6. **Completion is deletion**: when a task is done, its whole block (line, metadata, and subtasks) is removed, and history lives in `git log`. A top-level `[x]` is a lint error. Repos can optionally require a `closes <task-id>` line in the commit message.
7. **Metadata**: nested bullets with bold labels, as in `  - **Label**: value`. A value can span several indented lines. All metadata is optional, and custom fields are allowed. The defined fields:
   - `**ID**`: kebab-case, stable, unique across every file in the repo.
   - `**Tags**`: lowercase and comma-separated.
   - `**Details**`: implementation context.
   - `**Files**`: backtick-quoted paths, comma-separated.
   - `**Acceptance**`: the definition of done.
   - `**Plan**`: an agent-managed checklist written before coding.
   - `**Blocked by**`: comma-separated task IDs.
   - `**Blocked**`: a free-text external reason.
   - `**Parent**`: the ID of the task this was split from.
   - `**Research**`: notes an agent collects while the task is blocked.
   - `**Last-enriched**`: `YYYY-MM-DD`.
   - `**Estimate**`: a free-form duration such as `30m` or `2-3d`.
   - `**Verification**`: the runnable procedure for checking done.
   - `**Risk**`: `<risk>. Mitigation: <how>`.
   - The "rule-#9" pre-registration block: `**Hypothesis**`, `**Success**`, `**Pivot**`, `**Measurement**` (an exact runnable command), and `**Anchor**` (a citation).
   - `**Touches**`: the write-set of files, for orchestrators.
   - `**Surfaced-by**`: the task's provenance.
   - `**Milestone**`: a free-form identifier.
8. **Blockers**:
   - `**Blocked by**: a-id, b-id`. A blocker resolves when its ID no longer exists in any file, meaning the blocking task was completed and deleted.
   - A blocker can be scoped with `<repo>#<id>` for another repo or `<workspace>::<repo>#<id>` for another workspace.
   - Agents prioritize tasks that unblock others.
9. **Blocked for a reason**:
   - `**Blocked**: needs-user-approval — <text>`. Any non-empty value blocks picking.
   - Suggested reason codes: `needs-user-approval`, `needs-credentials`, `policy-refused`, `needs-external-action`.
   - `**Blocked**` and `**Blocked by**` can coexist, and both must clear before the task can be picked.
10. **Subtasks**:
    - Nested `- [ ]`/`- [x]` lines placed after the metadata. Checked subtasks stay in the file as progress.
    - The whole block is removed when the parent is done.
    - Subtasks inherit the parent's priority, and whoever claims the parent owns all of them.
    - Promote a subtask to top-level with `Blocked by` when it can run in parallel or ship on its own.
11. **Claiming**:
    - Append `(@agent-id)` to the task line, for example `- [ ] Add rate limiting (@cursor-1)`. The recommended identity form is `@<tool>-<instance>`.
    - In multi-agent setups, commit and push the claim immediately. This is best-effort, not a lock.
12. **Stale claims**: the same agent resumes its own claim automatically. A different agent checks `git log` for activity in the last 30 minutes and must **ask the user** before replacing someone else's `(@old)`.
13. **Policies**:
    - Written as HTML comments, `<!-- policy: <one directive> -->`. Several can share one comment, one per line.
    - File scope: between `# Tasks` and the first `## P*`. Section scope: right after a `## P*` heading.
    - They are invisible when the Markdown is rendered, and agents follow them as rules.
14. **Standing audit loop**:
    - A task with `**ID**: standing-audit-gap-loop` and `**Tags**: standing-loop, audit, queue`.
    - The agent audits only: it adds or refines follow-up tasks, deletes the loop task in the same commit, and stops.
    - Auto-pick skips it, so it runs only when targeted.
15. **Enrichment of blocked tasks**:
    - When every task is blocked, the agent spends its turn on read-only research.
    - It appends to `**Research**` under dated subheadings, may extend `Files`/`Acceptance`, and stamps `**Last-enriched**`. It never touches `Blocked`/`Blocked by`.
    - A task enriched in the last 7 days is skipped, and one task is enriched per turn.
16. **Disagreements**: an agent never silently reprioritizes or restructures a task. It flags the concern in `Details`, asks a human, or defers to the orchestrator.
17. **Writing tasks**: an agent appends new tasks to the end of the right section (P2 if unsure), adds an ID if the task may be referenced, and includes at least `Details`. If an orchestrator manages the file, it is the only writer of new tasks.
18. **AGENTS.md integration**: `init` adds a `## Task Management` section (read TASKS.md before asking the user, claim, remove on completion, prioritize unblockers, add discovered work). There is an optional `## Agents` section mapping agents to tags, for routing.
19. **Tag routing**: untagged tasks go to any agent. Tagged tasks go to any agent with at least one overlapping tag. Tags are a soft preference, and the most overlap wins.

### 3b. Agent workflow (`/next-task`)

20. **`/next-task` command**: generated per agent (Claude Code skill, Codex skill, Cursor, Devin, Gemini TOML, Windsurf workflow) from the canonical `commands/next-task.md`. There are also `/setup`, `/lint-tasks`, and `/migrate`.
21. **Entry modes**: queue pick (`/next-task`), targeted (`/next-task <id>`, which refuses a missing, duplicate, claimed, or blocked target and stops after one task), and the standing audit loop.
22. **Loop steps**:
    1. Stop check (a zero-ship streak or a fully blocked queue).
    2. Snapshot git and TASKS.md.
    3. Preserve dirty worktree edits.
    4. Tidy PRs and branches.
    5. Find every TASKS.md.
    6. Read the policies.
    7. Resume its own claim.
    8. Pick the highest priority that is unblocked and unclaimed, preferring impact on others and then harder tasks.
    9. **Refuse forbidden work**: public posts, issue comments, publishing, email, and protected pushes get a `**Blocked**` added instead. Opening a PR is allowed.
    10. Enrich if everything is blocked.
    11. **Plan and validate**: non-trivial tasks get a `docs/plans/<id>.md`, which a reviewer subagent must approve (up to 3 revisions, and a reject halts).
    12. Claim.
    13. Work.
    14. **Scout**: record any bugs or gaps found along the way as new tasks.
    15. Complete (delete the block, commit, push).
    16. Loop.
    17. **Roam**: scan `~/apps/*/TASKS.md` for other repos.
    18. **Audit cascade**: verify, then security/dead code, then doc drift, then dependency updates, then DX polish. Write the findings as tasks.
    19. Terminal summary.
23. **Human/agent contract**: "humans read the queue and tell agents what to do; agents (or tools) mutate task state." In the file backend, editing by hand is also valid.

### 3c. CLI (`@tasks-md/cli`, binary `tasks`)

24. `init [--install]`: scaffolds `TASKS.md` with P0-P3 headings and the AGENTS.md section. It is idempotent.
25. `install [--all|--agent <name>|--hooks]`: installs `/next-task` for the agents it detects (via `.claude/`, `.cursor/`, `.devin/`, and similar).
26. `pick [--tags a,b] [--json]`: a read-only pick. The JSON is `{picked, summary, priority, file, line, metadata, candidates, unblocks}`. `next` is an alias.
27. `list [--priority P1] [--tag t] [--unclaimed] [--unblocked] [--json]`: prints `<priority>\t<id>\t<summary>` per line.
28. `stats [--json]`: counts by priority, blocked, claimed, and available, plus throughput from `git log`.
29. `diff [<ref>] [--json]`: tasks added or removed since a git ref (default `HEAD`).
30. `watch [<dir>] [--fix]`: lints on save, and with `--fix` auto-removes `[x]` tasks.
31. **Backend-neutral operations**, which all take `--backend`, `--as <actor>` (default `$TASKS_ACTOR`), and `--json`, and return a typed `unsupported` result where a backend can't perform them:
    - `create <title> --priority --body --tag`
    - `update <id>` (unsupported on the file backend)
    - `claim <id>`
    - `unclaim <id>`
    - `complete <id>`
    - `cancel <id>` (drops the task, with the reason in the commit)
    - `render`
32. `sync github|jira|linear [--merge] [--output FILE]`: imports open issues as tasks.
    - Priority mapping: GitHub labels `critical|p0` become P0 and so on, with P2 as the default. Jira `Highest|Blocker|Critical` becomes P0. Linear priority `1` becomes P0, and `4` or `0` become P3.
    - IDs are prefixed `issue-`, `jira-PROJ-`, and `linear-TEAM-`.
    - `--merge` keeps hand-written tasks, adds new issues, and removes tasks whose issues were closed. Re-running it is idempotent.
33. `migrate [--apply]`: converts a file queue into the git-native log. It is a dry run by default and preserves IDs, priority, tags, and claims.
34. `fleet init|stats|compact`, `doctor`, and `check-push`: fleet setup, contention metrics, log compaction, an install health check, and a path-scoped push gate (a code push needs a live claim plus a matching `Task-Claim` trailer, while doc-only pushes pass).
35. `workspaces list|add|detect`: manages `~/.config/tasks-md/workspaces.json`.
    - An entry looks like `{ name, root, exclude?, priorityWeight? }`, with `discovery.scanRoots` and `autoDetect`.
    - `next --workspace/--workspaces/--workspace-name` aggregates across repos and prints `<workspace>::<repo>:<task-id>`.
    - A workspace is a directory holding a `.tasks-md-workspace` sentinel, or one with at least 2 child repos that each have a TASKS.md.
36. `generate-commands`: regenerates the per-agent command variants.
37. **Library API**: `loadAllTasks`, `pickBestTask`, `listTasks`, `getQueueStats`, `getQueueDiff`.

### 3d. MCP server (`tasks-mcp`)

38. Tools:
    - `list_tasks` (filters: priority, tag, unclaimedOnly, unblockedOnly).
    - `pick_task` (optional `task_id` and `agent_name`; returns the status `missing|duplicate|already_claimed|blocked|ready|resumed|claimed`).
    - `claim_task`, `unclaim_task`, `complete_task`: an exact ID match wins, otherwise they fall back to a summary substring.
    - `add_task` and `enrich_task`.
    - `find_next_task_across_workspaces`.
    - `TASKS_MCP_DIR` sets the root. On non-file backends, mutations delegate to the CLI.

### 3e. Linter (`@tasks-md/lint`) and CI

39. Rules checked:
    - The header is `# Tasks`, and P0-P3 appear in order with nothing above P3.
    - Tasks use `- [ ]` syntax and have no top-level `[x]`.
    - Tasks sit under a priority heading.
    - IDs are kebab-case and unique across files, and every blocker ID exists.
    - `Blocked` and `Research` are non-empty, and `Last-enriched` is an ISO date.
    - No metadata sits outside a task.
    - Policies are inside comments and non-empty, and comments are closed.
    - Opt-in: `--require-prereg` demands the rule-#9 fields, with `--prereg-allowlist` to grandfather existing tasks.
    - `--fix` removes `[x]` tasks and collapses blank lines. Exit code 1 means lint errors, 2 means a usage error.
40. **GitHub Action**: `uses: tasksmd/tasks.md/.github/actions/lint@main`.
41. **Conformance suite** (`@tasks-md/conformance`): a backend self-certifies against its capability classes (file, operation, or collision-free).

### 3f. Backends and sync

42. **Selection**: `.tasksmd.json` at the git root, `{ "backend": "tasks-md" | "git-native" | "github-issues", "repo": "owner/repo", "label": "tasks.md" }`. It defaults to `tasks-md`, and `--backend` overrides it per invocation.
43. **GitHub Issues backend**:
    - Open issues carrying the marker label are the queue. The ID is the issue number, and `priority/P0..P3` labels set priority (the looser `critical/high/medium/low` also work).
    - Other labels become tags, the assignee is the claim, and closing the issue completes the task.
    - It goes through the `gh` CLI with the user's existing auth.
44. **Git-native backend (fleet)**:
    - The source of truth is an append-only event log on the `tasks-claims` ref, stored as `events/<event_id>.json` with one event per commit.
    - The event schema is `schema_version, event_id, task_id, event_type, actor_id, instance_id, created_at, parent_event_ids, payload`.
    - Event types: `created, updated, claimed, heartbeat, released, completed, cancelled`.
    - `fold(log)` produces the state, and malformed events are skipped.
    - Claims are collision-free through git ref compare-and-swap. The loser of a race yields and picks again.
    - `claim_id` works as a fencing token, carried in `Task:` / `Task-Claim:` commit trailers.
    - Leases are renewed by heartbeats, and an expired lease can be stolen with a fresh `claim_id`.
    - Compaction shrinks the log.
    - `TASKS.md` becomes a generated snapshot written by a single CI job and is never edited by hand. Snapshots show actor handles, not emails.
45. **Workspace aggregation is backend-aware**: each repo contributes its own backend's open list, and a claim is written to that repo's backend with that backend's guarantees.

### 3g. Stated non-goals (VISION.md)

46. No GUI, no time tracking, no assignee model beyond the claim. It is not a scheduler or workflow engine, and blocked-by is a flat list of IDs, not a graph engine.

---

## 4. Reddit: "TASKS.md changed how I use Claude Code, less lies"

**Status: UNREACHABLE.** None of these returned the post:

- `www.reddit.com` (403 "Blocked").
- The `.json` endpoint, `old.reddit.com/...json` (served the "Welcome to Reddit" interstitial with a block), `api.reddit.com`, and `.rss` (all 403).
- The Chrome browser tool ("This site is not allowed due to safety restrictions").
- The Wayback Machine CDX (no captures; the availability API returned 429).
- Exa fetch (no content) and Exa search (not indexed).

**No workflow claims or comment ideas were extracted.** The title only suggests that a TASKS.md file reduced Claude Code falsely claiming work was done. That is an inference from the title, not content.

To fill this in:

- Paste the post text into the session, or
- Open it in a normal logged-in browser and save the page into `tmp/wayfinder/`.

---

## 5. tasks.md (website): TasksMD

**What it is:** "TasksMD", a private, local-first task editor that runs in the browser (Rust/Dioxus compiled to WASM). It opens Markdown files or folders on your device through the File System Access API and writes changes back only when you save. It is **not** related to tasksmd/tasks.md. There are no public docs or repo. Items marked *(bundle)* come from UI strings in the shipped WASM, so they are inferred.

### Features

1. **Local-first, no account**: nothing is uploaded ("Your file contents never leave your device"), and there is no signup.
2. **Open file / Open folder**: uses `showOpenFilePicker`, `showDirectoryPicker`, and `showSaveFilePicker`. An "Include nested folders next time" toggle makes folder opening recursive. Browsers without full File System Access get a warning banner and reduced features. *(bundle)*
3. **Recent workspaces**: remembered folders and files that can be reopened or forgotten, "Stored only in this browser" (IndexedDB). *(bundle)*
4. **Workspace sidebar**: a file list with a count. Files can be created in the folder, renamed, and deleted, but a dirty file must be saved or reloaded before it can be renamed, deleted, or switched away from. *(bundle)*
5. **Explicit save model**: nothing is written until Save (`Ctrl/Cmd+S`). The save-state pill shows Saved, Modified, Saving, Conflict, or Save failed. Unsaved changes prompt before closing. *(bundle)*
6. **External-change conflict detection**: "This file changed outside TasksMD. Review or reload it before overwriting." The conflict panel offers **Load disk version**, **Preview external version**, and **Overwrite anyway**. There is also a Reload button. *(bundle)*
7. **Undo/redo**: `Ctrl/Cmd+Z`, and `Ctrl+Y` or `Ctrl/Cmd+Shift+Z`. *(bundle)*
8. **Quick add**: an "Add a task" input with the placeholder hint `Review draft #work due:2026-09-01`. Inline tokens are parsed on entry. *(bundle)*
9. **Inline task syntax** *(bundle and landing page, partly inferred)*:
   - Checkbox tasks in Markdown.
   - `#tag` tags, and `@name` mentions or contexts (the landing shows `@marketing`).
   - `due:YYYY-MM-DD`.
   - `rec:<n><d|w|m|y>` recurrence, optionally anchored with `@due` (for example `1w@due`). The interval must be a positive whole number, and an `@due` anchor needs exactly one valid due field.
   - `_done:<timestamp>`, which is valid only on a completed task.
   - `_spent:<duration>` for tracked time (the landing shows `45m`).
   - `_timer:<start>` for a running timer.
10. **Structure rules** *(bundle)*: the first task in a block must be at indent level 0, indentation can increase by only one level at a time, and task text must be non-empty. Other Markdown blocks are preserved and rendered.
11. **Nested tasks**: `Tab`/`Shift+Tab` nests or unnests, and there are Indent and Outdent buttons. The visual indent follows the level. *(bundle)*
12. **Reordering**: `Alt+↑/↓` or the Move up/down buttons. *(bundle)*
13. **Complete a task**: `Ctrl/Cmd+Enter` or the checkbox. Completed tasks show struck through. Completing a recurring task presumably reschedules it (only the recurrence syntax was seen). *(bundle)*
14. **Task action panel**: a Due date picker (`<input type=date>`), a Repeat field, Start/Stop timer with a running-timer dot, Indent/Outdent, Move up/down, and Delete. *(bundle)*
15. **Time tracking**: a per-task start/stop timer that accumulates into `_spent`. It refuses to start a second timer or stop one that isn't running. *(bundle)*
16. **Inline Markdown block editing**: non-task blocks render as Markdown. Click to edit the source, and finish with `Esc` or `Ctrl+Enter`. *(bundle)*
17. **Filters**: All, Done, Due, and presumably Active. It shows "N tasks shown". *(bundle)*
18. **Metrics header**: Completed count, a Progress bar, and Diagnostics (parse warnings shown per document). *(bundle)*
19. **Keyboard shortcut help**: a popover listing all of the above. *(bundle)*
20. **PWA-ish**: a web manifest and icons. It is responsive and collapses the sidebar on narrow screens.

---

## Feature catalog (deduplicated)

Tags: GT, BM, TM, WEB (RD contributed nothing, because it was unreachable).

### File format

- Plain Markdown is the source of truth, readable without the app: GT, BM, TM, WEB
- One file holds many tasks: GT, TM, WEB
- One file per task (card = file): BM
- Directory = list or lane: BM (WEB can open folders of files but gives directories no meaning)
- Heading = list or group: GT (`## List`), TM (`## P0..P3` priority)
- Priority as a section heading (P0-P3): TM
- Checkbox status `[ ]`/`[x]`: GT (ordered list), TM (bullets), WEB
- Completed tasks deleted rather than checked, history in git: TM
- Completed tasks kept and checked: GT, WEB (with a `_done:` timestamp)
- Nested subtasks: GT (one level), TM (checkbox children, progress kept), WEB (multi-level, one step at a time)
- Free-form notes or body under a task: GT (blocks become notes), BM (the whole file), TM (`**Details**`), WEB (Markdown blocks)
- Bold-label metadata bullets `**Field**: value`: TM
- Inline token metadata: BM (`[tag:x]`, `[due:YYYY-MM-DD]`), WEB (`#tag`, `@ctx`, `due:`, `rec:`, `_spent:`, `_timer:`, `_done:`)
- Stable task IDs: TM (`**ID**: kebab-case`). The others use none, or a server id/filename (GT server ids, BM filename)
- Tags: BM, TM, WEB
- Due dates: BM, WEB. GT does not map them. TM has none, only `Milestone`/`Estimate`
- Recurrence: WEB
- Time tracking (timer, spent): WEB
- Estimates: TM (`Estimate`), WEB (duration shown)
- Dependencies (blocked-by task IDs, cross-file, cross-repo): TM
- Blocked with a free-text reason: TM
- Acceptance criteria and a verification procedure: TM
- Relevant files and write-set: TM (`Files`, `Touches`)
- Embedded images: BM
- Policies or rules in HTML comments: TM
- Custom or extra fields allowed: TM
- Format validation or diagnostics: TM (linter), WEB (in-app diagnostics), GT (strict parse errors)
- Multiple files discovered and merged: TM (repo discovery, workspaces), WEB (folder workspace)

### Agent workflow

- File designed as an agent work queue: TM
- Deterministic "pick next task" (priority, then unblocked, unclaimed, and impact): TM
- Claim or lease on a task: TM (`(@agent)`, or collision-free git CAS with leases, heartbeats, and fencing)
- Resume your own stale claim, and ask before stealing another's: TM
- Agent adds discovered work as new tasks (scouting): TM
- Agent refuses forbidden actions and records them as `Blocked`: TM
- Agent enriches blocked tasks with research, with a cooldown: TM
- Plan-then-review gate before coding: TM
- Standing audit loop that generates tasks: TM
- Per-agent installable command (`/next-task`) plus an MCP server: TM
- AGENTS.md integration and tag-based routing to specialist agents: TM
- Every mutation expressed as a small set of backend-neutral operations (create/update/list/claim/release/complete/cancel/render): TM
- Actor identity on each write: TM (claim suffix, `actor_id` in events)

### UI / board

- Browser UI: BM (self-hosted server), WEB (client-only)
- Kanban lanes with drag and drop: BM
- Single list with nesting: WEB
- Search over title and body: BM
- Filter by tag: BM. Filter by status or due: WEB
- Multiple sort modes (manual, name, tag, due, updated, created): BM
- Density or view modes: BM
- Bulk select (tag, due, delete): BM
- Keyboard-first navigation and shortcuts: BM (vim keys), WEB
- Undo/redo: WEB
- Quick-add with inline-token parsing: WEB
- Rich-text / Markdown editor toggle: BM. Inline Markdown block editing: WEB
- Due-date highlighting (today, overdue): BM
- Progress and completed metrics: WEB. Queue stats: TM CLI
- Themes, dark mode, custom CSS: BM
- i18n: BM
- PWA or installable: BM, WEB
- Deep link to a single task: BM
- Mobile long-press menus: BM

### Sync / storage / conflicts

- Two-way sync with a hosted service: GT (Google Tasks)
- One-way import from issue trackers with merge that preserves manual tasks: TM (GitHub, Jira, Linear)
- A hosted tracker as the backend: TM (GitHub Issues backend)
- Pluggable storage backends by config: TM
- Append-only event log with folding and compaction: TM
- Title-based diff and reconcile: GT
- Local backup ring plus rollback: GT
- Detect external edits, then offer load, preview, or overwrite: WEB
- Explicit save and dirty-state tracking: WEB
- Autosave on change: BM
- No conflict handling (last writer wins): GT, BM
- Everything stays on the device, no account: WEB. Self-hosted: BM
- Multi-account: GT (`--user`)
- OAuth auth flow: GT. `gh` auth reuse: TM

### CLI

- Print everything as Markdown: GT (`view`), TM (`render`)
- Edit everything in `$EDITOR`, then apply: GT (`edit`)
- Apply a Markdown file declaratively: GT (`reconcile`)
- Pick, list, and filter with `--json` output: TM
- Create, claim, complete, and cancel from the CLI: TM
- Queue stats and a diff since a git ref: TM
- Watch and auto-lint: TM
- Scaffold or init, and install into agent config: TM
- Health check (`doctor`): TM

### Misc

- Scheduled cleanup of orphaned images: BM
- Env-var configuration and a single Docker image: BM
- CI lint action and pre-push gate: TM
- Conformance test suite for backends: TM
- Explicit narrow-scope or non-goals policy: BM (low maintenance), TM (no GUI, no time tracking, no scheduler)
