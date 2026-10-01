import { afterEach, expect, test, vi } from 'vitest'

import {
  addSubtask,
  addTask,
  ask,
  blank,
  breakdown,
  capture,
  deleteTask,
  draftOf,
  editTask,
} from './write'
import { queryString, WIDE } from './state'
import { task } from './testing'

// What the last call sent, which is how the body is checked: the routes take
// JSON and a client that posted something else would still get a Response.
let sent: { url: string; init?: RequestInit } | undefined

function answering(status: number, body: unknown) {
  return (url: string, init?: RequestInit) => {
    sent = { url, init }
    return Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  }
}

function body() {
  return JSON.parse(String(sent?.init?.body)) as Record<string, unknown>
}

afterEach(() => {
  sent = undefined
  vi.unstubAllGlobals()
})

test('a dump goes to the broker and comes back as the body the sheet submits', async () => {
  vi.stubGlobal(
    'fetch',
    answering(200, {
      title: 'Paint the fence',
      estimate: 'medium',
      tags: ['house'],
    }),
  )

  const draft = await capture('paint the fence before march')

  expect(sent?.url).toBe('/api/capture')
  expect(sent?.init?.method).toBe('POST')
  expect(body()).toEqual({ text: 'paint the fence before march' })
  expect(draft.title).toBe('Paint the fence')
  expect(draft.tags).toEqual(['house'])
})

test('a question comes back as prose', async () => {
  vi.stubGlobal('fetch', answering(200, { answer: 'Start with the fence.' }))

  expect(await ask('what first?', WIDE)).toBe('Start with the fence.')
  expect(sent?.url).toBe('/api/ask')
  expect(body()).toEqual({ question: 'what first?' })
})

// The question is about the Tasks on the screen. `POST /api/ask` reads the
// query string the way `GET /api/state` does, so the narrowing goes on the
// question or the Broker answers about a list nobody is looking at.
test('a question is asked about the list as it is narrowed', async () => {
  vi.stubGlobal('fetch', answering(200, { answer: 'Two of them are overdue.' }))
  const narrowing = { ...WIDE, repo: 'work', states: ['doing' as const] }

  await ask('how many?', narrowing)

  expect(sent?.url).toBe('/api/ask?repo=work&state=doing')
  // The same string the poll carries, built by the same function: the two
  // agreeing is the point, and a second spelling is how they stop agreeing.
  expect(sent?.url).toBe(`/api/ask${queryString(narrowing)}`)
})

test('a written task is named by the id the api answers with', async () => {
  vi.stubGlobal('fetch', answering(201, { id: 'task_abc' }))

  const draft = { ...blank('house-move'), title: 'Paint the fence' }
  const id = await addTask(draft)

  expect(id).toBe('task_abc')
  expect(sent?.url).toBe('/api/tasks')
  expect(body()).toEqual(draft)
})

test('a refusal surfaces the sentence the CLI would have printed', async () => {
  const sentence =
    'cannot read "next tuesday" as a date: want 2006-01-02, 2006-01-02 15:04, or RFC 3339'
  vi.stubGlobal('fetch', answering(400, { error: sentence }))

  await expect(
    addTask({ ...blank('work'), deadline: 'next tuesday' }),
  ).rejects.toThrow(sentence)
})

test('an error with no sentence in it still says what happened', async () => {
  vi.stubGlobal('fetch', () =>
    Promise.resolve(new Response('nope', { status: 500 })),
  )

  await expect(capture('paint the fence')).rejects.toThrow(
    'the API answered 500',
  )
})

test('a subtask is written under the task in the path', async () => {
  vi.stubGlobal('fetch', answering(201, { id: 'task_child' }))

  const id = await addSubtask('task_parent', { title: 'Buy the paint' })

  expect(id).toBe('task_child')
  expect(sent?.url).toBe('/api/tasks/task_parent/subtasks')
  expect(body()).toEqual({ title: 'Buy the paint' })
})

test('a breakdown turn carries everything already answered', async () => {
  vi.stubGlobal(
    'fetch',
    answering(200, { proposals: [{ title: 'Sand it' }, { title: 'Sand it' }] }),
  )

  const step = await breakdown('task_abc', [
    { question: 'How big is the fence?', answer: 'Six panels.' },
  ])

  expect(sent?.url).toBe('/api/breakdown')
  expect(body()).toEqual({
    task: 'task_abc',
    answers: [{ question: 'How big is the fence?', answer: 'Six panels.' }],
  })
  // Two proposals saying the same thing stay two, because only the order tells
  // them apart and approval is by position.
  expect(step.proposals).toHaveLength(2)
})

const WRAPPED = task({
  id: 'm3qc',
  title: 'Wrap glassware',
  state: 'doing',
  tags: ['fragile'],
  version: 'abc123',
  attrs: [
    { label: 'deadline', value: '2026-10-10' },
    { label: 'blocked by', value: 'm3qb, v9t1' },
    { label: 'attachment', value: '/a' },
    { label: 'attachment', value: '/b' },
    { label: 'reach', value: 'top shelf' },
  ],
})

test('a task opens in the form with the attributes the read carries', () => {
  const draft = draftOf('house-move', WRAPPED)

  expect(draft.repo).toBe('house-move')
  expect(draft.deadline).toBe('2026-10-10')
  expect(draft.blockedBy).toEqual(['m3qb', 'v9t1'])
  expect(draft.attach).toEqual(['/a', '/b'])
  expect(draft.priority).toBe('')
})

// An edit sends what changed and nothing else, so an attribute the form does
// not show, or one somebody else changed meanwhile, is left as it is.
test('an edit sends the version and only what changed', async () => {
  vi.stubGlobal('fetch', answering(200, { id: 'm3qc' }))
  const before = draftOf('house-move', WRAPPED)

  await editTask('m3qc', 'abc123', before, {
    ...before,
    priority: 'high',
    tags: ['fragile', 'kitchen'],
    attach: ['/b', '/c'],
  })

  expect(sent?.url).toBe('/api/tasks/m3qc')
  expect(sent?.init?.method).toBe('POST')
  expect(body()).toEqual({
    repo: 'house-move',
    version: 'abc123',
    priority: 'high',
    tags: ['fragile', 'kitchen'],
    attach: ['/c'],
    detach: ['/a'],
  })
})

test('a delete names the repo and the version it read', async () => {
  vi.stubGlobal('fetch', answering(200, { id: 'm3qc' }))

  await deleteTask('house-move', 'm3qc', 'abc123')

  expect(sent?.url).toBe('/api/tasks/m3qc/delete')
  expect(body()).toEqual({ repo: 'house-move', version: 'abc123' })
})

test('a stale write surfaces the conflict in the api words', async () => {
  const sentence = "^m3qc's tree has changed since it was read; read it again"
  vi.stubGlobal('fetch', answering(409, { error: sentence }))

  await expect(deleteTask('house-move', 'm3qc', 'old')).rejects.toThrow(
    sentence,
  )
})
