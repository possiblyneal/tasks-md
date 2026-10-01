// @vitest-environment jsdom

// A Task's history in the panel: what happened to it, newest first, each
// write with its Actor, read from the server when the panel opens and again
// whenever the Task changes. A direct edit's Actor is unknown, and a Task with
// no history yet shows that rather than an error.

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { App } from './App'
import { asked, BOARD } from './testing'

function routing(history: { status: number; body: unknown }) {
  return (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    if (url.includes('/history')) {
      return Promise.resolve(
        new Response(JSON.stringify(history.body), { status: history.status }),
      )
    }
    return Promise.resolve(
      new Response(JSON.stringify(BOARD), { headers: { ETag: '"1"' } }),
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

test('the panel lists what happened to the Task, newest first, with each Actor', async () => {
  vi.stubGlobal(
    'fetch',
    routing({
      status: 200,
      body: {
        entries: [
          {
            at: '2026-09-30T18:02:00+02:00',
            actor: 'neal',
            subject: 'edit ^v9t1 Book the van',
          },
          {
            at: '2026-09-30T09:15:00+02:00',
            actor: 'claude-code/opus',
            subject: 'move ^v9t1 to Doing',
          },
          {
            at: '2026-09-29T21:40:00+02:00',
            actor: '',
            subject: 'commit a direct edit',
          },
        ],
      },
    }),
  )
  render(<App />)
  const panel = await open('Book the van')

  const history = await panel.findByRole('list', { name: 'History' })
  const items = within(history)
    .getAllByRole('listitem')
    .map((li) => li.textContent)
  expect(items).toHaveLength(3)
  expect(items[0]).toContain('edit ^v9t1 Book the van')
  expect(items[0]).toContain('neal')
  expect(items[0]).toContain('2026-09-30')
  expect(items[1]).toContain('move ^v9t1 to Doing')
  expect(items[1]).toContain('claude-code/opus')
  expect(items[2]).toContain('commit a direct edit')
  expect(items[2]).toContain('Actor unknown')
  expect(asked.map((a) => a.url)).toContain(
    '/api/tasks/v9t1/history?repo=house-move',
  )
})

test('a Task with no history yet says so rather than failing', async () => {
  vi.stubGlobal('fetch', routing({ status: 200, body: { entries: [] } }))
  render(<App />)
  const panel = await open('Book the van')

  expect(
    await panel.findByText('Nothing has happened to it yet.'),
  ).toBeDefined()
})

test('a refused history read says why in the API’s words', async () => {
  vi.stubGlobal(
    'fetch',
    routing({ status: 404, body: { error: 'no Task ^v9t1' } }),
  )
  render(<App />)
  const panel = await open('Book the van')

  expect(await panel.findByText('no Task ^v9t1')).toBeDefined()
})
