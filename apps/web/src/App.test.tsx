// @vitest-environment jsdom

// The board: six State lanes of leaf-Task cards over every Repo, drawn from
// what `GET /api/state` answered and nothing else. A parent is not a card of
// its own; its title rides on each leaf under it. The one write on the board
// is a move: a card dragged to a lane at a desk, or held on a phone and sent
// through Move to.

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
import type { Board, Task } from './state'

function task(fields: Partial<Task> & Pick<Task, 'title' | 'state'>): Task {
  return {
    id: '',
    tags: [],
    created: '2026-09-20',
    attrs: [],
    description: '',
    line: 1,
    depth: 0,
    parent: '',
    parents: [],
    leaf: true,
    blocked: false,
    ...fields,
  }
}

const BOARD: Board = {
  repos: [
    {
      name: 'house-move',
      path: '/home/n/code/house-move',
      color: 'green',
      problems: [],
      flags: [],
      tasks: [
        task({
          id: 'm3qa',
          title: 'Pack the kitchen',
          state: 'doing',
          leaf: false,
          line: 4,
        }),
        task({
          id: 'm3qc',
          title: 'Wrap glassware',
          state: 'doing',
          tags: ['fragile'],
          attrs: [{ label: 'deadline', value: '2026-10-10' }],
          depth: 1,
          parent: 'm3qa',
          parents: ['Pack the kitchen'],
          line: 8,
        }),
        task({
          id: 'm3qb',
          title: 'Buy boxes',
          state: 'done',
          depth: 1,
          parent: 'm3qa',
          parents: ['Pack the kitchen'],
          line: 11,
        }),
        task({
          id: 'v9t1',
          title: 'Book the van',
          state: 'backlog',
          blocked: true,
          line: 14,
        }),
      ],
    },
    {
      name: 'work',
      path: '/home/n/code/work',
      color: '',
      problems: [{ line: 3, message: '"Write the report" has no id line' }],
      flags: ['not backed up'],
      tasks: [task({ title: 'Write the report', state: 'inbox', line: 3 })],
    },
  ],
  errors: [],
}

// Every request the board made, so what it asked for and how is checkable.
let asked: { url: string; init?: RequestInit }[] = []

function answering(...bodies: (Board | { status: number; error: string })[]) {
  return (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    const body = bodies[Math.min(asked.length, bodies.length) - 1]
    if (body && 'status' in body) {
      return Promise.resolve(
        new Response(JSON.stringify({ error: body.error }), {
          status: body.status,
        }),
      )
    }
    return Promise.resolve(
      new Response(JSON.stringify(body), { headers: { ETag: '"1"' } }),
    )
  }
}

beforeEach(() => {
  asked = []
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

  expect(screen.queryAllByRole('button')).toEqual([])
  expect(screen.queryAllByRole('textbox')).toEqual([])
  expect(screen.queryAllByRole('checkbox')).toEqual([])
  for (const { url, init } of asked) {
    expect(url).toBe('/api/state')
    expect(init?.method ?? 'GET').toBe('GET')
  }
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
