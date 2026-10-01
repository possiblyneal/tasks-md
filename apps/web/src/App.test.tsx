// @vitest-environment jsdom

// The board: six State lanes of leaf-Task cards over every Repo, drawn from
// what `GET /api/state` answered and nothing else. A parent is not a card of
// its own; its title rides on each leaf under it. Nothing on the board writes.

import {
  cleanup,
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

test('nothing on the board writes', async () => {
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
