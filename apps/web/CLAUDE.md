# apps/web

## Purpose

The person's surface: a TypeScript browser client, and the only thing a person
looks at. It reads the tracker over the JSON `tasks api` serves and builds to
static files that same process serves beside the JSON, so there is no second
process and no CORS. `docs/adrs/0003-replace-the-tui-with-a-browser-client.md`
records why the surface moved off the terminal.

What it holds today (#119, #120): the board, six State lanes of leaf-Task
cards over every Repo, drawn from `GET /api/state` and read only, with Repo and
Tag chips, a search box, a sort picker, and a read-only panel a card opens
into. The box, the add sheet and the breakdown are kept unmounted for #123,
which brings the Broker routes back; writes come with #121 and #122.

## Ownership

- `src/api.ts` — the fetch plumbing every call shares: one JSON body out and
  one back, and the one place a failed response becomes an Error.
- `src/state.ts` — the wire shapes and the one read the board makes. It mirrors
  `apps/tasks/src/board/board.go`, which is the side that decides them. It
  holds the Narrowing and `queryString`, the one function that writes it as a
  query, plus the types the unmounted sheet still reads (`Offered`,
  `Collection`, `Level`) and `GET /api/files`'s shapes.
- `src/write.ts` — the calls the sheet, the box and the breakdown make. No
  route answers them until #121, #122 and #123; they stay so those components
  keep compiling and their tests keep their grammar.
- `src/read.ts` — the guard around a read a screen makes for itself, used by the
  sheet's file picker.
- `src/App.tsx` — the board: the six lanes, the cards, the problems a Repo's
  file has, and which panel is open. It is what `main.tsx` mounts.
- `src/poll.ts` — `usePoll`, the once-a-second read of `GET /api/state` under
  one Narrowing.
- `src/Narrow.tsx` — the Repo chips, Tag chips, search box and sort picker.
- `src/Panel.tsx` — the read-only panel: every attribute, the parent with its
  leaf counts per State, and the Subtasks.
- `src/Due.tsx` — a Deadline marked today or Overdue against the served
  `today`.
- `src/Box.tsx`, `src/Sheet.tsx`, `src/Breakdown.tsx` — the dump box, the add
  sheet and the breakdown, unmounted until #123.
- `src/main.tsx` — the mount, and nothing else.
- `src/App.test.tsx`, `src/Narrow.test.tsx`, `src/Panel.test.tsx`,
  `src/Box.test.tsx`, `src/Sheet.test.tsx`, `src/Breakdown.test.tsx` — the
  components with a grammar. `src/testing.ts` is the board fixture and the
  mocked fetch the board's tests share.
- `src/state.test.ts`, `src/write.test.ts` — what a Narrowing becomes as a
  query, what a read does with a `304` and a refusal, and what a write sends.
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
- **Today is the host's.** The read's `today` is what a Deadline is due today
  or Overdue against, never the browser's clock.
- **A card opens a read-only panel.** A bottom sheet 88% of a phone's height,
  a side panel at a desk. It shows every attribute; a Subtask's opens on its
  parent, with the parent's State and its leaves per State
  (`Doing 1 · Backlog 2 · Done 3`), and the parent opens its own panel. The
  tree is read from the wide read in file order. A card without an id is not
  opened.
- **The address names the open panel.** `?task=<id>&repo=<name>` opens one on
  load, since an id is unique only within its Repo; with no `repo` the first
  Repo holding the id is used. Opening and closing replace the address.
- **Colors are tokens on `:root`,** redefined under
  `prefers-color-scheme: dark`, so light and dark follow the OS.
- **A Repo whose file has problems is flagged, line by line.** The problems
  `tasks lint` reports arrive on the read and are drawn above the lanes, and
  the Repo's chip is marked; config errors are drawn the same way.
- **Nothing on the board writes.** Its buttons and fields narrow and open; the
  only request it makes is `GET /api/state`.
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
- **A question is asked about the Tasks the read asked for.** `ask` and
  `fetchState` both build their query from a Narrowing through `queryString`,
  so a narrowing added there is on both.
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
- No `public/`: `scripts/structure` reads a folder under a unit that is neither
  `src/` nor a scoped folder as a domain and requires a `src/` inside it.
- The screens #119 deleted (Series, collections, activity, the log) are in
  git history for #122 to recover from.

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
