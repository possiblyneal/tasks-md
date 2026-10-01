// @vitest-environment jsdom

// What the approval is over. A tick that writes six attributes and shows three
// is a gate over half of what it lets through, so this is about what reaches
// the screen rather than about what the Broker said.

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { Breakdown } from './Breakdown'
import type { Task } from './state'
import * as write from './write'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const TASK: Task = {
  id: 't1',
  title: 'Move house',
  state: 'doing',
  tags: [],
  created: '2026-03-04',
  attrs: [],
  description: '',
  line: 4,
  depth: 0,
  parent: '',
  parents: [],
  leaf: true,
  blocked: false,
  rank: 0,
  version: 'v0',
}

test('every attribute a proposal would write is drawn beside its tick', async () => {
  vi.spyOn(write, 'breakdown').mockResolvedValue({
    questions: [],
    proposals: [
      {
        title: 'Book the van',
        description: 'The big one, not the transit.',
        why: 'Nothing else moves until it is booked.',
        estimate: '30m',
        priority: 'high',
        impact: 'med',
      },
    ],
  })
  render(
    <Breakdown
      repo="house-move"
      task={TASK}
      offline={false}
      onBack={() => {}}
    />,
  )

  await screen.findByText('Book the van')
  expect(screen.getByText('The big one, not the transit.')).toBeDefined()
  expect(
    screen.getByText('Nothing else moves until it is booked.'),
  ).toBeDefined()
  // The labels are what is pinned; `Breakdown.tsx` is where they are argued.
  expect(screen.getByText('Estimate 30m')).toBeDefined()
  expect(screen.getByText('Priority high')).toBeDefined()
  expect(screen.getByText('Impact med')).toBeDefined()
})

// The screen draws what the Broker said and parses none of it. A level that is
// none of the three is the store's to refuse, and blanking it here would let it
// be approved without ever having been seen.
test('a level the store would refuse is drawn as the word the Broker used', async () => {
  vi.spyOn(write, 'breakdown').mockResolvedValue({
    questions: [],
    proposals: [{ title: 'Book the van', priority: 'urgent' }],
  })
  render(
    <Breakdown
      repo="house-move"
      task={TASK}
      offline={false}
      onBack={() => {}}
    />,
  )

  await screen.findByText('Book the van')
  expect(screen.getByText('Priority urgent')).toBeDefined()
})

// The line under a proposal holds the estimate and the two levels, and the
// Broker answers none of them on a Subtask it has nothing to say about. Drawn
// anyway it is an empty strip of nothing under the title, which reads as an
// attribute that failed to load rather than one nobody set.
test('the line of single words is not drawn when there are none', async () => {
  vi.spyOn(write, 'breakdown').mockResolvedValue({
    questions: [],
    proposals: [{ title: 'Book the van' }],
  })
  const drawn = render(
    <Breakdown
      repo="house-move"
      task={TASK}
      offline={false}
      onBack={() => {}}
    />,
  )

  await screen.findByText('Book the van')
  expect(drawn.container.querySelector('.facts')).toBeNull()
})

// The gate is that the write carries nothing the tick did not show, and that
// every ticked proposal goes in one write under the version the breakdown
// opened on, so a refusal leaves nothing half written.
test('approving writes the ticked proposals as one write and nothing added to them', async () => {
  vi.spyOn(write, 'breakdown').mockResolvedValue({
    questions: [],
    proposals: [
      { title: 'Book the van', estimate: 'small' },
      { title: 'Pack the books' },
      { title: 'Cancel the milk' },
    ],
  })
  const wrote = vi.spyOn(write, 'addSubtasks').mockResolvedValue(['t2', 't3'])
  const onBack = vi.fn()
  render(
    <Breakdown repo="house-move" task={TASK} offline={false} onBack={onBack} />,
  )

  fireEvent.click(await screen.findByLabelText('Pack the books'))
  fireEvent.click(screen.getByRole('button', { name: 'Add 2' }))

  await vi.waitFor(() => expect(onBack).toHaveBeenCalled())
  expect(wrote).toHaveBeenCalledTimes(1)
  expect(wrote).toHaveBeenCalledWith('house-move', 't1', 'v0', [
    { title: 'Book the van', estimate: 'small' },
    { title: 'Cancel the milk' },
  ])
})

test('offline, nothing is approved', async () => {
  vi.spyOn(write, 'breakdown').mockResolvedValue({
    questions: [],
    proposals: [{ title: 'Book the van' }],
  })
  render(
    <Breakdown
      repo="house-move"
      task={TASK}
      offline={true}
      onBack={() => {}}
    />,
  )

  await screen.findByText('Book the van')
  expect(screen.getByRole('button', { name: 'Add 1' })).toHaveProperty(
    'disabled',
    true,
  )
})

// A turn the Broker never answered loses nothing: the questions stay with what
// was typed into them, and asking again sends each answer once.
test('a failed turn keeps the questions and the replies to retry', async () => {
  const turn = vi
    .spyOn(write, 'breakdown')
    .mockResolvedValueOnce({ questions: ['Which van?'], proposals: [] })
    .mockRejectedValueOnce(new Error('the broker timed out'))
    .mockResolvedValueOnce({
      questions: [],
      proposals: [{ title: 'Book the big van' }],
    })
  render(
    <Breakdown
      repo="house-move"
      task={TASK}
      offline={false}
      onBack={() => {}}
    />,
  )

  fireEvent.change(await screen.findByLabelText('Which van?'), {
    target: { value: 'The big one' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Answer' }))

  await screen.findByText('the broker timed out')
  expect(screen.getByLabelText('Which van?')).toHaveProperty(
    'value',
    'The big one',
  )

  fireEvent.click(screen.getByRole('button', { name: 'Answer' }))
  await screen.findByText('Book the big van')
  expect(turn).toHaveBeenLastCalledWith('house-move', 't1', [
    { question: 'Which van?', answer: 'The big one' },
  ])
})
