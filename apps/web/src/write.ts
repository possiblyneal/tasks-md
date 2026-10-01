// The wire shapes the write and Broker routes took, and the calls that reach
// them. No route answers any of them now: writes onto tasks.md land in #121
// and #122, and the Broker's routes come back with #123, which mounts `Box`
// and `Breakdown` again. They are kept so that work starts from them.

import { send, sentence } from './api'
import { type Narrowing, queryString } from './state'

/**
 * A Task's attributes and its memberships as they are sent, and the same shape
 * `POST /api/capture` answers in: what the Broker read comes back as the body
 * this submits once somebody has corrected it.
 *
 * An absent attribute is left alone and an empty one is cleared, which is the
 * same rule a typed `-title ""` follows. Lists and Tags are the ids
 * `GET /api/state` already named.
 */
export type TaskBody = {
  title?: string
  description?: string
  why?: string
  color?: string
  deadline?: string
  estimate?: string
  priority?: string
  impact?: string
  /**
   * How long to hide the Task for, which the API reads from now: one of the
   * offered labels or a plain duration. It is the one attribute a Task cannot
   * be read back into, since what it carries is the instant it wakes rather
   * than the span somebody asked for, so an absent one leaves the Task as it
   * is and an empty one wakes it.
   */
  snooze?: string
  /**
   * The key/value pairs, sent whole. A key mapped to the empty string removes
   * it and a key left out is left alone, which is the store's rule rather than
   * a second one written here.
   */
  fields?: Record<string, string>
  parent?: string
  /**
   * Pointers to attach once the Task exists. They never travel in the body: no
   * write route takes an attachment and every one of them refuses a field it
   * does not know, so `addTask`, `addSubtask`, `editTask` and `detachEdited`
   * each split them off and attach them one at a time afterwards.
   *
   * Nothing here detaches. A pointer already on the Task is taken off from the
   * detail screen, which is the only screen that can show what is there.
   */
  attachments?: string[]
  intoLists?: string[]
  outOfLists?: string[]
  addTags?: string[]
  dropTags?: string[]
}

/**
 * Hands a dump to the Broker and gets back the Task it read, filled into the
 * body the add sheet submits. It writes nothing: a dump read and then
 * abandoned leaves nothing behind, so the sheet is the gate.
 */
export function capture(text: string): Promise<TaskBody> {
  return send<TaskBody>('POST', '/api/capture', { text })
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
 * Splits the pointers off a body. The routes refuse a field they do not know,
 * and an Attachment is a write of its own against a Task that has to exist
 * first, so this is where the two part company.
 */
function unattached(body: TaskBody): [TaskBody, string[]] {
  const { attachments, ...rest } = body
  return [rest, attachments ?? []]
}

/**
 * Attaches each pointer to a Task that now exists. One at a time and in order,
 * because each is its own guarded write; a refusal on one stops there, with the
 * Task and whatever was attached before it left standing.
 *
 * The sentence says the Task was written, because the form that sent it cannot
 * tell otherwise: every other way a submit fails leaves nothing behind, and a
 * second press of the same button after this one would write a second Task
 * rather than retry the pointer. What did not land is added from the Task
 * itself, which is the screen that can show what is already on it.
 */
async function attachAll(id: string, pointers: string[]): Promise<void> {
  for (const [landed, pointer] of pointers.entries()) {
    try {
      await attach(id, pointer)
    } catch (caught) {
      throw new Error(
        `the task was written, and ${landed} of ${pointers.length} attachments with it. ` +
          `${pointer} was refused: ${sentence(caught)}. ` +
          `Add the rest from the task rather than submitting again, which writes a second task.`,
        { cause: caught },
      )
    }
  }
}

/** Writes one Task and files it, and names the Task it wrote. */
export async function addTask(body: TaskBody): Promise<string> {
  const [create, pointers] = unattached(body)
  const written = await send<{ id: string }>('POST', '/api/tasks', create)
  await attachAll(written.id, pointers)
  return written.id
}

/** Writes one Task under another, which is the same write with a parent. */
export async function addSubtask(
  parent: string,
  body: TaskBody,
): Promise<string> {
  const [create, pointers] = unattached(body)
  const written = await send<{ id: string }>(
    'POST',
    `/api/tasks/${encodeURIComponent(parent)}/subtasks`,
    create,
  )
  await attachAll(written.id, pointers)
  return written.id
}

/**
 * The Lists and Tags one write joins and leaves, worked out from what the
 * Task carried when the sheet opened and what is ticked on it now.
 *
 * Ticking and unticking are different fields on the wire, so the difference has
 * to be taken somewhere: taking it here means a sheet opened on a Task and a
 * sheet opened on a draft submit the same body, and an untick is a membership
 * dropped rather than one silently left on.
 */
export function memberships(before: TaskBody, after: TaskBody): TaskBody {
  const on = (body: TaskBody, name: 'intoLists' | 'addTags') => body[name] ?? []
  const missing = (from: string[], against: string[]) =>
    from.filter((id) => !against.includes(id))
  return {
    intoLists: missing(on(after, 'intoLists'), on(before, 'intoLists')),
    outOfLists: missing(on(before, 'intoLists'), on(after, 'intoLists')),
    addTags: missing(on(after, 'addTags'), on(before, 'addTags')),
    dropTags: missing(on(before, 'addTags'), on(after, 'addTags')),
  }
}

/** One question the Broker asked and the answer it was given back. */
export type QA = { question: string; answer: string }

/**
 * One Subtask the Broker proposes. Exactly the six `ai.Proposal` carries in
 * `apps/tasks/src/ai/ai.go` and no more: a proposal is approved on what was
 * drawn beside its tick, so an attribute this type admits is one the screen
 * has to draw. `TaskBody` is wider and is what the approval goes out as, and
 * typing a proposal as one would let a deadline nobody saw be written by a
 * route that never answers one.
 */
export type Proposal = Pick<
  TaskBody,
  'title' | 'description' | 'why' | 'estimate' | 'priority' | 'impact'
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
 * One turn of a breakdown. It writes nothing and holds no Lease: the Broker
 * keeps nothing between calls, so every turn carries everything already
 * answered, and an approved proposal is written afterwards by `addSubtask`
 * like any other Subtask.
 */
export function breakdown(task: string, answers: QA[]): Promise<Step> {
  return send<Step>('POST', '/api/breakdown', { task, answers })
}

/**
 * A List or a Tag as `api.collectionBody` takes one. Both attributes follow the
 * same rule every attribute of a Task follows: absent leaves it alone, which is
 * what makes a rename and a recolor one body rather than two writes.
 */
export type CollectionBody = {
  name?: string
  color?: string
}

/**
 * Which of the two a write is about, as the path spells it. A List and a Tag
 * are the same three writes against different aggregates, so the calls are
 * written once and given the segment, the way `api.kind` hands the routes a
 * pair of store calls rather than writing each route twice.
 */
export type Kind = 'lists' | 'tags'

/** Writes one List or Tag and names it. */
export async function addCollection(
  kind: Kind,
  body: CollectionBody,
): Promise<string> {
  const written = await send<{ id: string }>('POST', `/api/${kind}`, body)
  return written.id
}

/**
 * Points a Task at something outside the tracker: a web address, or a path as
 * the machine running `tasks api` would read it. Nothing is uploaded and nothing
 * is copied -- an Attachment is the text and nothing else, so a pointer typed
 * on a phone naming a file on that phone points nowhere anybody can follow.
 */
export async function attach(id: string, target: string): Promise<void> {
  await send<{ id: string }>(
    'POST',
    `/api/tasks/${encodeURIComponent(id)}/attachments`,
    { target },
  )
}
