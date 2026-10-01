// @vitest-environment jsdom

// The weekly metrics: Added, Done and Declined for each of the last twelve
// weeks, as the read counted them. They follow the Repo and Tag chips because
// the read made under the chips carries its own.

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
import { answeringBy, asked, BOARD, weeks } from './testing'

const VAN = {
  ...BOARD,
  metrics: weeks({ '2026-09-28': { added: 0, done: 0, declined: 1 } }),
}

beforeEach(() => {
  asked.length = 0
  vi.stubGlobal('fetch', answeringBy({ '?tag=van': VAN }))
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const metrics = () => screen.getByRole('group', { name: 'Metrics' })

test('this week is in sight, and the twelve are one tap away, newest first', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const shown = within(metrics())
  expect(
    shown.getByText('This week: 2 added · 1 done · 0 declined'),
  ).toBeDefined()

  fireEvent.click(shown.getByText(/^This week/))
  const rows = shown
    .getAllByRole('row')
    .slice(1)
    .map((row) =>
      Array.from(row.children, (cell) => cell.textContent).join(' '),
    )
  expect(rows).toHaveLength(12)
  expect(rows.slice(0, 2)).toEqual(['2026-09-28 2 1 0', '2026-09-21 0 4 0'])
  expect(rows[11]).toBe('2026-07-13 0 0 0')
})

test('a Tag chip narrows the metrics to the read made under it', async () => {
  render(<App />)
  await screen.findByText('Wrap glassware')

  const tags = within(screen.getByRole('group', { name: 'Tags' }))
  fireEvent.click(tags.getByRole('button', { name: '#van' }))

  await waitFor(() =>
    expect(
      within(metrics()).getByText('This week: 0 added · 0 done · 1 declined'),
    ).toBeDefined(),
  )
  expect(asked.map((a) => a.url)).toContain('/api/state?tag=van')
})
