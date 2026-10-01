import { useState } from 'react'

import { Due } from './Due'
import { Narrow } from './Narrow'
import { Panel } from './Panel'
import { usePoll } from './poll'
import { queryString, type Repo, STATES, type Task, WIDE } from './state'

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
 */
export function App() {
  const [narrowing, setNarrowing] = useState(WIDE)
  const [opened, setOpened] = useState<Opened | null>(linked)
  const wide = usePoll(WIDE)
  const narrowed = usePoll(queryString(narrowing) ? narrowing : null)
  const board = queryString(narrowing) ? narrowed.board : wide.board
  const error = wide.error ?? narrowed.error

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

  const flagged =
    wide.board?.repos.filter((repo) => repo.problems.length > 0) ?? []

  return (
    <main className="board">
      {/* A failed poll takes nothing off the board: the sentence goes over
          the cards it interrupted. */}
      {error && <p className="message">{error}</p>}
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
      <div className="lanes">
        {STATES.map((state) => {
          const cards = (board?.repos ?? [])
            .flatMap((repo) =>
              repo.tasks
                .filter((task) => task.leaf && task.state === state)
                .map((task) => ({ repo, task })),
            )
            .sort((a, b) => a.task.rank - b.task.rank)
          const name = state[0]?.toUpperCase() + state.slice(1)
          return (
            <section className="lane" aria-label={name} key={state}>
              <h2>
                {name} <small>{cards.length}</small>
              </h2>
              {cards.map(({ repo, task }) => (
                <Card
                  key={`${repo.name}:${task.line}`}
                  repo={repo}
                  task={task}
                  today={board?.today ?? ''}
                  onOpen={() => open({ id: task.id, repo: repo.name })}
                />
              ))}
            </section>
          )
        })}
      </div>
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
  onOpen,
}: {
  repo: Repo
  task: Task
  today: string
  onOpen: () => void
}) {
  const deadline = attr(task, 'deadline')
  const estimate = attr(task, 'estimate')
  return (
    <article
      className="card"
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
