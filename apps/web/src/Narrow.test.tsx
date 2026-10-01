// @vitest-environment jsdom

// Narrowing the board: Repo chips, Tag chips, one search box and one sort
// picker. None of them is worked out in the browser: each one goes to the
// server in the query string, so the lanes hold what `tasks list` would print
// under the same flags. The chips are drawn from the wide read, so narrowing
// never takes away the chip that would undo it.

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
import { answeringBy, asked, BOARD, task } from './testing'

const [HOUSE, WORK] = BOARD.repos
if (!HOUSE || !WORK) throw new Error('BOARD has two Repos')

// What the server answers under ?repo=work: that Repo and nothing else.
const ONLY_WORK = { ...BOARD, repos: [WORK] }

beforeEach(() => {
  asked.length = 0
  vi.stubGlobal('fetch', answeringBy({ '?repo=work': ONLY_WORK }))
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState(null, '', '/')
})

const group = (name: string) => within(screen.getByRole('group', { name }))
const lane = (name: string) => within(screen.getByRole('region', { name }))
const urls = () => asked.map((a) => a.url)

test('a Repo chip narrows the lanes to what the server read for it', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const chip = group('Repos').getByRole('button', { name: /^work/ })
  expect(chip.getAttribute('aria-pressed')).toBe('false')
  fireEvent.click(chip)

  await waitFor(() => expect(screen.queryByText('Wrap glassware')).toBeNull())
  expect(urls()).toContain('/api/state?repo=work')
  expect(lane('Inbox').getByText('Write the report')).toBeDefined()
  expect(chip.getAttribute('aria-pressed')).toBe('true')
  // The other Repo's chip is still there to switch to: chips come from the
  // wide read, not the narrowed one.
  expect(
    group('Repos').getByRole('button', { name: /^house-move/ }),
  ).toBeDefined()

  // Tapped again, it lets go, and the board is the wide read again.
  fireEvent.click(chip)
  expect(await screen.findByText('Wrap glassware')).toBeDefined()
  expect(chip.getAttribute('aria-pressed')).toBe('false')
})

test('a Repo whose file has problems is flagged on its chip', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  expect(
    group('Repos').getByRole('button', { name: 'work, has problems' }),
  ).toBeDefined()
  expect(
    group('Repos').getByRole('button', { name: 'house-move' }),
  ).toBeDefined()
})

// Several Tags widen within the field, as `-tag` repeated does.
test('Tag chips are any-of, each one another tag in the query', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  fireEvent.click(group('Tags').getByRole('button', { name: '#fragile' }))
  await waitFor(() => expect(urls()).toContain('/api/state?tag=fragile'))
  fireEvent.click(group('Tags').getByRole('button', { name: '#van' }))
  await waitFor(() =>
    expect(urls()).toContain('/api/state?tag=fragile&tag=van'),
  )
})

test('the search box asks the server over every lane', async () => {
  vi.stubGlobal(
    'fetch',
    answeringBy({
      '?search=van': {
        ...BOARD,
        repos: [
          {
            ...HOUSE,
            tasks: [task({ title: 'Book the van', state: 'backlog' })],
          },
          { ...WORK, tasks: [] },
        ],
      },
    }),
  )
  render(<App />)
  await screen.findByText('Wrap glassware')

  fireEvent.change(screen.getByRole('searchbox', { name: 'Search' }), {
    target: { value: 'van' },
  })

  await waitFor(() => expect(screen.queryByText('Wrap glassware')).toBeNull())
  expect(urls()).toContain('/api/state?search=van')
  expect(lane('Backlog').getByText('Book the van')).toBeDefined()
})

test('the sort picker offers the served sorts and sends the one picked', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const picker = screen.getByRole('combobox', { name: 'Sort' })
  expect(
    within(picker)
      .getAllByRole('option')
      .map((o) => o.textContent),
  ).toEqual(BOARD.sorts)
  expect((picker as HTMLSelectElement).value).toBe('file')

  fireEvent.change(picker, { target: { value: 'deadline' } })
  await waitFor(() => expect(urls()).toContain('/api/state?sort=deadline'))

  // File order is the server's own order, so it is the bare path again.
  asked.length = 0
  fireEvent.change(picker, { target: { value: 'file' } })
  await waitFor(() => expect(asked.length).toBeGreaterThan(0))
  expect(urls().every((url) => url === '/api/state')).toBe(true)
})
