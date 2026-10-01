// @vitest-environment jsdom

// The board: six State lanes of leaf-Task cards over every Repo, drawn from
// what `GET /api/state` answered and nothing else. A parent is not a card of
// its own; its title rides on each leaf under it. Tapping a card opens it to
// read. The one write on the board is a move: a card dragged to a lane at a
// desk, or held on a phone and sent through Move to.

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { App } from './App'
import { answering, asked, BOARD, task } from './testing'

beforeEach(() => {
  asked.length = 0
  vi.stubGlobal('fetch', answering(BOARD))
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const lane = (name: string) => screen.getByRole('region', { name })

test('the board is six lanes, one per State, in order', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  expect(
    screen.getAllByRole('region').map((r) => r.getAttribute('aria-label')),
  ).toEqual(['Inbox', 'Backlog', 'Doing', 'Deferred', 'Done', 'Declined'])
})

test('a card is a leaf Task in its lane, carrying its parents titles', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const doing = within(lane('Doing'))
  expect(doing.getByText('Wrap glassware')).toBeDefined()
  expect(doing.getByText('Pack the kitchen ›')).toBeDefined()
  // The parent is virtual: it is on its leaves, never a card of its own.
  expect(screen.queryByText('Pack the kitchen')).toBeNull()
  expect(within(lane('Done')).getByText('Buy boxes')).toBeDefined()
  expect(within(lane('Deferred')).queryAllByRole('article')).toEqual([])
})

test('every Repo is on the one board, and each card says which', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const report = within(lane('Inbox')).getByRole('article')
  expect(within(report).getByText('Write the report')).toBeDefined()
  expect(within(report).getByText('work')).toBeDefined()

  const van = within(lane('Backlog')).getByRole('article')
  expect(within(van).getByText('house-move')).toBeDefined()
  expect(within(van).getByText('blocked')).toBeDefined()

  const glass = within(lane('Doing')).getByRole('article')
  expect(within(glass).getByText('#fragile')).toBeDefined()
  expect(within(glass).getByText('due 2026-10-10')).toBeDefined()
})

test('a Repo whose file has problems is flagged, line by line', async () => {
  render(<App />)

  const flagged = await screen.findByRole('complementary', { name: 'Problems' })
  expect(
    within(flagged).getByText('work line 3: "Write the report" has no id line'),
  ).toBeDefined()
  expect(within(flagged).queryByText(/house-move/)).toBeNull()
})

test('until a card is moved, the board only reads', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  expect(screen.queryAllByRole('checkbox')).toEqual([])
  for (const { url, init } of asked) {
    expect(url).toMatch(/^\/api\/state(\?|$)/)
    expect(init?.method ?? 'GET').toBe('GET')
  }
})

test('each lane says how many cards it holds', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  expect(within(lane('Doing')).getByRole('heading').textContent).toBe('Doing 1')
  expect(within(lane('Deferred')).getByRole('heading').textContent).toBe(
    'Deferred 0',
  )
})

// Rank is the read's order across every Repo, so a lane holding two Repos'
// cards draws them in the order `tasks list -sort` would print them.
test('a lane draws its cards in the order the read ranked them', async () => {
  const [house, work] = BOARD.repos
  if (!house || !work) throw new Error('BOARD has two Repos')
  vi.stubGlobal(
    'fetch',
    answering({
      ...BOARD,
      repos: [
        {
          ...house,
          tasks: [task({ title: 'Label boxes', state: 'inbox', rank: 1 })],
        },
        {
          ...work,
          tasks: [task({ title: 'File expenses', state: 'inbox', rank: 0 })],
        },
      ],
    }),
  )
  render(<App />)
  await screen.findByText('Label boxes')

  expect(
    within(lane('Inbox'))
      .getAllByRole('article')
      .map((card) => card.querySelector('.card-title')?.textContent),
  ).toEqual(['File expenses', 'Label boxes'])
})

// Today is the host's, served with the read, so a phone in another time zone
// marks the same Deadlines the host would.
test('a Deadline today or past is marked against the served today', async () => {
  const [house, work] = BOARD.repos
  if (!house || !work) throw new Error('BOARD has two Repos')
  const due = (value: string) => [{ label: 'deadline', value }]
  vi.stubGlobal(
    'fetch',
    answering({
      ...BOARD,
      today: '2026-10-01',
      repos: [
        {
          ...house,
          tasks: [
            task({
              title: 'Return keys',
              state: 'doing',
              attrs: due('2026-10-01'),
            }),
            task({
              title: 'Cancel internet',
              state: 'doing',
              attrs: due('2026-09-28'),
            }),
            task({
              title: 'Sell sofa',
              state: 'doing',
              attrs: due('2026-10-02'),
            }),
          ],
        },
        work,
      ],
    }),
  )
  render(<App />)
  await screen.findByText('Return keys')

  const card = (title: string) =>
    within(screen.getByText(title).closest('article') as HTMLElement)
  expect(card('Return keys').getByText('due today').className).toBe('today')
  expect(
    card('Cancel internet').getByText('overdue 2026-09-28').className,
  ).toBe('overdue')
  expect(card('Sell sofa').getByText('due 2026-10-02').className).toBe('')
})

test('a Repo whose tasks history is flagged says so', async () => {
  render(<App />)

  const flagged = await screen.findByRole('complementary', { name: 'Flags' })
  expect(within(flagged).getByText('work: not backed up')).toBeDefined()
  expect(within(flagged).queryByText(/house-move/)).toBeNull()
})

// The board's reads answer with BOARD, and every write with what is given.
function writesAnswering(answer: { status: number; body: unknown }) {
  vi.stubGlobal('fetch', (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    if ((init?.method ?? 'GET') === 'GET') {
      return Promise.resolve(
        new Response(JSON.stringify(BOARD), { headers: { ETag: '"1"' } }),
      )
    }
    return Promise.resolve(
      new Response(JSON.stringify(answer.body), { status: answer.status }),
    )
  })
}

const writes = () =>
  asked
    .filter(({ init }) => init?.method === 'POST')
    .map(({ url, init }) => ({ url, body: JSON.parse(String(init?.body)) }))

const HOLD_MS = 450

async function hold(card: HTMLElement) {
  fireEvent.pointerDown(card, {
    pointerType: 'touch',
    clientX: 10,
    clientY: 10,
  })
  await new Promise((resolve) => setTimeout(resolve, HOLD_MS + 50))
  fireEvent.pointerUp(card, { pointerType: 'touch' })
}

test('holding a card on a phone opens Move to, with its own State greyed', async () => {
  writesAnswering({ status: 200, body: { id: 'm3qc' } })
  const vibrate = vi.fn()
  Object.defineProperty(navigator, 'vibrate', {
    value: vibrate,
    configurable: true,
  })
  render(<App />)
  await screen.findByText('Wrap glassware')
  const card = within(lane('Doing')).getByRole('article')

  // A long press is not also a tap: the click a phone sends when the finger
  // lifts goes no further than the card.
  const tapped = vi.fn()
  document.addEventListener('click', tapped)
  await hold(card)
  fireEvent.click(card)
  document.removeEventListener('click', tapped)
  expect(tapped).not.toHaveBeenCalled()
  expect(vibrate).toHaveBeenCalled()

  const sheet = screen.getByRole('dialog', { name: 'Move to' })
  const states = within(sheet).getAllByRole('button', {
    name: /^(Inbox|Backlog|Doing|Deferred|Done|Declined)$/,
  })
  expect(states.map((b) => b.textContent)).toEqual([
    'Inbox',
    'Backlog',
    'Doing',
    'Deferred',
    'Done',
    'Declined',
  ])
  expect(
    states
      .filter((b) => (b as HTMLButtonElement).disabled)
      .map((b) => b.textContent),
  ).toEqual(['Doing'])

  fireEvent.click(within(sheet).getByRole('button', { name: 'Done' }))
  await waitFor(() =>
    expect(writes()).toEqual([
      {
        url: '/api/tasks/m3qc/move',
        body: { repo: 'house-move', state: 'done' },
      },
    ]),
  )
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
})

test('a press let go early is a tap, and opens no sheet', async () => {
  render(<App />)
  const card = await screen.findByText('Wrap glassware')

  fireEvent.pointerDown(card, { pointerType: 'touch' })
  fireEvent.pointerUp(card, { pointerType: 'touch' })
  await new Promise((resolve) => setTimeout(resolve, HOLD_MS + 50))
  expect(screen.queryByRole('dialog')).toBeNull()
})

test('the browser menu a long press would open is not opened', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')
  const card = within(lane('Doing')).getByRole('article')

  expect(fireEvent.contextMenu(card)).toBe(false)
})

test('a card dragged to another lane is moved there', async () => {
  writesAnswering({ status: 200, body: { id: 'v9t1' } })
  render(<App />)
  await screen.findByText('Book the van')
  const van = within(lane('Backlog')).getByRole('article')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }

  expect(van.getAttribute('draggable')).toBe('true')
  fireEvent.dragStart(van, { dataTransfer })
  fireEvent.dragOver(lane('Doing'), { dataTransfer })
  fireEvent.drop(lane('Doing'), { dataTransfer })

  await waitFor(() =>
    expect(writes()).toEqual([
      {
        url: '/api/tasks/v9t1/move',
        body: { repo: 'house-move', state: 'doing' },
      },
    ]),
  )
})

test('a move the API refuses is said in its words', async () => {
  writesAnswering({
    status: 409,
    body: { error: '^v9t1 is already in Doing, so somebody has taken it' },
  })
  render(<App />)
  await screen.findByText('Book the van')
  const van = within(lane('Backlog')).getByRole('article')
  const dataTransfer = { setData: vi.fn(), effectAllowed: '', dropEffect: '' }

  fireEvent.dragStart(van, { dataTransfer })
  fireEvent.drop(lane('Doing'), { dataTransfer })

  expect(
    await screen.findByText(
      '^v9t1 is already in Doing, so somebody has taken it',
    ),
  ).toBeDefined()
})

test('a config error is said over the board', async () => {
  vi.stubGlobal(
    'fetch',
    answering({
      ...BOARD,
      errors: ['two Repos are named work: /a/work and /b/work'],
    }),
  )
  render(<App />)

  expect(
    await screen.findByText('two Repos are named work: /a/work and /b/work'),
  ).toBeDefined()
})

// A read that failed says nothing about the Tasks already drawn, so they stay
// and the sentence goes over them. The poll hands the ETag back, so the second
// request is conditional.
test('a failed poll takes nothing off the board', async () => {
  vi.stubGlobal(
    'fetch',
    answering(BOARD, { status: 500, error: 'the disk went away' }),
  )
  render(<App />)
  await screen.findByText('Wrap glassware')

  expect(
    await screen.findByText('the disk went away', {}, { timeout: 3000 }),
  ).toBeDefined()
  expect(screen.getByText('Wrap glassware')).toBeDefined()
  await waitFor(() =>
    expect(asked[1]?.init?.headers).toEqual({ 'If-None-Match': '"1"' }),
  )
})
