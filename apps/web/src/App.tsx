import { type DragEvent, useRef, useState } from 'react'

import { sentence } from './api'
import { Box } from './Box'
import { Breakdown } from './Breakdown'
import { Due } from './Due'
import { useHold } from './hold'
import { Metrics } from './Metrics'
import { MoveTo } from './MoveTo'
import { Narrow } from './Narrow'
import { Panel } from './Panel'
import { Sheet } from './Sheet'
import { usePoll } from './poll'
import {
  named,
  queryString,
  type Repo,
  type State,
  STATES,
  type Task,
  WIDE,
} from './state'
import {
  addTask,
  blank,
  deleteTask,
  type Draft,
  draftOf,
  editTask,
  moveTask,
} from './write'

/** The Task a panel is open on. An id is unique only within its Repo. */
type Opened = { id: string; repo: string }

/**
 * What the form is open on: a new Task, or one being edited, with its id, its
 * tree's version and the Task itself as they were read when the form opened.
 * An edit sends what differs from the Task read, which is not the draft when a
 * dump amended it.
 */
type Form = { draft: Draft } & (
  { id: string; version: string; was: Draft } | { id?: never; dumped?: boolean }
)

/**
 * The board: six State lanes over every Repo, a card per leaf Task. It draws
 * what the read returned and works nothing out for itself: a Task's State,
 * whether it is Blocked, its parents' titles and its order all arrive with it.
 *
 * Two reads are polled. The wide one draws the chips and the panel, which
 * need every Repo, Tag and parent whatever the lanes are narrowed to; the
 * narrowed one draws the lanes, and is not made while nothing is narrowed.
 *
 * A move is a card dragged to another lane at a desk, or held on a phone and
 * sent through Move to. Add opens the form on a new Task, and a panel's Edit
 * opens it on that Task. The next poll draws what any write did.
 *
 * The box under the board opens the same form on what the Broker read out of
 * a dump, and the box in a panel opens it on that Task as a dump amended it.
 * A panel's Break down opens the breakdown in its place. None of them writes
 * before somebody submits.
 */
export function App() {
  const [narrowing, setNarrowing] = useState(WIDE)
  const [opened, setOpened] = useState<Opened | null>(linked)
  const [refusal, setRefusal] = useState<string | null>(null)
  const [moving, setMoving] = useState<Placed | null>(null)
  const [form, setForm] = useState<Form | null>(null)
  const [breaking, setBreaking] = useState(false)
  // Bumped once a dump's Task is added, which empties the box. A dump
  // abandoned in the form keeps its words there to be read again.
  const [said, setSaid] = useState(0)
  // The card a drag picked up. It is kept here rather than in the drag's own
  // data, which a browser hands back only on the drop.
  const dragged = useRef<Placed | null>(null)
  const wide = usePoll(WIDE)
  const narrowed = usePoll(queryString(narrowing) ? narrowing : null)
  const board = queryString(narrowing) ? narrowed.board : wide.board
  const error = wide.error ?? narrowed.error
  // Away from home the board is the last one read, and nothing on it writes.
  const offline = wide.offline || narrowed.offline

  // The address names the open panel, so a panel can be linked to and a
  // reload comes back to it.
  const open = (next: Opened | null) => {
    setOpened(next)
    setBreaking(false)
    const query = next
      ? `?${new URLSearchParams({ task: next.id, repo: next.repo })}`
      : ''
    window.history.replaceState(null, '', `${window.location.pathname}${query}`)
  }

  const found = opened && wide.board && find(wide.board.repos, opened)

  const move = async ({ repo, task }: Placed, to: State) => {
    setMoving(null)
    if (task.state === to) return
    try {
      await moveTask(repo.name, task.id, to)
      setRefusal(null)
    } catch (caught) {
      setRefusal(sentence(caught))
    }
  }

  const save = async (after: Draft) => {
    if (!form) return
    if (form.id !== undefined) {
      await editTask(form.id, form.version, form.was, after)
    } else {
      await addTask(after)
      if (form.dumped) setSaid((n) => n + 1)
    }
    setForm(null)
  }

  const drop = (to: State) => (event: DragEvent) => {
    event.preventDefault()
    const card = dragged.current
    dragged.current = null
    if (card) void move(card, to)
  }

  const flagged =
    wide.board?.repos.filter((repo) => repo.problems.length > 0) ?? []
  const marked = wide.board?.repos.filter((repo) => repo.flags.length > 0) ?? []

  return (
    <main className="board">
      {/* A failed poll takes nothing off the board: the sentence goes over
          the cards it interrupted. */}
      {error && <p className="message">{error}</p>}
      {offline && (
        <p className="message" role="status">
          Offline: this is the last board read, and nothing can be changed until
          tasks.lan answers again.
        </p>
      )}
      {refusal && <p className="message">{refusal}</p>}
      {board?.errors.map((said) => (
        <p className="message" key={said}>
          {said}
        </p>
      ))}
      {wide.board && (
        <button
          type="button"
          className="control"
          disabled={offline || wide.board.repos.length === 0}
          onClick={() =>
            setForm({ draft: blank(wide.board?.repos[0]?.name ?? '') })
          }
        >
          Add
        </button>
      )}
      {wide.board && (
        <Narrow
          narrowing={narrowing}
          wide={wide.board}
          onChange={setNarrowing}
        />
      )}
      {board && <Metrics weeks={board.metrics} />}
      {flagged.length > 0 && (
        <aside className="problems" aria-label="Problems">
          <h2>Problems</h2>
          <ul>
            {flagged.flatMap((repo) =>
              repo.problems.map((p) => (
                <li key={`${repo.name}:${p.line}:${p.message}`}>
                  {`${repo.name} line ${p.line}: ${p.message}`}
                </li>
              )),
            )}
          </ul>
        </aside>
      )}
      {marked.length > 0 && (
        <aside className="problems" aria-label="Flags">
          <h2>Flags</h2>
          <ul>
            {marked.flatMap((repo) =>
              repo.flags.map((flag) => (
                <li key={`${repo.name}:${flag}`}>{`${repo.name}: ${flag}`}</li>
              )),
            )}
          </ul>
        </aside>
      )}
      <div className="lanes">
        {STATES.map((state) => {
          const cards = (board?.repos ?? [])
            .flatMap((repo) =>
              repo.tasks
                .filter((task) => task.leaf && task.state === state)
                .map((task) => ({ repo, task })),
            )
            .sort((a, b) => a.task.rank - b.task.rank)
          const name = named(state)
          return (
            <section
              className="lane"
              aria-label={name}
              key={state}
              onDragOver={(event) => event.preventDefault()}
              onDrop={drop(state)}
            >
              <h2>
                {name} <small>{cards.length}</small>
              </h2>
              {cards.map(({ repo, task }) => (
                <Card
                  key={`${repo.name}:${task.line}`}
                  repo={repo}
                  task={task}
                  today={board?.today ?? ''}
                  movable={!offline}
                  onOpen={() => open({ id: task.id, repo: repo.name })}
                  onHold={() => setMoving({ repo, task })}
                  onDragStart={(event) => {
                    dragged.current = { repo, task }
                    event.dataTransfer.effectAllowed = 'move'
                    event.dataTransfer.setData('text/plain', task.id)
                  }}
                  // A card let go anywhere but a lane is no longer being
                  // dragged, so a later drop of text or a file moves nothing.
                  onDragEnd={() => {
                    dragged.current = null
                  }}
                />
              ))}
            </section>
          )
        })}
      </div>
      {/* Before every sheet and dialog, which are drawn over it. */}
      {wide.board && (
        <Box
          key={said}
          repos={wide.board.repos.map((repo) => repo.name)}
          narrowing={narrowing}
          offline={offline}
          onDraft={(draft) => setForm({ draft, dumped: true })}
        />
      )}
      {moving && !offline && (
        <MoveTo
          title={moving.task.title}
          from={moving.task.state}
          onMove={(to) => void move(moving, to)}
          onCancel={() => setMoving(null)}
        />
      )}
      {opened && wide.board && !found && (
        <p className="message">
          {opened.repo
            ? `No Task ${opened.id} is in ${opened.repo} on the board.`
            : `No Task ${opened.id} is on the board.`}
        </p>
      )}
      {found && wide.board && !form && breaking && (
        <>
          <div className="scrim" aria-hidden="true" />
          <div
            className="panel"
            role="dialog"
            aria-modal="true"
            aria-label="Break down the Task"
          >
            <Breakdown
              repo={found.repo.name}
              task={found.task}
              offline={offline}
              onBack={() => setBreaking(false)}
            />
          </div>
        </>
      )}
      {found && wide.board && !form && !breaking && (
        <Panel
          // Each Task opens on a fresh panel, so an armed Delete, a refusal or
          // another Task's history is never carried over to it.
          key={`${found.repo.name}/${found.task.id}`}
          repo={found.repo}
          task={found.task}
          today={wide.board.today}
          onOpen={(id) => open({ id, repo: found.repo.name })}
          onEdit={() =>
            setForm({
              draft: draftOf(found.repo.name, found.task),
              id: found.task.id,
              version: found.task.version,
              was: draftOf(found.repo.name, found.task),
            })
          }
          onAmend={(draft) =>
            setForm({
              draft,
              id: found.task.id,
              version: found.task.version,
              was: draftOf(found.repo.name, found.task),
            })
          }
          onDelete={async () => {
            await deleteTask(found.repo.name, found.task.id, found.task.version)
            open(null)
          }}
          onBreakdown={() => setBreaking(true)}
          onClose={() => open(null)}
          offline={offline}
        />
      )}
      {form && wide.board && (
        <>
          <div className="scrim" aria-hidden="true" />
          <div
            className="panel"
            role="dialog"
            aria-modal="true"
            aria-label={form.id ? 'Edit the Task' : 'Add a Task'}
          >
            <Sheet
              draft={form.draft}
              repos={wide.board.repos.map((repo) => repo.name)}
              existing={form.id !== undefined}
              offline={offline}
              action={form.id ? 'Save' : 'Add'}
              onSubmit={save}
              onCancel={() => setForm(null)}
            />
          </div>
        </>
      )}
    </main>
  )
}

/** A card as the board placed it: the Task, and the Repo it is in. */
type Placed = { repo: Repo; task: Task }

// A drag between lanes is for a mouse or a pen. On a phone a press on a card is
// a scroll or a hold, never a drag.
const DESK = window.matchMedia?.('(pointer: fine)').matches ?? true

/** The Task a link names: `?task=<id>&repo=<name>`, the Repo optional. */
function linked(): Opened | null {
  const query = new URLSearchParams(window.location.search)
  const id = query.get('task')
  return id ? { id, repo: query.get('repo') ?? '' } : null
}

/** The Task opened, in the named Repo, or the first Repo holding the id. */
function find(
  repos: Repo[],
  { id, repo: name }: Opened,
): { repo: Repo; task: Task } | null {
  for (const repo of repos) {
    if (name && repo.name !== name) continue
    const task = repo.tasks.find((t) => t.id === id)
    if (task) return { repo, task }
  }
  return null
}

/**
 * One leaf Task. A parent is not a card of its own: its title is drawn above
 * each leaf under it, which is how a nested Task is placed on a flat lane.
 */
function Card({
  repo,
  task,
  today,
  movable,
  onOpen,
  onHold,
  onDragStart,
  onDragEnd,
}: {
  repo: Repo
  task: Task
  today: string
  movable: boolean
  onOpen: () => void
  onHold: () => void
  onDragStart: (event: DragEvent) => void
  onDragEnd: () => void
}) {
  const deadline = attr(task, 'deadline')
  const estimate = attr(task, 'estimate')
  const hold = useHold(onHold)
  return (
    <article
      className="card"
      draggable={DESK && movable}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      {...(movable ? hold : {})}
      // The Repo's own color, as its preamble names it. A word CSS cannot read
      // draws no edge rather than a wrong one.
      style={repo.color ? { borderInlineStartColor: repo.color } : undefined}
    >
      {task.parents.length > 0 && (
        <small className="parents">{`${task.parents.join(' › ')} ›`}</small>
      )}
      {/* Opened by its id, so a hand-typed Task without one, which lint
          already flags, is not a link. */}
      {task.id ? (
        <button type="button" className="card-title" onClick={onOpen}>
          {task.title}
        </button>
      ) : (
        <span className="card-title">{task.title}</span>
      )}
      <span className="facts">
        <b className="repo">{repo.name}</b>
        {task.tags.map((tag) => (
          <span key={tag}>#{tag}</span>
        ))}
        {deadline && <Due on={deadline} today={today} />}
        {estimate && <span>{estimate}</span>}
        {task.blocked && <span className="blocked">blocked</span>}
      </span>
    </article>
  )
}

function attr(task: Task, label: string): string | undefined {
  return task.attrs.find((a) => a.label === label)?.value
}
