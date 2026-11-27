// PROTOTYPE — throwaway, issue #99: what the all-Repos page looks like.
// Three variants of the list route, switchable via `?variant=A|B|C`, over stub
// Tasks in memory. A = State lanes (kanban), B = nested list grouped by Repo
// (Google Tasks / tasks.md), C = one list with a State strip and Repo chips.
// The box (dump / ask) and a Task's Break down are drawn in every variant so
// the tap count to each can be compared.
import { useState } from 'react'

import {
  add,
  ENDED,
  findParent,
  findTask,
  move,
  overdue,
  PROPOSALS,
  type PState,
  type PTask,
  REPOS,
  seed,
  STATES,
} from './data'
import { Switcher } from './Switcher'
import './prototype.css'

const VARIANTS = [
  { key: 'A', name: 'State lanes' },
  { key: 'B', name: 'Nested list by Repo' },
  { key: 'C', name: 'One list, State strip' },
  { key: 'D', name: 'State lanes, Done and Declined apart' },
]

type Props = {
  tasks: PTask[]
  repo: string | null
  onOpen: (id: string) => void
  onMove: (id: string, s: PState) => void
}

export function AllRepos() {
  const [variant, setVariant] = useState(
    new URLSearchParams(location.search).get('variant') ?? 'A',
  )
  const [tasks, setTasks] = useState(seed)
  const [repo, setRepo] = useState<string | null>(null)
  const [open, setOpen] = useState<string | null>(null)
  const [last, setLast] = useState('')

  const change = (key: string) => {
    const url = new URL(location.href)
    url.searchParams.set('variant', key)
    history.replaceState(null, '', url)
    setVariant(key)
  }
  const onMove = (id: string, s: PState) => {
    setTasks((ts) => move(ts, id, s))
    setLast(`moved ${findTask(tasks, id)?.title} → ${s}`)
  }
  const props: Props = { tasks, repo, onOpen: setOpen, onMove }
  const task = open ? findTask(tasks, open) : undefined

  return (
    <div className={`p-page p-${variant}`}>
      <Dump
        repo={repo}
        onDump={(text) => {
          setTasks((ts) => add(ts, text, repo ?? 'todo'))
          setLast(
            `dumped “${text}” → Inbox of ${repo ?? 'todo (Broker guess)'}`,
          )
        }}
      />
      <RepoChips repo={repo} onChange={setRepo} tasks={tasks} />
      {variant === 'A' && <Lanes {...props} />}
      {variant === 'D' && <Lanes {...props} split />}
      {variant === 'B' && <ByRepo {...props} />}
      {variant === 'C' && <Strip {...props} />}
      {task && (
        <Panel
          task={task}
          parent={findParent(tasks, task.id)}
          onOpen={setOpen}
          onClose={() => setOpen(null)}
          onMove={onMove}
        />
      )}
      <pre className="p-state">
        {last || 'no action yet'} · {count(tasks)} Tasks · repo=
        {repo ?? 'all'}
      </pre>
      <Switcher variants={VARIANTS} current={variant} onChange={change} />
    </div>
  )
}

function count(ts: PTask[]): number {
  return ts.reduce((a, x) => a + 1 + count(x.subtasks), 0)
}

function Dump({
  repo,
  onDump,
}: {
  repo: string | null
  onDump: (t: string) => void
}) {
  const [text, setText] = useState('')
  const [answer, setAnswer] = useState('')
  return (
    <form
      className="p-dump"
      onSubmit={(e) => {
        e.preventDefault()
        if (text) onDump(text)
        setText('')
      }}
    >
      <input
        value={text}
        onChange={(e) => setText(e.target.value)}
        placeholder={`Dump a Task or ask about ${repo ?? 'every Repo'}…`}
      />
      <button type="submit">Add</button>
      <button
        type="button"
        onClick={() =>
          setAnswer(
            `(Broker, stubbed) About “${text || '…'}”: 2 Tasks are Overdue, both errands in house-move.`,
          )
        }
      >
        Ask
      </button>
      {answer && <p className="p-answer">{answer}</p>}
    </form>
  )
}

function RepoChips({
  repo,
  onChange,
  tasks,
}: {
  repo: string | null
  onChange: (r: string | null) => void
  tasks: PTask[]
}) {
  const live = (r: string) =>
    tasks.filter((t) => t.repo === r && !ENDED.includes(t.state)).length
  return (
    <nav className="p-chips">
      <button
        type="button"
        aria-pressed={repo === null}
        onClick={() => onChange(null)}
      >
        All Repos
      </button>
      {REPOS.map((r) => (
        <button
          type="button"
          key={r}
          aria-pressed={repo === r}
          onClick={() => onChange(r)}
        >
          {r} <small>{live(r)}</small>
        </button>
      ))}
    </nav>
  )
}

function Card({
  task,
  onOpen,
  showRepo,
  showState,
}: {
  task: PTask
  onOpen: (id: string) => void
  showRepo?: boolean
  showState?: boolean
}) {
  const done = task.subtasks.filter((s) => ENDED.includes(s.state)).length
  return (
    <button
      type="button"
      className={`p-card ${ENDED.includes(task.state) ? 'p-ended' : ''}`}
      onClick={() => onOpen(task.id)}
    >
      {task.parents && task.parents.length > 0 && (
        <small className="p-parents">{task.parents.join(' › ')} ›</small>
      )}
      <span className="p-title">{task.title}</span>
      <span className="p-facts">
        {showRepo && <b className="p-repo">{task.repo}</b>}
        {showState && <i>{task.state}</i>}
        {task.tags.map((g) => (
          <span key={g}>#{g}</span>
        ))}
        {task.deadline && (
          <span className={overdue(task) ? 'p-overdue' : ''}>
            due {task.deadline.slice(5)}
          </span>
        )}
        {task.until && <span>until {task.until.slice(5)}</span>}
        {task.estimate && <span>{task.estimate}</span>}
        {task.series && <span>↻ {task.series}</span>}
        {task.blockedBy && <span>⛔ blocked</span>}
        {task.subtasks.length > 0 && (
          <span>
            {done}/{task.subtasks.length}
          </span>
        )}
      </span>
    </button>
  )
}

const inRepo = (tasks: PTask[], repo: string | null) =>
  repo ? tasks.filter((t) => t.repo === repo) : tasks

/* A — kanban lanes by State. Drag is replaced by a tap-and-pick for now. */
function Lanes({
  tasks,
  repo,
  onOpen,
  onMove,
  split,
}: Props & { split?: boolean }) {
  // D: a parent is virtual; its leaves are the cards, carrying its title.
  const leaves = (ts: PTask[], path: string[]): PTask[] =>
    ts.flatMap((x) =>
      split && x.subtasks.length
        ? leaves(x.subtasks, [...path, x.title])
        : [{ ...x, parents: path }],
    )
  const shown = leaves(inRepo(tasks, repo), [])
  const lanes: { name: string; states: PState[] }[] = [
    { name: 'Inbox', states: ['Inbox'] },
    { name: 'Backlog', states: ['Backlog'] },
    { name: 'Doing', states: ['Doing'] },
    { name: 'Deferred', states: ['Deferred'] },
    ...(split
      ? [
          { name: 'Done', states: ['Done' as PState] },
          { name: 'Declined', states: ['Declined' as PState] },
        ]
      : [{ name: 'Ended', states: ['Done', 'Declined'] as PState[] }]),
  ]
  return (
    <div className="p-lanes">
      {lanes.map((lane) => {
        const cards = shown.filter((t) => lane.states.includes(t.state))
        return (
          <section
            key={lane.name}
            className="p-lane"
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) =>
              onMove(e.dataTransfer.getData('text'), lane.states[0]!)
            }
          >
            <h3>
              {lane.name} <small>{cards.length}</small>
            </h3>
            {cards.map((t) => (
              <div
                key={t.id}
                draggable
                onDragStart={(e) => e.dataTransfer.setData('text', t.id)}
              >
                <Card task={t} onOpen={onOpen} showRepo={!repo} />
              </div>
            ))}
          </section>
        )
      })}
    </div>
  )
}

/* B — Google Tasks / tasks.md: a section per Repo, nested Subtasks, a
   checkbox that ends a Task, ended ones sunk under a fold. */
function ByRepo({ tasks, repo, onOpen, onMove }: Props) {
  const repos = repo ? [repo] : REPOS
  return (
    <div className="p-byrepo">
      {repos.map((r) => {
        const mine = tasks.filter((t) => t.repo === r)
        const live = mine.filter((t) => !ENDED.includes(t.state))
        const ended = mine.filter((t) => ENDED.includes(t.state))
        return (
          <section key={r} className="p-repo-section">
            <h3>{r}</h3>
            <Tree tasks={live} onOpen={onOpen} onMove={onMove} />
            {ended.length > 0 && (
              <details>
                <summary>Ended ({ended.length})</summary>
                <Tree tasks={ended} onOpen={onOpen} onMove={onMove} />
              </details>
            )}
          </section>
        )
      })}
    </div>
  )
}

function Tree({
  tasks,
  onOpen,
  onMove,
}: {
  tasks: PTask[]
  onOpen: (id: string) => void
  onMove: (id: string, s: PState) => void
}) {
  return (
    <ul className="p-tree">
      {tasks.map((t) => (
        <li key={t.id}>
          <div className="p-line">
            <input
              type="checkbox"
              checked={t.state === 'Done'}
              onChange={(e) =>
                onMove(t.id, e.target.checked ? 'Done' : 'Backlog')
              }
            />
            <Card task={t} onOpen={onOpen} showState />
          </div>
          {t.subtasks.length > 0 && (
            <Tree tasks={t.subtasks} onOpen={onOpen} onMove={onMove} />
          )}
        </li>
      ))}
    </ul>
  )
}

/* C — one list across Repos, a State strip as the Narrowing, Repo on each
   row, the box pinned to the bottom under the thumb. */
function Strip({ tasks, repo, onOpen }: Props) {
  const [states, setStates] = useState<PState[]>(['Inbox', 'Backlog', 'Doing'])
  const shown = inRepo(tasks, repo).filter((t) => states.includes(t.state))
  const toggle = (s: PState) =>
    setStates((ss) => (ss.includes(s) ? ss.filter((x) => x !== s) : [...ss, s]))
  const order = (t: PTask) =>
    (overdue(t) ? -10 : 0) + STATES.indexOf(t.state) * -1
  return (
    <div className="p-strip">
      <nav className="p-states">
        {STATES.map((s) => (
          <button
            type="button"
            key={s}
            aria-pressed={states.includes(s)}
            onClick={() => toggle(s)}
          >
            {s}{' '}
            <small>
              {inRepo(tasks, repo).filter((t) => t.state === s).length}
            </small>
          </button>
        ))}
      </nav>
      <ul className="p-flat">
        {[...shown]
          .sort((a, b) => order(a) - order(b))
          .map((t) => (
            <li key={t.id}>
              <Card task={t} onOpen={onOpen} showRepo={!repo} showState />
            </li>
          ))}
      </ul>
    </div>
  )
}

/* The Task opened: detail, State moves, Subtasks, and Break down. */
function Panel({
  task,
  parent,
  onOpen,
  onClose,
  onMove,
}: {
  task: PTask
  parent?: PTask
  onOpen: (id: string) => void
  onClose: () => void
  onMove: (id: string, s: PState) => void
}) {
  const [proposed, setProposed] = useState<string[] | null>(null)
  return (
    <aside className="p-panel">
      <header>
        <button type="button" onClick={onClose}>
          ✕
        </button>
        <b>{task.repo}</b>
      </header>
      {parent && (
        <section className="p-parent">
          <button type="button" onClick={() => onOpen(parent.id)}>
            Part of: {parent.title} <i>{parent.state}</i>
          </button>
          <ul>
            {parent.subtasks.map((s) => (
              <li key={s.id}>
                {s.id === task.id ? <b>{s.title}</b> : s.title} <i>{s.state}</i>
              </li>
            ))}
          </ul>
        </section>
      )}
      <h2>{task.title}</h2>
      {task.description && <p>{task.description}</p>}
      {task.reason && <p>Reason: {task.reason}</p>}
      {task.blockedBy && <p>Blocked by: {task.blockedBy}</p>}
      <div className="p-moves">
        {STATES.map((s) => (
          <button
            type="button"
            key={s}
            aria-pressed={task.state === s}
            onClick={() => onMove(task.id, s)}
          >
            {s}
          </button>
        ))}
      </div>
      <ul>
        {task.subtasks.map((s) => (
          <li key={s.id}>
            {s.title} <i>{s.state}</i>
          </li>
        ))}
      </ul>
      <button type="button" onClick={() => setProposed(PROPOSALS)}>
        Break down
      </button>
      {proposed && (
        <ul>
          {proposed.map((p) => (
            <li key={p}>
              <label>
                <input type="checkbox" defaultChecked /> {p}
              </label>
            </li>
          ))}
        </ul>
      )}
    </aside>
  )
}
