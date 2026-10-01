import { expect, test } from 'vitest'

import { answer, keep, OFFLINE } from './offline'

// What the service worker does with each request, driven through `answer` and
// `keep` with a Map standing in for the Cache and a fetch that either reaches
// the server or does not. What is asserted is what the page would be handed.

type Kept = Pick<Cache, 'match' | 'put' | 'delete' | 'keys'>

function shelf(): Kept & { paths: () => string[] } {
  const held = new Map<string, Response>()
  const key = (r: RequestInfo | URL) => {
    const url = new URL(r instanceof Request ? r.url : String(r), ORIGIN)
    return url.pathname + url.search
  }
  return {
    match: (r) => Promise.resolve(held.get(key(r))?.clone()),
    put: (r, response) => {
      held.set(key(r), response.clone())
      return Promise.resolve()
    },
    delete: (r) => Promise.resolve(held.delete(key(r))),
    keys: () =>
      Promise.resolve([...held.keys()].map((k) => new Request(ORIGIN + k))),
    paths: () => [...held.keys()].sort(),
  } as Kept & { paths: () => string[] }
}

const ORIGIN = 'https://tasks.lan'

const PAGE = `<!doctype html><html><head>
<script type="module" crossorigin src="/assets/index-a1.js"></script>
<link rel="stylesheet" crossorigin href="/assets/index-b2.css">
</head><body><div id="root"></div></body></html>`

/** A server that answers every path it is given, and records each one. */
function server(routes: Record<string, () => Response>) {
  const asked: string[] = []
  const fetcher = (r: RequestInfo | URL) => {
    const path = new URL(r instanceof Request ? r.url : String(r), ORIGIN)
      .pathname
    asked.push(path)
    const route = routes[path]
    return route
      ? Promise.resolve(route())
      : Promise.resolve(new Response('', { status: 404 }))
  }
  return { fetcher, asked }
}

/** A network that is not there: what fetch does away from home Wi-Fi. */
const away = () => Promise.reject(new TypeError('Load failed'))

const later: Promise<unknown>[] = []
const after = (p: Promise<unknown>) => void later.push(p)

const board = (etag: string, body: string) =>
  new Response(body, { headers: { ETag: etag } })

function navigation(path: string): Request {
  const request = new Request(ORIGIN + path)
  Object.defineProperty(request, 'mode', { value: 'navigate' })
  return request
}

test('a board read that reaches the server is handed on and kept', async () => {
  const cache = shelf()
  const { fetcher } = server({ '/api/state': () => board('"1"', '{"a":1}') })

  const response = await answer(
    new Request(`${ORIGIN}/api/state?repo=work`),
    cache,
    fetcher,
    after,
  )

  expect(await response?.text()).toBe('{"a":1}')
  expect(response?.headers.has(OFFLINE)).toBe(false)
  expect(cache.paths()).toEqual(['/api/state?repo=work'])
})

test('a board read that cannot reach the server is the last one kept, marked offline', async () => {
  const cache = shelf()
  const home = server({ '/api/state': () => board('"1"', '{"a":1}') })
  await answer(new Request(`${ORIGIN}/api/state`), cache, home.fetcher, after)

  const response = await answer(
    new Request(`${ORIGIN}/api/state`),
    cache,
    away,
    after,
  )

  expect(response?.status).toBe(200)
  expect(await response?.text()).toBe('{"a":1}')
  expect(response?.headers.get('ETag')).toBe('"1"')
  expect(response?.headers.get(OFFLINE)).toBe('1')
})

test('a 304 is handed on and the board kept is not replaced by it', async () => {
  const cache = shelf()
  await cache.put(`${ORIGIN}/api/state`, board('"1"', '{"a":1}'))
  const { fetcher } = server({
    '/api/state': () => new Response(null, { status: 304 }),
  })

  const response = await answer(
    new Request(`${ORIGIN}/api/state`),
    cache,
    fetcher,
    after,
  )

  expect(response?.status).toBe(304)
  expect(await (await cache.match(`${ORIGIN}/api/state`))?.text()).toBe(
    '{"a":1}',
  )
})

// The page's first read is made before the worker takes it, so every read the
// worker sees afterwards carries a tag and would be answered 304 for ever,
// leaving nothing kept for away from home.
test('a read whose tag is not the board kept is asked whole, so one is kept', async () => {
  const cache = shelf()
  const sent: (string | null)[] = []
  const fetcher = (r: RequestInfo | URL) => {
    const tag = r instanceof Request ? r.headers.get('If-None-Match') : null
    sent.push(tag)
    return Promise.resolve(
      tag === '"1"'
        ? new Response(null, { status: 304 })
        : board('"1"', '{"a":1}'),
    )
  }
  const asking = () =>
    new Request(`${ORIGIN}/api/state`, {
      headers: { 'If-None-Match': '"1"' },
    })

  const first = await answer(asking(), cache, fetcher, after)
  const second = await answer(asking(), cache, fetcher, after)

  expect(first?.status).toBe(200)
  expect(second?.status).toBe(304)
  expect(sent).toEqual([null, '"1"'])
  expect(cache.paths()).toEqual(['/api/state'])
})

test('a refusal is handed on and not kept', async () => {
  const cache = shelf()
  const { fetcher } = server({
    '/api/state': () => new Response('{"error":"no"}', { status: 404 }),
  })

  const response = await answer(
    new Request(`${ORIGIN}/api/state?repo=nowhere`),
    cache,
    fetcher,
    after,
  )

  expect(response?.status).toBe(404)
  expect(cache.paths()).toEqual([])
})

test('with no board kept, a read that cannot reach the server fails as it would have', async () => {
  await expect(
    answer(new Request(`${ORIGIN}/api/state`), shelf(), away, after),
  ).rejects.toThrow('Load failed')
})

test('a write is left to the network', () => {
  expect(
    answer(
      new Request(`${ORIGIN}/api/tasks/m3qa/move`, { method: 'POST' }),
      shelf(),
      away,
      after,
    ),
  ).toBeNull()
})

test('opening the page away from home is the page kept', async () => {
  const cache = shelf()
  await keep(cache, server({}).fetcher, new Response(PAGE))

  const response = await answer(
    navigation('/?task=m3qa&repo=work'),
    cache,
    away,
    after,
  )

  expect(await response?.text()).toBe(PAGE)
})

test('the page opened at home is the fresh one, and it is kept afterwards', async () => {
  const cache = shelf()
  const { fetcher } = server({
    '/': () => new Response(PAGE),
    '/assets/index-a1.js': () => new Response('js'),
    '/assets/index-b2.css': () => new Response('css'),
  })
  later.length = 0

  const response = await answer(navigation('/'), cache, fetcher, after)
  await Promise.all(later)

  expect(await response?.text()).toBe(PAGE)
  expect(cache.paths()).toEqual([
    '/',
    '/assets/index-a1.js',
    '/assets/index-b2.css',
  ])
})

test('keeping a page keeps what it loads and drops what an older one did', async () => {
  const cache = shelf()
  await cache.put(`${ORIGIN}/assets/index-old.js`, new Response('old'))
  await cache.put(`${ORIGIN}/api/state`, board('"1"', '{}'))
  const { fetcher, asked } = server({
    '/assets/index-a1.js': () => new Response('js'),
    '/assets/index-b2.css': () => new Response('css'),
  })

  await keep(cache, fetcher, new Response(PAGE))

  expect(cache.paths()).toEqual([
    '/',
    '/api/state',
    '/assets/index-a1.js',
    '/assets/index-b2.css',
  ])
  expect(asked).toEqual(['/assets/index-a1.js', '/assets/index-b2.css'])
})

// The worker installs by keeping whatever `/` answered, which during a deploy
// can be a 502 that names no assets at all.
test('a page the server failed to answer keeps nothing and drops nothing', async () => {
  const cache = shelf()
  await cache.put(`${ORIGIN}/`, new Response(PAGE))
  await cache.put(`${ORIGIN}/assets/index-a1.js`, new Response('js'))
  const { fetcher, asked } = server({})

  await keep(cache, fetcher, new Response('Bad Gateway', { status: 502 }))

  expect(cache.paths()).toEqual(['/', '/assets/index-a1.js'])
  expect(await (await cache.match(`${ORIGIN}/`))?.text()).toBe(PAGE)
  expect(asked).toEqual([])
})

test('an asset already kept is not fetched again', async () => {
  const cache = shelf()
  await cache.put(`${ORIGIN}/assets/index-a1.js`, new Response('js'))
  const { fetcher, asked } = server({})

  const response = await answer(
    new Request(`${ORIGIN}/assets/index-a1.js`),
    cache,
    fetcher,
    after,
  )

  expect(await response?.text()).toBe('js')
  expect(asked).toEqual([])
})
