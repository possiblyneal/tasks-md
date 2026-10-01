import { type DragEvent, useRef, useState } from 'react'

import { sentence } from './api'
import { Due } from './Due'
import { useHold } from './hold'
import { MoveTo } from './MoveTo'
import { Narrow } from './Narrow'
import { Panel } from './Panel'
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
import { moveTask } from './write'

/** The Task a panel is open on. An id is unique only within its Repo. */
type Opened = { id: string; repo: string }

/**
 * The board: six State lanes over every Repo, a card per leaf Task. It draws
 * what the read returned and works nothing out for itself: a Task's State,
 * whether it is Blocked, its parents' titles and its order all arrive with it.
 *
 * Two reads are polled. The wide one draws the chips and the panel, which
 * need every Repo, Tag and parent whatever the lanes are narrowed to; the
 * narrowed one draws the lanes, and is not made while nothing is narrowed.
 *
 * The one write it makes is a move: a card dragged to another lane at a desk,
 * or held on a phone and sent through Move to. The next poll draws where it
 * landed. The other writes come with #122, and the box and the breakdown come
 * back with #123.
 */
export function App() {
  const [narrowing, setNarrowing] = useState(WIDE)
  const [opened, setOpened] = useState<Opened | null>(linked)
  const [refusal, setRefusal] = useState<string | null>(null)
  const [moving, setMoving] = useState<Placed | null>(null)
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
        <Narrow
          narrowing={narrowing}
          wide={wide.board}
          onChange={setNarrowing}
        />
      )}
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
                />
              ))}
            </section>
          )
        })}
      </div>
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
      {found && wide.board && (
        <Panel
          repo={found.repo}
          task={found.task}
          today={wide.board.today}
          onOpen={(id) => open({ id, repo: found.repo.name })}
          onClose={() => open(null)}
        />
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
}: {
  repo: Repo
  task: Task
  today: string
  movable: boolean
  onOpen: () => void
  onHold: () => void
  onDragStart: (event: DragEvent) => void
}) {
  const deadline = attr(task, 'deadline')
  const estimate = attr(task, 'estimate')
  const hold = useHold(onHold)
  return (
    <article
      className="card"
      draggable={DESK && movable}
      onDragStart={onDragStart}
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
