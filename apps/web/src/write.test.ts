import { afterEach, expect, test, vi } from 'vitest'

import {
  addSubtask,
  addTask,
  ask,
  attach,
  breakdown,
  capture,
  memberships,
} from './write'
import { queryString, WIDE } from './state'

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
      estimate: '90m',
      intoLists: ['list_house'],
    }),
  )

  const draft = await capture('paint the fence before march')

  expect(sent?.url).toBe('/api/capture')
  expect(sent?.init?.method).toBe('POST')
  expect(body()).toEqual({ text: 'paint the fence before march' })
  expect(draft.title).toBe('Paint the fence')
  expect(draft.intoLists).toEqual(['list_house'])
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

  const id = await addTask({ title: 'Paint the fence', intoLists: ['list_a'] })

  expect(id).toBe('task_abc')
  expect(body()).toEqual({ title: 'Paint the fence', intoLists: ['list_a'] })
})

test('a refusal surfaces the sentence the CLI would have printed', async () => {
  const sentence =
    'cannot read "next tuesday" as a date: want 2006-01-02, 2006-01-02 15:04, or RFC 3339'
  vi.stubGlobal('fetch', answering(400, { error: sentence }))

  await expect(addTask({ deadline: 'next tuesday' })).rejects.toThrow(sentence)
})

test('an error with no sentence in it still says what happened', async () => {
  vi.stubGlobal('fetch', () =>
    Promise.resolve(new Response('nope', { status: 500 })),
  )

  await expect(capture('paint the fence')).rejects.toThrow(
    'the API answered 500',
  )
})

test('the sheet submits the memberships that changed, both ways', () => {
  const before = { intoLists: ['house'], addTags: ['outdoors'] }
  const after = { intoLists: ['garage'], addTags: ['outdoors', 'spring'] }

  expect(memberships(before, after)).toEqual({
    intoLists: ['garage'],
    outOfLists: ['house'],
    addTags: ['spring'],
    dropTags: [],
  })
})

// A sheet opened on a draft carries nothing to leave, so every tick is a join.
test('a draft with no memberships joins whatever is ticked', () => {
  expect(memberships({}, { intoLists: ['house'] })).toEqual({
    intoLists: ['house'],
    outOfLists: [],
    addTags: [],
    dropTags: [],
  })
})

test('a subtask is written under the task in the path', async () => {
  vi.stubGlobal('fetch', answering(201, { id: 'task_child' }))

  const id = await addSubtask('task_parent', { title: 'Buy the paint' })

  expect(id).toBe('task_child')
  expect(sent?.url).toBe('/api/tasks/task_parent/subtasks')
  expect(body()).toEqual({ title: 'Buy the paint' })
})

// The target rides in the body and not the path, because a pointer carries its
// own slashes. Both sides have to agree on the field name for that to work, and
// `decode` refuses a field it does not know, so a rename on either side is a
// 400 nothing else would catch.
test('a pointer is added in the body under the task in the path', async () => {
  vi.stubGlobal('fetch', answering(200, { id: 'task_abc' }))

  await attach('task_abc', '/home/neal/plans/shed.pdf')

  expect(sent?.url).toBe('/api/tasks/task_abc/attachments')
  expect(sent?.init?.method).toBe('POST')
  expect(body()).toEqual({ target: '/home/neal/plans/shed.pdf' })
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

// `POST /api/tasks` refuses a field it does not know and `store.Attach` is a
// guarded write against a Task that has to exist first, so the pointers are
// split off the create and written one at a time after it.
test('an attachment on the body is written after the task it is for', async () => {
  const calls: { url: string; body: Record<string, unknown> }[] = []
  vi.stubGlobal('fetch', (url: string, init?: RequestInit) => {
    calls.push({
      url,
      body: JSON.parse(String(init?.body)) as Record<string, unknown>,
    })
    return Promise.resolve(
      new Response(JSON.stringify({ id: 'task_abc' }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })

  await addTask({ title: 'Paint the fence', attachments: ['/a', '/b'] })

  expect(calls.map((one) => one.url)).toEqual([
    '/api/tasks',
    '/api/tasks/task_abc/attachments',
    '/api/tasks/task_abc/attachments',
  ])
  // The create carries no trace of them: the route would refuse the field.
  expect(calls[0]?.body).toEqual({ title: 'Paint the fence' })
  expect(calls.slice(1).map((one) => one.body)).toEqual([
    { target: '/a' },
    { target: '/b' },
  ])
})

// Every other way a submit fails leaves nothing behind, so a refused pointer
// has to say that this one did not: the form would otherwise offer the same
// button again and a second press would write a second Task.
test('a refused attachment says the task was written', async () => {
  let written = false
  vi.stubGlobal('fetch', (url: string) => {
    if (String(url).endsWith('/attachments')) {
      return Promise.resolve(
        new Response(JSON.stringify({ error: 'a pointer cannot be empty' }), {
          status: 400,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
    }
    written = true
    return Promise.resolve(
      new Response(JSON.stringify({ id: 'task_abc' }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })

  await expect(
    addTask({ title: 'Paint the fence', attachments: ['/a', '/b'] }),
  ).rejects.toThrow(/the task was written, and 0 of 2 attachments with it/)
  expect(written).toBe(true)
})

// A Subtask is the same write with a parent, so it splits them the same way.
// It has its own test because `Breakdown.test.tsx` mocks `addSubtask` whole,
// which leaves nothing reaching this path.
test('an attachment on a subtask is written after the subtask exists', async () => {
  const calls: string[] = []
  vi.stubGlobal('fetch', (url: string) => {
    calls.push(String(url))
    return Promise.resolve(
      new Response(JSON.stringify({ id: 'task_kid' }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })

  await addSubtask('task_abc', { title: 'Sand it', attachments: ['/a'] })

  expect(calls).toEqual([
    '/api/tasks/task_abc/subtasks',
    '/api/tasks/task_kid/attachments',
  ])
})
