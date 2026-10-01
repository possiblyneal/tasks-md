# apps/web

## Purpose

The person's surface: a TypeScript browser client, and the only thing a person
looks at. It reads the tracker over the JSON `tasks api` serves and builds to
static files that same process serves beside the JSON, so there is no second
process and no CORS. `docs/adrs/0003-replace-the-tui-with-a-browser-client.md`
records why the surface moved off the terminal.

What it holds today: the board, six State lanes of leaf-Task cards over every
Repo, drawn from `GET /api/state`, with Repo and Tag chips, a search box, a
sort picker, the weekly metrics, a panel a card opens into, which shows a Series' next dates and the Task's history, a move, by dragging a card to a lane at
a desk or holding it on a phone for Move to, the add/edit form with
delete, the box pinned under the board that hands a dump or a question to
the Broker, and Break down in the panel. It installs to an
iPhone's home screen as a PWA, and offline shows the last board read.

## Ownership

- `src/api.ts` — the fetch plumbing every call shares: one JSON body out and
  one back, and the one place a failed response becomes an Error.
- `src/state.ts` — the wire shapes and the one read the board makes. It mirrors
  `apps/tasks/src/board/board.go`, which is the side that decides them. It
  holds the Narrowing and `queryString`, the one function that writes it as a
  query, the values the form offers (`LEVELS`, `ESTIMATES`, `COLORS`,
  mirroring `write.go`), `fetchSeries`, `fetchHistory` and `GET /api/files`'s shapes.
- `src/write.ts` — the `Draft` the form holds, `draftOf`, and the board's
  writes: `moveTask`, `addTask`, `editTask`, `deleteTask`. `capture` and
  `ask` are the box's; `breakdown` and `addSubtasks` (one write of every
  approved proposal) are the breakdown's.
- `src/hold.ts` — `useHold`, a long press on touch: the timer, the buzz, and
  the click after it stopped at the element.
- `src/MoveTo.tsx` — Move to, the bottom sheet of six States a held card opens.
- `src/read.ts` — the guard around a read a screen makes for itself, used by the
  form's file picker and the panel's Series and history.
- `src/Series.tsx` — a `series:` attribute in the panel: the rule, and the next
  dates `GET /api/tasks/{id}/series` works out.
- `src/History.tsx` — the panel's history: what `GET /api/tasks/{id}/history`
  answers, newest first.
- `src/App.tsx` — the board: the six lanes, the cards, the moves, the
  problems and flags a Repo has, and which panel is open. It is what
  `main.tsx` mounts.
- `src/poll.ts` — `usePoll`, the once-a-second read of `GET /api/state` under
  one Narrowing.
- `src/Narrow.tsx` — the Repo chips, Tag chips, search box and sort picker.
- `src/Panel.tsx` — the panel: every attribute, the parent with its leaf
  counts per State, the Subtasks, its history, and Edit, Delete and Break down.
- `src/Sheet.tsx` — the add/edit form, and the file picker for attachments.
- `src/Metrics.tsx` — the weekly metrics the read carries: this week in the
  summary line, the twelve newest first under it.
- `src/Due.tsx` — a Deadline marked today or Overdue against the served
  `today`.
- `src/Box.tsx` — the box: one field with a ＋/? toggle.
- `src/Breakdown.tsx` — the breakdown: the Broker's questions, then
  proposals to tick.
- `src/main.tsx` — the mount, and the service worker's registration in a
  build.
- `src/offline.ts` — what the service worker answers each request with, and
  `keep`, which holds the page and the files it loads. `OFFLINE` is the header
  a board read answered from the cache carries, which `fetchState` reads.
- `src/sw.ts` — the service worker's wiring, built to `/sw.js`.
- `src/public/` — Vite's `publicDir`, copied to the build's root unhashed:
  `manifest.json` and the icons. `icon.svg` is the source; the PNGs are
  `rsvg-convert -w <n> -h <n> icon.svg -o icon-<n>.png` for 180 (the
  `apple-touch-icon` Safari's home screen reads), 192 and 512.
- `src/App.test.tsx`, `src/Narrow.test.tsx`, `src/Panel.test.tsx`,
  `src/Series.test.tsx`, `src/Metrics.test.tsx`, `src/History.test.tsx`,
  `src/Box.test.tsx`, `src/Sheet.test.tsx`, `src/Breakdown.test.tsx` — the
  components with a grammar. `src/testing.ts` is the board fixture and the
  mocked fetch the board's tests share.
- `src/state.test.ts`, `src/write.test.ts` — what a Narrowing becomes as a
  query, what a read does with a `304` and a refusal, and what a write sends.
- `src/offline.test.ts` — what the service worker answers, with a Map for the
  Cache and a fetch that reaches the server or does not.
- `src/index.css` — the whole of the styling. There is no component-level
  stylesheet and no CSS-in-JS, so the 44px rule below is checkable by reading
  one file.
- `index.html`, `vite.config.ts`, `tsconfig.json`, `eslint.config.js` — the
  build. The dev server proxies `/api` so development has the one origin
  production has.
- `.unit.json` — `ships: none`, because what this unit produces is files
  somebody places rather than a program somebody starts. `npm run build` writes
  them to `dist/`, and `apps/tasks/deploy/systemd/tasks-api.service` points
  `tasks api -web` at where they are copied.

## Local Contracts

- **Vite, React, and nothing else.** No router, no data-fetching library, no
  component kit. A dependency added here is a decision, not a convenience.
- **The client works nothing out that the read already did.** A Task's State
  (a parent's included), whether it is Blocked, and its parents' titles all
  arrive with it; the board draws them.
- **A card is a leaf Task, and a parent is never a card of its own.** Its title
  rides above each leaf under it, outermost first, which is how a nested Task
  sits on a flat lane. A card names its Repo, and its edge is the Repo's color.
- **One board over every Repo.** The lanes are the six States in
  `state.STATES` order, and every Repo's leaves share them, ordered by the
  read's `rank`. Each lane header counts its cards.
- **Narrowing is the server's.** Every chip, the search box and the sort
  picker set a field of the Narrowing, and the lanes are the read made under
  it, so they match `tasks list` under the same flags. Repo chips are one at a
  time, because a read names one Repo or all; Tag chips are any-of. The sorts
  offered are the read's `sorts`; `file` is the bare path.
- **Two reads.** The wide read draws the chips, the flags and the panel, which
  need every Repo, Tag and parent whatever is narrowed; the narrowed read draws
  the lanes and is not made while nothing is narrowed. A narrowed board is
  drawn only under the narrowing it answered.
- **The metrics are the read's `metrics`, drawn as they come.** Twelve weeks
  of Added, Done and Declined, counted by the server under the read's Repo and
  Tags alone, so the board draws the narrowed read's while anything is
  narrowed and they follow the chips without the client counting anything.
  This week is in sight; the twelve open under it in a `<details>`.
- **Today is the host's.** The read's `today` is what a Deadline is due today
  or Overdue against, never the browser's clock.
- **A card opens a panel.** A bottom sheet 88% of a phone's height,
  a side panel at a desk. It shows every attribute; a Subtask's opens on its
  parent, with the parent's State and its leaves per State
  (`Doing 1 · Backlog 2 · Done 3`), and the parent opens its own panel. The
  tree is read from the wide read in file order. A card without an id is not
  opened. Edit opens the form on the Task; Delete asks with a second tap
  rather than a browser dialog, deletes the Task and its Subtasks, and closes
  the panel. Each Task opens on a fresh panel, so nothing armed or read for
  one is drawn on the next.
- **One form adds and edits.** Add on the board opens it on a blank Task in
  the first Repo; Edit opens it on the Task, in the same scrim and panel.
  It carries Title, Description, Why, Acceptance, Tags, Deadline, Estimate
  S/M/L, Priority, Impact, Color, Blocked by and Attachments; Reason only
  while the State is Deferred or Declined, Until only while Deferred. An add
  picks the Repo and State; an edit shows both and changes neither, since a
  move holds State's rules. A value the form does not offer is kept as one
  more option. Nothing is written before submit, and a refusal is drawn in
  the form with every value still in its field.
- **An edit sends what changed and the version it read.** `editTask` diffs
  the Draft the form opened on against the one submitted, so an attribute the
  form hides or nobody touched is not rewritten; attachments go as `attach`
  and `detach`. A delete sends the version too, and a tree changed since is a
  `409` drawn in the API's words.
  The panel's foot is the Task's history, newest first, each write with its
  moment and Actor and `Actor unknown` for a direct edit; which commits are
  the Task's is the server's to decide. It is read again when the Task's
  `version` changes, and offline it fails as any other read does.
  A `series:` attribute shows its rule and the next dates the server works
  out; the client does no rule arithmetic.
- **The address names the open panel.** `?task=<id>&repo=<name>` opens one on
  load, since an id is unique only within its Repo; with no `repo` the first
  Repo holding the id is used. Opening and closing replace the address.
- **Colors are tokens on `:root`,** redefined under
  `prefers-color-scheme: dark`, so light and dark follow the OS.
- **A Repo whose file has problems is flagged, line by line.** The problems
  `tasks lint` reports arrive on the read and are drawn above the lanes, and
  the Repo's chip is marked; config errors are drawn the same way.
- **A move is a drag or a hold.** At a desk (`pointer: fine`) a card is
  draggable and a lane is a drop target. On touch a hold of `HOLD_MS` (450ms)
  buzzes where `navigator.vibrate` exists, suppresses the browser's menu and
  opens Move to, listing the six States with the card's own disabled; the
  click the lifted finger sends is stopped at the card, so the hold opens
  nothing else. A move does not redraw the card itself: the next poll does.
  A refusal is drawn over the board in the API's words. Move to offers no
  Reason; `tasks move -reason` does.
- **A Repo's flags are drawn beside its problems.** `not pushed`, `not backed
up` or why it refuses writes, as the read carries them in `flags`.
- **A 304 is nothing to redraw, not an empty state.** `fetchState` returns
  `null` for one and the board keeps what it has; the ETag is handed back on
  the next poll, once a second.
- **One poll at a time.** The next is scheduled once the one before it has
  settled rather than on an interval, so the slower of an overlapping pair
  cannot draw an older board over a newer one.
- **A failed poll takes nothing off the screen.** The error is drawn over the
  board it interrupted, and the next poll asks without a tag.
- **An API error is shown in the API's own words.** The body's sentence is the
  one the CLI would have printed, so it is drawn rather than restated.
- **The box is pinned under the board and writes nothing.** ＋ reads a dump
  through `POST /api/capture` and opens the add form on it in the Repo the
  Broker guessed, or the first Repo when it guessed none offered; only the
  form's Add writes, and that empties the box, while Cancel keeps the words.
  ? asks and draws the answer above the field.
- **Break down replaces the panel until Back.** Every proposal starts ticked;
  approving is one `addSubtasks` under the version the breakdown opened on,
  so a tree changed since is a `409` and nothing is half written.
- **A question is asked about the Tasks the read asked for.** `ask` and
  `fetchState` both build their query from a Narrowing through `queryString`,
  so a narrowing added there is on both.
- **Offline, the board is the last one read, and nothing on it writes.** The
  service worker answers navigations from the network and falls back to the
  kept page; the files under `/assets/` from the cache first (Vite hashes their
  names); and `GET /api/state` from the network, keeping each `200` under its
  URL and falling back to it marked `X-Tasks-Offline`. A read whose tag is not
  the kept board's is asked without one, or the reads after the worker takes
  the page would all be `304` and nothing would be kept. Writes are never
  intercepted. The page is offline while the last poll was answered from the
  cache or never reached a server (a `TypeError`); it says so in a
  `role="status"` line and disables every write control: no drag, no hold,
  no Move to, no Add, no Edit, Delete or Break down in the panel, no
  submitting a form already open, and no dump or question in the box; a
  breakdown already open answers and approves nothing. A write control added later reads the same `offline`. The next
  poll that
  reaches the server ends it, with no reload.
- **The worker is a module, and every chunk the page loads is in
  `index.html`.** `sw.js` imports the chunk it shares with the page, which
  Safari allows from iOS 15. `keep` finds what to hold by reading `/assets/`
  paths out of the page, which works because Vite lists each static chunk as
  a `modulepreload`; a lazy `import()` would be missed and must not be added
  without changing that.
- **Touch targets no smaller than 44px, one thumb, no hover — at every width.**
  On a phone each lane is most of a screen wide and they scroll across,
  snapping one at a time with the next one's edge in view; from 64rem the six
  sit across as columns.

## Work Guidance

- Formatting is Prettier's alone. `eslint-config-prettier` sits last in
  `eslint.config.js`, so `npm run lint` and `npm run format:check` cannot
  disagree about a line.
- The npm scripts here are the interface `scripts/libs/detect.sh` dispatches
  node on, through the root workspace manifest. A script renamed here goes
  unrun and reports `unavailable`, which fails the gate.
- No `apps/web/public/`: `scripts/structure` reads a folder under a unit that
  is neither `src/` nor a scoped folder as a domain and requires a `src/`
  inside it. That is why `publicDir` is `src/public`.
- The screens #119 deleted (collections, activity, the log) are in git
  history.

## Verification

`npm run lint`, `npm run typecheck`, `npm run test` and `npm run build` from
the repository root, or `scripts/check` for the gate CI runs.

`environment: 'node'` is the default, and the modules that talk to the API are
tested under it. A component test opts into a DOM with a
`// @vitest-environment jsdom` docblock at the top of its file.

`jsdom` and `@testing-library/react` are the only two devDependencies here that
exist for the tests. `@testing-library/user-event` and `jest-dom` are
deliberately absent: `fireEvent` and plain `expect` cover what these tests
assert.

## Child Index

None.
