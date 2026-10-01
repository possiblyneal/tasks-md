// The board's reads. A Repo's TASKS.md changing is what says something
// happened, so a read is polled once a second over HTTP, and the ETag, hashed
// over the files' modification times, keeps that to a 304 while nothing
// changes.

import { useEffect, useState } from 'react'

import { sentence } from './api'
import { type Board, fetchState, type Narrowing, queryString } from './state'

const POLL_MS = 1000

/**
 * Polls `GET /api/state` under one Narrowing, or nothing for `null`. The
 * Narrowing is compared by identity, so a caller keeps it in state rather than
 * building one each render.
 *
 * A board is handed back only while it answers the Narrowing asked for now,
 * so a lane never draws one narrowing's Tasks under another's chips while the
 * new read is on its way.
 */
export function usePoll(narrowing: Narrowing | null): {
  board: Board | null
  error: string | null
  offline: boolean
} {
  const [read, setRead] = useState<{ query: string; board: Board } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [offline, setOffline] = useState(false)

  useEffect(() => {
    if (!narrowing) return
    const query = queryString(narrowing)
    const controller = new AbortController()
    let etag: string | null = null

    const poll = async () => {
      try {
        const snapshot = await fetchState(narrowing, etag, controller.signal)
        if (controller.signal.aborted) return
        // A null snapshot is 304: nothing changed, so nothing is redrawn.
        if (snapshot) {
          etag = snapshot.etag
          setRead({ query, board: snapshot.board })
        }
        // A 304 came from the server, so it is as live as a whole board is.
        setOffline(snapshot?.offline ?? false)
        setError(null)
      } catch (caught) {
        if (controller.signal.aborted) return
        // The tag described a response this client may no longer hold, so the
        // next poll asks for the whole thing rather than a 304 against a board
        // that failed to draw.
        etag = null
        setError(sentence(caught))
        // A fetch rejects with a TypeError only when the request never reached
        // a server, which with no board kept is what away from home looks like.
        setOffline(caught instanceof TypeError)
      }
    }

    // Each poll is scheduled once the one before it has settled rather than
    // on an interval, so two are never in flight together: the slower of an
    // overlapping pair would draw its older board over the newer one.
    let timer: ReturnType<typeof setTimeout> | undefined
    const tick = async () => {
      await poll()
      if (!controller.signal.aborted) {
        timer = setTimeout(() => void tick(), POLL_MS)
      }
    }

    void tick()
    return () => {
      controller.abort()
      clearTimeout(timer)
    }
  }, [narrowing])

  const current =
    narrowing && read?.query === queryString(narrowing) ? read.board : null
  return {
    board: current,
    error: narrowing ? error : null,
    offline: narrowing ? offline : false,
  }
}
