# apps/web

## Purpose

The person's surface: a TypeScript browser client, and the only thing a person
looks at. It reads the tracker over the JSON `tasks api` serves and builds to
static files that same process serves beside the JSON, so there is no second
process and no CORS. `docs/adrs/0003-replace-the-tui-with-a-browser-client.md`
records why the surface moved off the terminal.

What it holds today: the board, six State lanes of leaf-Task cards over every
Repo, drawn from `GET /api/state` (#119), and a move, by dragging a card to a
lane at a desk or holding it on a phone for Move to (#121). The box, the add
sheet and the breakdown are kept unmounted for #123, which brings the Broker
routes back; the other writes come with #122.

## Ownership

- `src/api.ts` — the fetch plumbing every call shares: one JSON body out and
  one back, and the one place a failed response becomes an Error.
- `src/state.ts` — the wire shapes and the one read the board makes. It mirrors
  `apps/tasks/src/board/board.go`, which is the side that decides them. It
  holds the Narrowing and `queryString`, the one function that writes it as a
  query, plus the types the unmounted sheet still reads (`Offered`,
  `Collection`, `Level`) and `GET /api/files`'s shapes.
- `src/write.ts` — `moveTask`, the one write the board makes, and the calls
  the sheet, the box and the breakdown make. No route answers those until #122
  and #123; they stay so those components keep compiling and their tests keep
  their grammar.
- `src/hold.ts` — `useHold`, a long press on touch: the timer, the buzz, and
  the click after it stopped at the element.
- `src/MoveTo.tsx` — Move to, the bottom sheet of six States a held card opens.
- `src/read.ts` — the guard around a read a screen makes for itself, used by the
  sheet's file picker.
- `src/App.tsx` — the board: the poll, the six lanes, the cards, the moves,
  and the problems and flags a Repo has. It is what `main.tsx` mounts.
- `src/Box.tsx`, `src/Sheet.tsx`, `src/Breakdown.tsx` — the dump box, the add
  sheet and the breakdown, unmounted until #123.
- `src/main.tsx` — the mount, and nothing else.
- `src/App.test.tsx`, `src/Box.test.tsx`, `src/Sheet.test.tsx`,
  `src/Breakdown.test.tsx` — the components with a grammar.
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
  `state.STATES` order, and every Repo's leaves share them.
- **A Repo whose file has problems is flagged, line by line.** The problems
  `tasks lint` reports arrive on the read and are drawn above the lanes; config
  errors are drawn the same way.
- **A move is the board's one write.** At a desk (`pointer: fine`) a card is
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
- **A question is asked about the Tasks the read asked for.** `ask` and
  `fetchState` both build their query from a Narrowing through `queryString`,
  so a narrowing added there is on both.
- **Touch targets no smaller than 44px, one thumb, no hover — at every width.**
  The lanes scroll across and snap one at a time, so a phone shows one lane and
  most of the next.

## Work Guidance

- Formatting is Prettier's alone. `eslint-config-prettier` sits last in
  `eslint.config.js`, so `npm run lint` and `npm run format:check` cannot
  disagree about a line.
- The npm scripts here are the interface `scripts/libs/detect.sh` dispatches
  node on, through the root workspace manifest. A script renamed here goes
  unrun and reports `unavailable`, which fails the gate.
- No `public/`: `scripts/structure` reads a folder under a unit that is neither
  `src/` nor a scoped folder as a domain and requires a `src/` inside it.
- The screens #119 deleted (detail, Series, collections, activity, the log) are
  in git history for #120 and #122 to recover from.

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
