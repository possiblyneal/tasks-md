import { type DragEvent, useEffect, useRef, useState } from 'react'

import { sentence } from './api'
import { useHold } from './hold'
import { MoveTo } from './MoveTo'
import {
  type Board,
  fetchState,
  named,
  type Repo,
  type State,
  STATES,
  type Task,
  WIDE,
} from './state'
import { moveTask } from './write'

// A Repo's tasks.md changing is what says something happened, so this polls
// once a second over HTTP, and the ETag, hashed over the files' modification
// times, is what keeps that to a 304 while nothing changes.
const POLL_MS = 1000

/**
 * The board: six State lanes over every Repo, a card per leaf Task. It draws
 * what the read returned and works nothing out for itself: a Task's State,
 * whether it is Blocked and its parents' titles all arrive with it.
 *
 * The one write it makes is a move: a card dragged to another lane at a desk,
 * or held on a phone and sent through Move to. The next poll draws where it
 * landed. The other writes come with #122, and the box and the breakdown come
 * back with #123.
 */
export function App() {
  const [board, setBoard] = useState<Board | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [refusal, setRefusal] = useState<string | null>(null)
  const [moving, setMoving] = useState<Placed | null>(null)
  // The card a drag picked up. It is kept here rather than in the drag's own
  // data, which a browser hands back only on the drop.
  const dragged = useRef<Placed | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    let etag: string | null = null

    const poll = async () => {
      try {
        const snapshot = await fetchState(WIDE, etag, controller.signal)
        if (controller.signal.aborted) return
        // A null snapshot is 304: nothing changed, so nothing is redrawn.
        if (snapshot) {
          etag = snapshot.etag
          setBoard(snapshot.board)
        }
        setError(null)
      } catch (caught) {
        if (controller.signal.aborted) return
        // The tag described a response this client may no longer hold, so the
        // next poll asks for the whole thing rather than a 304 against a board
        // that failed to draw.
        etag = null
        setError(sentence(caught))
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
  }, [])

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

  const flagged = board?.repos.filter((repo) => repo.problems.length > 0) ?? []
  const marked = board?.repos.filter((repo) => repo.flags.length > 0) ?? []

  return (
    <main className="board">
      {/* A failed poll takes nothing off the board: the sentence goes over
          the cards it interrupted. */}
      {error && <p className="message">{error}</p>}
      {refusal && <p className="message">{refusal}</p>}
      {board?.errors.map((said) => (
        <p className="message" key={said}>
          {said}
        </p>
      ))}
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
          const cards = (board?.repos ?? []).flatMap((repo) =>
            repo.tasks
              .filter((task) => task.leaf && task.state === state)
              .map((task) => ({ repo, task })),
          )
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
      {moving && (
        <MoveTo
          title={moving.task.title}
          from={moving.task.state}
          onMove={(to) => void move(moving, to)}
          onCancel={() => setMoving(null)}
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

/**
 * One leaf Task. A parent is not a card of its own: its title is drawn above
 * each leaf under it, which is how a nested Task is placed on a flat lane.
 */
function Card({
  repo,
  task,
  onHold,
  onDragStart,
}: {
  repo: Repo
  task: Task
  onHold: () => void
  onDragStart: (event: DragEvent) => void
}) {
  const deadline = attr(task, 'deadline')
  const estimate = attr(task, 'estimate')
  const hold = useHold(onHold)
  return (
    <article
      className="card"
      draggable={DESK}
      onDragStart={onDragStart}
      {...hold}
      // The Repo's own color, as its preamble names it. A word CSS cannot read
      // draws no edge rather than a wrong one.
      style={repo.color ? { borderInlineStartColor: repo.color } : undefined}
    >
      {task.parents.length > 0 && (
        <small className="parents">{`${task.parents.join(' › ')} ›`}</small>
      )}
      <span className="card-title">{task.title}</span>
      <span className="facts">
        <b className="repo">{repo.name}</b>
        {task.tags.map((tag) => (
          <span key={tag}>#{tag}</span>
        ))}
        {deadline && <span>due {deadline}</span>}
        {estimate && <span>{estimate}</span>}
        {task.blocked && <span className="blocked">blocked</span>}
      </span>
    </article>
  )
}

function attr(task: Task, label: string): string | undefined {
  return task.attrs.find((a) => a.label === label)?.value
}
