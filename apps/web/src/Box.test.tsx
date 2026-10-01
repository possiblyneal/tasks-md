// @vitest-environment jsdom

// The box hands a dump to the Broker and a question to it, and writes neither:
// a dump comes back as a Draft for the form, which is the gate.

import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { App } from './App'
import { Box } from './Box'
import { WIDE } from './state'
import { asked, BOARD } from './testing'
import * as write from './write'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function say(words: string) {
  fireEvent.change(screen.getByRole('textbox'), { target: { value: words } })
}

test('a dump opens a Draft in the Repo the Broker guessed and writes nothing', async () => {
  vi.spyOn(write, 'capture').mockResolvedValue({
    repo: 'work',
    title: 'Write the report',
    priority: 'high',
  })
  const addTask = vi.spyOn(write, 'addTask')
  const onDraft = vi.fn()
  render(
    <Box
      repos={['house-move', 'work']}
      narrowing={WIDE}
      offline={false}
      onDraft={onDraft}
    />,
  )

  say('the report, it is urgent')
  fireEvent.click(screen.getByRole('button', { name: 'Read' }))

  await vi.waitFor(() =>
    expect(onDraft).toHaveBeenCalledWith({
      ...write.blank('work'),
      title: 'Write the report',
      priority: 'high',
    }),
  )
  expect(write.capture).toHaveBeenCalledWith('the report, it is urgent')
  expect(addTask).not.toHaveBeenCalled()
})

test('a dump the Broker placed nowhere goes in the first Repo', async () => {
  vi.spyOn(write, 'capture').mockResolvedValue({ repo: '', title: 'Hmm' })
  const onDraft = vi.fn()
  render(
    <Box
      repos={['house-move', 'work']}
      narrowing={WIDE}
      offline={false}
      onDraft={onDraft}
    />,
  )

  say('hmm')
  fireEvent.click(screen.getByRole('button', { name: 'Read' }))

  await vi.waitFor(() =>
    expect(onDraft).toHaveBeenCalledWith(
      expect.objectContaining({ repo: 'house-move', title: 'Hmm' }),
    ),
  )
})

test('? asks about the Tasks in view and draws the answer above the field', async () => {
  const narrowing = { ...WIDE, repo: 'work' }
  vi.spyOn(write, 'ask').mockResolvedValue('Only the report is left.')
  render(
    <Box
      repos={['work']}
      narrowing={narrowing}
      offline={false}
      onDraft={() => {}}
    />,
  )

  fireEvent.click(
    screen.getByRole('button', { name: 'Adding: switch to asking' }),
  )
  say('what is left?')
  fireEvent.click(screen.getByRole('button', { name: 'Ask' }))

  expect(await screen.findByText('Only the report is left.')).toBeDefined()
  expect(write.ask).toHaveBeenCalledWith('what is left?', narrowing)
})

test('offline, the box neither reads a dump nor asks', () => {
  render(
    <Box repos={['work']} narrowing={WIDE} offline={true} onDraft={() => {}} />,
  )
  say('anything')
  for (const button of screen.getAllByRole('button')) {
    expect(button).toHaveProperty('disabled', true)
  }
  expect(screen.getByRole('textbox')).toHaveProperty('disabled', true)
})

// On the board: the box opens the add form, and only the form's Add writes.
describe('under the board', () => {
  const writes = () =>
    asked.filter(({ init }) => init?.method === 'POST').map(({ url }) => url)

  beforeEach(() => {
    asked.length = 0
    vi.stubGlobal('fetch', (url: string, init?: RequestInit) => {
      asked.push({ url, init })
      const body =
        url === '/api/capture'
          ? { repo: 'house-move', title: 'Sand the shed' }
          : url === '/api/tasks'
            ? { id: 'n3w1' }
            : BOARD
      return Promise.resolve(new Response(JSON.stringify(body)))
    })
  })

  afterEach(() => vi.unstubAllGlobals())

  const dump = async () => {
    render(<App />)
    fireEvent.change(await screen.findByLabelText('Say the Task'), {
      target: { value: 'sand the shed' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Read' }))
    return within(await screen.findByRole('dialog', { name: 'Add a Task' }))
  }

  test('a dump abandoned in the form writes nothing and keeps its words', async () => {
    const form = await dump()
    expect(form.getByLabelText('Title')).toHaveProperty(
      'value',
      'Sand the shed',
    )
    fireEvent.click(form.getByRole('button', { name: 'Cancel' }))

    expect(writes()).toEqual(['/api/capture'])
    expect(screen.getByLabelText('Say the Task')).toHaveProperty(
      'value',
      'sand the shed',
    )
  })

  test("a dump's Task is written by the form's Add, and the box empties", async () => {
    const form = await dump()
    fireEvent.click(form.getByRole('button', { name: 'Add' }))

    await vi.waitFor(() =>
      expect(screen.getByLabelText('Say the Task')).toHaveProperty('value', ''),
    )
    expect(writes()).toEqual(['/api/capture', '/api/tasks'])
  })
})
