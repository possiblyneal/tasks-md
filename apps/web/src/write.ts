// The calls that write. `moveTask`, `addTask`, `editTask` and `deleteTask` are
// the board's; `addSubtasks` is the breakdown's. `capture`, `ask` and
// `breakdown` are the Broker's, and write nothing.

import { send } from './api'
import { type Narrowing, queryString, type State, type Task } from './state'

/**
 * Puts a Task in another State. The API holds every rule a move answers to,
 * such as a Task already in Doing being taken, and refuses in its own words.
 */
export async function moveTask(
  repo: string,
  id: string,
  state: State,
): Promise<void> {
  await send<{ id: string }>(
    'POST',
    `/api/tasks/${encodeURIComponent(id)}/move`,
    { repo, state },
  )
}

/**
 * A Task as the form holds it, and as `POST /api/tasks` takes one: every
 * attribute the form shows, as text, with the Repo it is written to. An empty
 * value is an attribute the Task does not carry.
 */
export type Draft = {
  repo: string
  title: string
  state: State
  tags: string[]
  description: string
  why: string
  acceptance: string
  deadline: string
  priority: string
  impact: string
  estimate: string
  color: string
  blockedBy: string[]
  reason: string
  until: string
  /** The Task's `attachment:` lines, one pointer each. */
  attach: string[]
}

/** A Draft carrying nothing, in the Repo named and the Inbox. */
export function blank(repo: string): Draft {
  return {
    repo,
    title: '',
    state: 'inbox',
    tags: [],
    description: '',
    why: '',
    acceptance: '',
    deadline: '',
    priority: '',
    impact: '',
    estimate: '',
    color: '',
    blockedBy: [],
    reason: '',
    until: '',
    attach: [],
  }
}

/** A Task as the form opens on it, read off the attributes the read carries. */
export function draftOf(repo: string, task: Task): Draft {
  const one = (label: string) =>
    task.attrs.find((a) => a.label === label)?.value ?? ''
  return {
    repo,
    title: task.title,
    state: task.state,
    tags: task.tags,
    description: task.description,
    why: one('why'),
    acceptance: one('acceptance'),
    deadline: one('deadline'),
    priority: one('priority'),
    impact: one('impact'),
    estimate: one('estimate'),
    color: one('color'),
    blockedBy: one('blocked by')
      .split(',')
      .map((id) => id.trim())
      .filter(Boolean),
    reason: one('reason'),
    until: one('until'),
    attach: task.attrs
      .filter((a) => a.label === 'attachment')
      .map((a) => a.value),
  }
}

/** Writes one Task and names the Task it wrote. */
export async function addTask(draft: Draft): Promise<string> {
  const written = await send<{ id: string }>('POST', '/api/tasks', draft)
  return written.id
}

/** The text attributes an edit compares one by one. */
const SAID = [
  'title',
  'description',
  'why',
  'acceptance',
  'deadline',
  'priority',
  'impact',
  'estimate',
  'color',
  'reason',
  'until',
] as const

/**
 * Sends what changed between the Task as the form opened on it and the form
 * now. An attribute left as it was is left out, so an edit to one never
 * rewrites another that somebody else changed meanwhile, nor one the form does
 * not show. Tags and Blocked by go whole when they changed; attachments go as
 * the pointers added and the pointers taken off. `version` is the Task's tree
 * as it was read, and the API refuses the edit when that tree has changed.
 */
export async function editTask(
  id: string,
  version: string,
  before: Draft,
  after: Draft,
): Promise<void> {
  const body: Record<string, unknown> = { repo: after.repo, version }
  for (const name of SAID) {
    if (after[name] !== before[name]) body[name] = after[name]
  }
  for (const name of ['tags', 'blockedBy'] as const) {
    if (after[name].join(' ') !== before[name].join(' ')) {
      body[name] = after[name]
    }
  }
  const attach = after.attach.filter((p) => !before.attach.includes(p))
  const detach = before.attach.filter((p) => !after.attach.includes(p))
  if (attach.length > 0) body.attach = attach
  if (detach.length > 0) body.detach = detach
  await send<{ id: string }>(
    'POST',
    `/api/tasks/${encodeURIComponent(id)}`,
    body,
  )
}

/**
 * Takes the Task out of its Repo's file, its Subtasks with it, refused when
 * its tree has changed since `version` was read.
 */
export async function deleteTask(
  repo: string,
  id: string,
  version: string,
): Promise<void> {
  await send<{ id: string }>(
    'POST',
    `/api/tasks/${encodeURIComponent(id)}/delete`,
    { repo, version },
  )
}

/**
 * Hands a dump to the Broker and gets back the Task it read, filled into the
 * body the add sheet submits. It writes nothing: a dump read and then
 * abandoned leaves nothing behind, so the sheet is the gate.
 */
export function capture(text: string): Promise<Partial<Draft>> {
  return send<Partial<Draft>>('POST', '/api/capture', { text })
}

/**
 * Asks about the Tasks in view and gets prose back. It is a read like the list
 * it is about: nothing is appended and nothing is kept between calls.
 *
 * "In view" is the narrowing the list is drawn under, sent as the same query
 * string the poll carries and read by the route the same way. A question asked
 * without it would be answered about every open Task while the screen shows a
 * sorted, narrowed few, and nothing on the screen would say so.
 */
export async function ask(
  question: string,
  narrowing: Narrowing,
): Promise<string> {
  const said = await send<{ answer: string }>(
    'POST',
    `/api/ask${queryString(narrowing)}`,
    { question },
  )
  return said.answer
}

/**
 * Writes the approved proposals under the parent as one write, refused when
 * the parent's tree has changed since `version` was read. Names what it wrote.
 */
export async function addSubtasks(
  repo: string,
  parent: string,
  version: string,
  subtasks: Proposal[],
): Promise<string[]> {
  const written = await send<{ ids: string[] }>(
    'POST',
    `/api/tasks/${encodeURIComponent(parent)}/subtasks`,
    { repo, version, subtasks },
  )
  return written.ids
}

/** One question the Broker asked and the answer it was given back. */
export type QA = { question: string; answer: string }

/**
 * One Subtask the Broker proposes. Exactly the six `ai.Proposal` carries in
 * `apps/tasks/src/ai/ai.go` and no more, with what the store would refuse
 * already dropped by the route: a proposal is approved on what was
 * drawn beside its tick, so an attribute this type admits is one the screen
 * has to draw. `Draft` is wider and is what the approval goes out as, and
 * typing a proposal as one would let a deadline nobody saw be written by a
 * route that never answers one.
 */
export type Proposal = Partial<
  Pick<
    Draft,
    'title' | 'description' | 'why' | 'estimate' | 'priority' | 'impact'
  >
>

/**
 * One turn coming back: what the Broker still needs to know, or what it
 * proposes. The two are alternatives.
 */
export type Step = {
  questions?: string[]
  proposals: Proposal[]
}

/**
 * One turn of a breakdown. It writes nothing: the Broker keeps nothing between
 * calls, so every turn carries everything already answered, and the approved
 * proposals are written afterwards by `addSubtasks`.
 */
export function breakdown(
  repo: string,
  task: string,
  answers: QA[],
): Promise<Step> {
  return send<Step>('POST', '/api/breakdown', { repo, task, answers })
}
