// What the service worker answers, kept apart from `sw.ts` so it is testable
// without a worker. Away from home Wi-Fi `tasks.lan` does not resolve, so the
// installed board has to open from what it kept: the page, the files the page
// loads, and the last board read under each query.
//
// Nothing here writes. A write is left to the network, so away from home it
// fails the way it would have without a worker; the board disables every
// write control before it gets that far.

/** The header a board read answered from the cache carries. */
export const OFFLINE = 'X-Tasks-Offline'

/** The page every navigation is, whatever its query names. */
const PAGE = '/'

type Kept = Pick<Cache, 'match' | 'put' | 'delete' | 'keys'>
type Fetch = (request: RequestInfo | URL) => Promise<Response>

/**
 * The response the worker answers `request` with, or `null` for one it leaves
 * to the network. `later` holds the worker open for work that need not delay
 * the answer.
 */
export function answer(
  request: Request,
  cache: Kept,
  fetcher: Fetch,
  later: (work: Promise<unknown>) => void,
): Promise<Response> | null {
  if (request.method !== 'GET') return null
  if (request.mode === 'navigate') return page(request, cache, fetcher, later)
  const path = new URL(request.url).pathname
  if (path === '/api/state') return read(request, cache, fetcher)
  if (path.startsWith('/assets/')) return asset(request, cache, fetcher)
  return null
}

/**
 * Keeps `response` as the page and the files it loads, fetching only those not
 * kept already, and drops the files an older page loaded. Vite names each
 * file by its hash, so one kept is never stale. A response that is not ok
 * keeps nothing: a 502 mid-deploy names no files, and keeping it would drop
 * every one the working page loads.
 */
export async function keep(
  cache: Kept,
  fetcher: Fetch,
  response: Response,
): Promise<void> {
  if (!response.ok) return
  const html = await response.clone().text()
  const loads = [...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map(
    (m) => m[1] ?? '',
  )
  await Promise.all(
    loads.map(async (path) => {
      if (await cache.match(path)) return
      const file = await fetcher(path)
      if (file.ok) await cache.put(path, file)
    }),
  )
  await cache.put(PAGE, response)
  for (const kept of await cache.keys()) {
    const path = new URL(kept.url).pathname
    if (path.startsWith('/assets/') && !loads.includes(path)) {
      await cache.delete(kept)
    }
  }
}

/** The page fresh when the server answers, kept when it does not. */
async function page(
  request: Request,
  cache: Kept,
  fetcher: Fetch,
  later: (work: Promise<unknown>) => void,
): Promise<Response> {
  try {
    const response = await fetcher(request)
    if (response.ok) later(keep(cache, fetcher, response.clone()))
    return response
  } catch (caught) {
    const kept = await cache.match(PAGE)
    if (kept) return kept
    throw caught
  }
}

/**
 * A board read fresh when the server answers, and kept when it is a whole
 * board; a 304 or a refusal is handed on and keeps nothing. When the server
 * cannot be reached it is the last board kept under the same query, marked
 * with `OFFLINE` so the page knows the board is not live.
 *
 * A read whose tag is not the kept board's is asked without one. The page's
 * first read is made before the worker takes the page, so otherwise every read
 * the worker saw would be a 304 and no board would ever be kept.
 */
async function read(
  request: Request,
  cache: Kept,
  fetcher: Fetch,
): Promise<Response> {
  const kept = await cache.match(request.url)
  const tag = request.headers.get('If-None-Match')
  const asked =
    tag && tag !== kept?.headers.get('ETag')
      ? new Request(request.url, { signal: request.signal })
      : request
  try {
    const response = await fetcher(asked)
    if (response.status === 200) await cache.put(request.url, response.clone())
    return response
  } catch (caught) {
    if (!kept) throw caught
    const headers = new Headers(kept.headers)
    headers.set(OFFLINE, '1')
    return new Response(kept.body, { status: kept.status, headers })
  }
}

/** A file the page loads, from the cache before the network. */
async function asset(
  request: Request,
  cache: Kept,
  fetcher: Fetch,
): Promise<Response> {
  const kept = await cache.match(request)
  if (kept) return kept
  const response = await fetcher(request)
  if (response.ok) await cache.put(request, response.clone())
  return response
}
