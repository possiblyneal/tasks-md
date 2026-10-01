// @vitest-environment jsdom

// A Task carrying a Series shows its rule in the panel with the next dates the
// server works out from it, asked for when the panel opens. A Task without one
// asks for nothing.

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { App } from './App'
import { asked, BOARD, task } from './testing'

const RULE = 'every week on mon from 2026-01-05'

const SERIES_BOARD = {
  ...BOARD,
  repos: [
    {
      ...BOARD.repos[0]!,
      tasks: [
        task({
          id: 'bin1',
          title: 'Take the bins out',
          state: 'backlog',
          attrs: [
            { label: 'deadline', value: '2026-10-05' },
            { label: 'series', value: RULE },
          ],
        }),
        task({ id: 'v9t1', title: 'Book the van', state: 'backlog', line: 6 }),
      ],
    },
  ],
}

function routing(series: { status: number; body: unknown }) {
  return (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    if (url.includes('/series')) {
      return Promise.resolve(
        new Response(JSON.stringify(series.body), { status: series.status }),
      )
    }
    return Promise.resolve(
      new Response(JSON.stringify(SERIES_BOARD), { headers: { ETag: '"1"' } }),
    )
  }
}

beforeEach(() => {
  asked.length = 0
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState(null, '', '/')
})

const open = async (title: string) => {
  fireEvent.click(await screen.findByRole('button', { name: title }))
  return within(screen.getByRole('dialog', { name: title }))
}

test('the panel shows a Series rule and the next dates the server works out', async () => {
  vi.stubGlobal(
    'fetch',
    routing({
      status: 200,
      body: { rule: RULE, dates: ['2026-10-05', '2026-10-12', '2026-10-19'] },
    }),
  )
  render(<App />)
  const panel = await open('Take the bins out')

  expect(panel.getByText(RULE)).toBeDefined()
  const next = await panel.findByRole('list', { name: 'Next dates' })
  expect(
    within(next)
      .getAllByRole('listitem')
      .map((li) => li.textContent),
  ).toEqual(['2026-10-05', '2026-10-12', '2026-10-19'])
  expect(asked.map((a) => a.url)).toContain(
    '/api/tasks/bin1/series?repo=house-move',
  )
})

test('a refused Series read says why in the API’s words', async () => {
  vi.stubGlobal(
    'fetch',
    routing({ status: 404, body: { error: 'no Task ^bin1' } }),
  )
  render(<App />)
  const panel = await open('Take the bins out')

  expect(await panel.findByText('no Task ^bin1')).toBeDefined()
  expect(panel.getByText(RULE)).toBeDefined()
})

test('a Task with no Series asks for none', async () => {
  vi.stubGlobal('fetch', routing({ status: 200, body: {} }))
  render(<App />)
  await open('Book the van')

  expect(asked.some((a) => a.url.includes('/series'))).toBe(false)
})
