// What the board's tests share: a Task with every field the read carries, a
// board of two Repos, and a fetch that answers `GET /api/state` and records
// what it was asked. No test file imports another, so this is where they meet.

import type { Board, Task, Week } from './state'

export function task(
  fields: Partial<Task> & Pick<Task, 'title' | 'state'>,
): Task {
  return {
    id: '',
    tags: [],
    created: '2026-09-20',
    attrs: [],
    description: '',
    line: 1,
    depth: 0,
    parent: '',
    parents: [],
    leaf: true,
    blocked: false,
    rank: 0,
    version: 'v0',
    ...fields,
  }
}

/** Twelve weeks to 2026-09-28, the ones named filled in. */
export function weeks(counted: Record<string, Omit<Week, 'start'>>): Week[] {
  return Array.from({ length: 12 }, (_, i) => {
    const start = new Date(Date.UTC(2026, 8, 28 - 7 * (11 - i)))
      .toISOString()
      .slice(0, 10)
    return { start, added: 0, done: 0, declined: 0, ...counted[start] }
  })
}

export const BOARD: Board = {
  repos: [
    {
      name: 'house-move',
      path: '/home/n/code/house-move',
      color: 'green',
      problems: [],
      flags: [],
      tasks: [
        task({
          id: 'm3qa',
          title: 'Pack the kitchen',
          state: 'doing',
          leaf: false,
          line: 4,
        }),
        task({
          id: 'm3qc',
          title: 'Wrap glassware',
          state: 'doing',
          tags: ['fragile'],
          attrs: [{ label: 'deadline', value: '2026-10-10' }],
          description: 'Newspaper first, then the boxes marked G.',
          depth: 1,
          parent: 'm3qa',
          parents: ['Pack the kitchen'],
          line: 8,
          rank: 1,
        }),
        task({
          id: 'm3qb',
          title: 'Buy boxes',
          state: 'done',
          depth: 1,
          parent: 'm3qa',
          parents: ['Pack the kitchen'],
          line: 11,
          rank: 2,
        }),
        task({
          id: 'v9t1',
          title: 'Book the van',
          state: 'backlog',
          tags: ['van'],
          blocked: true,
          line: 14,
          rank: 3,
        }),
      ],
    },
    {
      name: 'work',
      path: '/home/n/code/work',
      color: '',
      problems: [{ line: 3, message: '"Write the report" has no id line' }],
      flags: ['not backed up'],
      tasks: [
        task({ title: 'Write the report', state: 'inbox', line: 3, rank: 4 }),
      ],
    },
  ],
  errors: [],
  sorts: ['file', 'title', 'deadline', 'created', 'priority', 'estimate'],
  today: '2026-10-01',
  metrics: weeks({
    '2026-09-21': { added: 0, done: 4, declined: 0 },
    '2026-09-28': { added: 2, done: 1, declined: 0 },
  }),
}

/** Every request the board made, so what it asked for and how is checkable. */
export const asked: { url: string; init?: RequestInit }[] = []

type Refusal = { status: number; error: string }

function answer(body: Board | Refusal | undefined, etag: string): Response {
  if (body && 'status' in body) {
    return new Response(JSON.stringify({ error: body.error }), {
      status: body.status,
    })
  }
  return new Response(JSON.stringify(body), { headers: { ETag: etag } })
}

/** Answers each request with the next body, the last one over and over. */
export function answering(...bodies: (Board | Refusal)[]) {
  return (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    return Promise.resolve(
      answer(bodies[Math.min(asked.length, bodies.length) - 1], '"1"'),
    )
  }
}

/**
 * Answers by the query a request carries: the board under that query, or
 * BOARD for one not listed, the bare path included.
 */
export function answeringBy(boards: Record<string, Board>) {
  return (url: string, init?: RequestInit) => {
    asked.push({ url, init })
    const query = url.slice('/api/state'.length)
    return Promise.resolve(answer(boards[query] ?? BOARD, `"${query}"`))
  }
}
