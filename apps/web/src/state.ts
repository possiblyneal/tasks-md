// The wire shapes `GET /api/state` answers with, and the one call that reads
// it. They mirror `apps/tasks/src/board/board.go`, which is the side that
// decides them: `tasks list -json` prints the same shape.

import { refused } from './api'

/** One of the six, as the word on a title line writes it. */
export type State =
  'inbox' | 'backlog' | 'doing' | 'deferred' | 'done' | 'declined'

/** The six, in the order the board draws its lanes. */
export const STATES: State[] = [
  'inbox',
  'backlog',
  'doing',
  'deferred',
  'done',
  'declined',
]

/** A State as a lane's heading and a button say it. */
export function named(state: State): string {
  return state[0]?.toUpperCase() + state.slice(1)
}

/** One `- label: value` line under a title, other than `id` and `created`. */
export type Attr = { label: string; value: string }

/**
 * One Task as the read gives it: flat, in file order, carrying where it sits
 * in its tree and what the read worked out. `state` is the State that counts,
 * the checkbox over the word and a parent's worked out from its Subtasks, so
 * the client draws it and never works it out again.
 */
export type Task = {
  id: string
  title: string
  state: State
  tags: string[]
  created: string
  attrs: Attr[]
  description: string
  line: number
  depth: number
  parent: string
  /** Every title above this Task, outermost first. */
  parents: string[]
  leaf: boolean
  blocked: boolean
  /**
   * Its top-level tree's version. An edit or a delete hands it back, and the
   * API refuses the write when the tree has changed since it was read.
   */
  version: string
  /**
   * This Task's place in the whole read's order, across every Repo: by the
   * sort asked for, then Repo, then place in the file. A lane holds several
   * Repos' cards and draws them by it.
   */
  rank: number
}

/** One thing wrong with a Repo's tasks.md, at the line it is on. */
export type Problem = { line: number; message: string }

/** One Repo's read. Any problem flags the Repo wherever it is drawn. */
export type Repo = {
  name: string
  path: string
  color: string
  tasks: Task[]
  problems: Problem[]
  /**
   * What its tasks history is marked with: `not pushed`, `not backed up`, or
   * the sentence saying why it refuses writes.
   */
  flags: string[]
}

/**
 * A whole read: every Repo, what was wrong with the config, the sorts a read
 * can be ordered by, and the host's local date as `YYYY-MM-DD`, which is the
 * day a Deadline is today or Overdue against.
 */
export type Board = {
  repos: Repo[]
  errors: string[]
  sorts: string[]
  today: string
}

/**
 * The narrowings `GET /api/state` accepts, under the names `tasks list` takes
 * its flags under. Several States or several Tags widen within the field: a
 * Task in any one of them is in.
 */
export type Narrowing = {
  repo: string
  states: State[]
  tags: string[]
  search: string
  unblocked: boolean
  /** One of the served `sorts`; empty is file order. */
  sort: string
}

/** Narrowed by nothing: every Task in every Repo. */
export const WIDE: Narrowing = {
  repo: '',
  states: [],
  tags: [],
  search: '',
  unblocked: false,
  sort: '',
}

/**
 * A Narrowing as the query string the read carries. An empty value is left
 * out rather than sent empty, so the wide view is the bare path. A set is the
 * parameter repeated, which is how the route reads it. File order is what
 * the read gives with no sort named, so it is left out the same way.
 */
export function queryString({
  repo,
  states,
  tags,
  search,
  unblocked,
  sort,
}: Narrowing): string {
  const query = new URLSearchParams()
  if (repo) query.set('repo', repo)
  for (const state of states) query.append('state', state)
  for (const tag of tags) query.append('tag', tag)
  if (search) query.set('search', search)
  if (unblocked) query.set('unblocked', 'true')
  if (sort && sort !== 'file') query.set('sort', sort)
  const written = query.toString()
  return written ? `?${written}` : ''
}

export type Snapshot = {
  etag: string | null
  board: Board
}

/**
 * Reads the whole board in one request. The ETag is handed back on the next
 * call so files that have not changed answer 304 with no body, which is what
 * makes polling once a second cheap.
 *
 * A `null` return is "nothing changed", which is not the same as an empty
 * board and must not redraw as one.
 */
export async function fetchState(
  narrowing: Narrowing,
  etag: string | null,
  signal?: AbortSignal,
): Promise<Snapshot | null> {
  const response = await fetch(`/api/state${queryString(narrowing)}`, {
    headers: etag ? { 'If-None-Match': etag } : {},
    signal,
  })

  if (response.status === 304) return null

  // The API's error body is the sentence the CLI would have printed, so it is
  // shown as it is rather than restated in the client's own words.
  if (!response.ok) throw await refused(response)

  return {
    etag: response.headers.get('ETag'),
    board: (await response.json()) as Board,
  }
}

// The values the form offers, mirroring `apps/tasks/src/write/write.go`,
// which refuses any other.

/** Priority and Impact. */
export const LEVELS = ['low', 'med', 'high']

/** An Estimate, each with the letter the form shows it under. */
export const ESTIMATES = [
  { value: 'small', label: 'S' },
  { value: 'medium', label: 'M' },
  { value: 'large', label: 'L' },
]

export const COLORS = [
  'red',
  'orange',
  'yellow',
  'green',
  'cyan',
  'blue',
  'violet',
  'magenta',
  'brown',
]

/**
 * One directory on the machine `tasks api` runs on, as `GET /api/files` lists
 * it. `apps/tasks/src/api/files.go` is the side that decides the shape.
 *
 * It is a read and nothing else: the route lists names and never opens a file,
 * because an Attachment is a pointer and the tracker holds no copy of what it
 * points at. `parent` is empty at the root the listener will look no further
 * up than.
 */
export type Files = {
  path: string
  parent: string
  entries: Listed[]
}

/**
 * One name in it. Whether it is a directory is the whole of what a picker
 * needs: one is somewhere to go and the other is something to point at.
 */
export type Listed = {
  name: string
  dir: boolean
}

/** Lists one directory, or the listener's root where none is named. */
export async function fetchFiles(path?: string): Promise<Files> {
  const at = path === undefined ? '' : `?path=${encodeURIComponent(path)}`
  const response = await fetch(`/api/files${at}`)
  if (!response.ok) throw await refused(response)
  return (await response.json()) as Files
}
