import { afterEach, expect, test, vi } from 'vitest'

import { OFFLINE } from './offline'
import {
  fetchFiles,
  fetchHistory,
  fetchSeries,
  fetchState,
  queryString,
  WIDE,
} from './state'

// What the last call asked for, which is how the conditional request is
// checked: the ETag only earns its keep if it is actually sent back.
let asked: RequestInit | undefined
// Where it asked, which is how the narrowing is checked: the query string is
// the whole of what tells the API which Tasks the screen wants.
let at: string | undefined

function answering(
  status: number,
  body: unknown,
  headers: Record<string, string> = {},
) {
  return (url: string, init?: RequestInit) => {
    asked = init
    at = url
    return Promise.resolve(
      new Response(status === 304 ? null : JSON.stringify(body), {
        status,
        headers,
      }),
    )
  }
}

afterEach(() => {
  asked = undefined
  at = undefined
  vi.unstubAllGlobals()
})

test('a read returns the board and the tag it came with', async () => {
  vi.stubGlobal(
    'fetch',
    answering(
      200,
      {
        repos: [
          { name: 'work', path: '/c/work', color: '', tasks: [], problems: [] },
        ],
        errors: [],
      },
      { ETag: '"abc"' },
    ),
  )

  const snapshot = await fetchState(WIDE, null)

  expect(snapshot?.etag).toBe('"abc"')
  expect(snapshot?.board.repos[0]?.name).toBe('work')
  expect(asked?.headers).toEqual({})
  expect(snapshot?.offline).toBe(false)
})

// The service worker answers a read it could not get from the server with the
// last board it kept, and marks it, so the board knows it is not live.
test('a board the service worker kept is read as offline', async () => {
  vi.stubGlobal(
    'fetch',
    answering(200, { repos: [], errors: [] }, { [OFFLINE]: '1' }),
  )

  expect((await fetchState(WIDE, null))?.offline).toBe(true)
})

test('unchanged files are nothing to redraw rather than an empty board', async () => {
  vi.stubGlobal('fetch', answering(304, null))

  expect(await fetchState(WIDE, '"abc"')).toBeNull()
  expect(asked?.headers).toEqual({ 'If-None-Match': '"abc"' })
})

test('a refusal surfaces the sentence the CLI would have printed', async () => {
  const sentence = 'no Repo is named nowhere'
  vi.stubGlobal('fetch', answering(404, { error: sentence }))

  await expect(fetchState(WIDE, null)).rejects.toThrow(sentence)
})

test('an error with no sentence in it still says what happened', async () => {
  vi.stubGlobal('fetch', () =>
    Promise.resolve(new Response('nope', { status: 500 })),
  )

  await expect(fetchState(WIDE, null)).rejects.toThrow('the API answered 500')
})

// The wide view is the bare path. A query string of empty values would be a
// second spelling of the same representation, and the ETag is hashed over the
// query, so the two would never share a cached answer.
test('narrowed by nothing is the path and no query at all', () => {
  expect(queryString(WIDE)).toBe('')
})

// The names are `tasks list`'s flags, and a set is the parameter repeated,
// which is how the route reads several values.
test('each narrowing is sent under the name the route reads it by', () => {
  expect(
    queryString({
      repo: 'work',
      states: ['doing', 'inbox'],
      tags: ['kitchen', 'van'],
      search: '50% off',
      unblocked: true,
      blocked: false,
      sort: 'deadline',
    }),
  ).toBe(
    '?repo=work&state=doing&state=inbox&tag=kitchen&tag=van&search=50%25+off&unblocked=true&sort=deadline',
  )
  expect(queryString({ ...WIDE, blocked: true })).toBe('?blocked=true')
})

// File order is what the read gives when no sort is named, so naming it is a
// second spelling of the bare path and is left out the way an empty value is.
test('file order is the bare path', () => {
  expect(queryString({ ...WIDE, sort: 'file' })).toBe('')
})

test('the read asks under the narrowing it was given', async () => {
  vi.stubGlobal('fetch', answering(200, { repos: [], errors: [] }))

  await fetchState({ ...WIDE, repo: 'work' }, null)

  expect(at).toBe('/api/state?repo=work')
})

// The panel's and the picker's own reads: each asks where its route is and
// hands back what came, or the refusal in the API's words.

test('a Series is asked for by id under its Repo', async () => {
  const series = { rule: 'weekly', dates: ['2026-03-09'] }
  vi.stubGlobal('fetch', answering(200, series))

  expect(await fetchSeries('house move', 'a/b')).toEqual(series)
  expect(at).toBe('/api/tasks/a%2Fb/series?repo=house%20move')
})

test('a history is the entries it answered', async () => {
  const entry = { at: '2026-03-04T10:00:00Z', actor: '', subject: 'add' }
  vi.stubGlobal('fetch', answering(200, { entries: [entry] }))

  expect(await fetchHistory('work', 't1')).toEqual([entry])
  expect(at).toBe('/api/tasks/t1/history?repo=work')
})

test('files are listed at the path given, or the default with none', async () => {
  const files = { path: '/home', parent: '/', entries: [] }
  vi.stubGlobal('fetch', answering(200, files))

  expect(await fetchFiles('/a b')).toEqual(files)
  expect(at).toBe('/api/files?path=%2Fa%20b')
  await fetchFiles()
  expect(at).toBe('/api/files')
})

test("a refused Series, history or listing says why in the API's words", async () => {
  vi.stubGlobal('fetch', answering(404, { error: 'no Task t9 in work' }))

  await expect(fetchSeries('work', 't9')).rejects.toThrow('no Task t9 in work')
  await expect(fetchHistory('work', 't9')).rejects.toThrow('no Task t9 in work')
  await expect(fetchFiles('/gone')).rejects.toThrow('no Task t9 in work')
})
