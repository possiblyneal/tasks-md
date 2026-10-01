// The service worker, built to `/sw.js` so its scope is the whole board. It is
// only the wiring: what each request is answered with is `offline.ts`'s.
//
// The worker's own types live in TypeScript's `webworker` lib, which cannot sit
// beside the `DOM` one the rest of the client compiles against, so the little
// of it used here is declared here.

import { answer, keep } from './offline'

type Extendable = Event & { waitUntil(work: Promise<unknown>): void }
type Fetching = Extendable & {
  request: Request
  respondWith(response: Promise<Response>): void
}

declare const self: {
  addEventListener(
    type: 'install' | 'activate',
    on: (e: Extendable) => void,
  ): void
  addEventListener(type: 'fetch', on: (e: Fetching) => void): void
  skipWaiting(): Promise<void>
  clients: { claim(): Promise<void> }
}

/** One cache, replaced whole only if what it holds changes shape. */
const CACHE = 'tasks-v1'

// An installed board on an iPhone keeps a worker of its own, apart from
// Safari's, so the first time it opens is when this installs. The page and
// what it loads are kept then, rather than on the next open, and the worker
// takes the open page at once, so the board read it is about to poll is kept
// too.
self.addEventListener('install', (event) => {
  event.waitUntil(
    (async () => {
      const cache = await caches.open(CACHE)
      await keep(cache, fetch, await fetch('/'))
      await self.skipWaiting()
    })(),
  )
})

self.addEventListener('activate', (event) => {
  event.waitUntil(self.clients.claim())
})

self.addEventListener('fetch', (event) => {
  const response = answer(
    event.request,
    {
      match: async (r) => (await caches.open(CACHE)).match(r),
      put: async (r, v) => (await caches.open(CACHE)).put(r, v),
      delete: async (r) => (await caches.open(CACHE)).delete(r),
      keys: async () => (await caches.open(CACHE)).keys(),
    },
    fetch,
    (work) => event.waitUntil(work),
  )
  if (response) event.respondWith(response)
})
