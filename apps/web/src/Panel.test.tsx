// @vitest-environment jsdom

// The panel: tapping a card opens every attribute it has, and a
// Subtask's panel opens on its parent, with the parent's worked-out State and
// how many of its leaves sit in each State. `?task=<id>&repo=<name>` opens a
// panel on load, and opening or closing one keeps the address in step, so a
// panel can be linked to.

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { App } from './App'
import { answering, asked, BOARD } from './testing'

beforeEach(() => {
  asked.length = 0
  vi.stubGlobal('fetch', answering(BOARD))
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

test('tapping a card opens every attribute it has, read-only', async () => {
  render(<App />)
  const panel = await open('Wrap glassware')

  expect(panel.getByText('State').nextElementSibling?.textContent).toBe('Doing')
  for (const said of [
    'm3qc',
    'house-move',
    '2026-09-20',
    '#fragile',
    'due 2026-10-10',
    'Newspaper first, then the boxes marked G.',
  ]) {
    expect(panel.getByText(said)).toBeDefined()
  }
  // Nothing it shows is a field: the one textbox is the dump that amends it.
  expect(
    panel
      .queryAllByRole('textbox')
      .map((box) => box.getAttribute('aria-label')),
  ).toEqual(['Say what changes'])
  expect(panel.queryAllByRole('checkbox')).toEqual([])
  expect(window.location.search).toBe('?task=m3qc&repo=house-move')

  fireEvent.click(panel.getByRole('button', { name: 'Close' }))
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(window.location.search).toBe('')
})

test('a Subtask opens on its parent, its State and its leaves per State', async () => {
  render(<App />)
  const panel = await open('Wrap glassware')

  const parent = panel.getByRole('button', { name: /Pack the kitchen/ })
  expect(within(parent).getByText('Doing')).toBeDefined()
  expect(within(parent).getByText('Doing 1 · Done 1')).toBeDefined()

  // The parent is a Task like any other: tapping it opens its own panel,
  // which lists the Subtasks it holds.
  fireEvent.click(parent)
  const own = within(screen.getByRole('dialog', { name: 'Pack the kitchen' }))
  expect(own.getByText('Doing 1 · Done 1')).toBeDefined()
  expect(own.getByRole('button', { name: /Wrap glassware/ })).toBeDefined()
  expect(own.getByRole('button', { name: /Buy boxes/ })).toBeDefined()
  expect(window.location.search).toBe('?task=m3qa&repo=house-move')
})

test('Escape closes the panel', async () => {
  render(<App />)
  await open('Book the van')

  fireEvent.keyDown(document, { key: 'Escape' })
  expect(screen.queryByRole('dialog')).toBeNull()
})

// An id is unique within a Repo, not across them, so the link names both.
test('a link naming a Task and its Repo opens its panel on load', async () => {
  window.history.replaceState(null, '', '/?task=v9t1&repo=house-move')
  render(<App />)

  const panel = within(
    await screen.findByRole('dialog', { name: 'Book the van' }),
  )
  expect(panel.getByText('blocked')).toBeDefined()
})

test('a link with no Repo opens the first Task with that id', async () => {
  window.history.replaceState(null, '', '/?task=m3qb')
  render(<App />)

  expect(await screen.findByRole('dialog', { name: 'Buy boxes' })).toBeDefined()
})

test('a link to a Task the board does not have says so', async () => {
  window.history.replaceState(null, '', '/?task=zzzz&repo=work')
  render(<App />)

  expect(
    await screen.findByText('No Task zzzz is in work on the board.'),
  ).toBeDefined()
  expect(screen.queryByRole('dialog')).toBeNull()
})
