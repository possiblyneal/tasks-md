// @vitest-environment jsdom

// The board: six State lanes of leaf-Task cards over every Repo, drawn from
// what `GET /api/state` answered and nothing else. A parent is not a card of
// its own; its title rides on each leaf under it. Nothing on the board writes:
// tapping a card opens it to read, and every request is a GET of the read.

import {
  cleanup,
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

test('nothing on the board writes', async () => {
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
