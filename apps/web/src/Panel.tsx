// One Task, opened to read: every attribute it carries, its parent with the
// State the server worked out for it, and its Subtasks. A bottom sheet on a
// phone and a side panel at a desk, which is CSS's to decide. Edit opens the
// form on it, and Delete takes two taps. Its history is at the foot, and under
// it a box whose dump amends the Task through the same form.

import { Fragment, useEffect, useState } from 'react'

import { sentence } from './api'
import { Box } from './Box'
import { Due } from './Due'
import { History } from './History'
import { Series } from './Series'
import { type Repo, type State, type Task, WIDE } from './state'
import type { Draft } from './write'

/** The order the leaf counts are said in: open first, then ended. */
const COUNTED: State[] = [
  'doing',
  'backlog',
  'inbox',
  'deferred',
  'done',
  'declined',
]

export function Panel({
  repo,
  task,
  today,
  onOpen,
  onEdit,
  onAmend,
  onDelete,
  onBreakdown,
  onClose,
  offline,
}: {
  /** The Repo's wide read, in file order, so the whole tree is in it. */
  repo: Repo
  task: Task
  today: string
  onOpen: (id: string) => void
  onEdit: () => void
  /** Opens the form on the Task as a dump amending it was read. */
  onAmend: (draft: Draft) => void
  /** Deletes the Task, its Subtasks with it, and rejects with a refusal. */
  onDelete: () => Promise<void>
  /** Opens the breakdown on this Task in the panel's place. */
  onBreakdown: () => void
  onClose: () => void
  /** Offline, the panel still reads but neither edits nor deletes. */
  offline: boolean
}) {
  // The first tap on Delete asks; the second deletes. No browser dialog,
  // which would block the page.
  const [sure, setSure] = useState(false)
  const [refusal, setRefusal] = useState<string | null>(null)

  const remove = async () => {
    if (!sure) {
      setSure(true)
      return
    }
    try {
      await onDelete()
    } catch (caught) {
      setRefusal(sentence(caught))
      setSure(false)
    }
  }

  useEffect(() => {
    const close = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', close)
    return () => document.removeEventListener('keydown', close)
  }, [onClose])

  const at = repo.tasks.indexOf(task)
  const parent = repo.tasks
    .slice(0, at)
    .reverse()
    .find((t) => t.depth < task.depth)
  const subtasks = under(repo.tasks, at).filter(
    (t) => t.depth === task.depth + 1,
  )

  return (
    <>
      {/* Tapping beside the panel closes it. The Close button is the way a
          reader gets the same thing. */}
      <div className="scrim" aria-hidden="true" onClick={onClose} />
      <div
        className="panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="panel-title"
      >
        <div className="panel-bar">
          <button
            type="button"
            className="control"
            disabled={offline}
            onClick={onEdit}
          >
            Edit
          </button>
          <button
            type="button"
            className="control"
            disabled={offline}
            onClick={() => void remove()}
          >
            {sure ? 'Delete it and its Subtasks' : 'Delete'}
          </button>
          <button
            type="button"
            className="control"
            disabled={offline}
            onClick={onBreakdown}
          >
            Break down
          </button>
          <button type="button" className="control" onClick={onClose}>
            Close
          </button>
        </div>
        {refusal && <p className="message">{refusal}</p>}
        {parent && (
          <button
            type="button"
            className="parent"
            disabled={!parent.id}
            onClick={() => onOpen(parent.id)}
          >
            <span className="parent-title">{parent.title}</span>
            <span className="facts">
              <span>{named(parent.state)}</span>
              <span>{counts(repo.tasks, repo.tasks.indexOf(parent))}</span>
            </span>
          </button>
        )}
        <h2 id="panel-title" className="title">
          {task.title}
        </h2>
        <dl className="attrs">
          <dt>State</dt>
          <dd>{named(task.state)}</dd>
          <dt>Repo</dt>
          <dd>{repo.name}</dd>
          <dt>id</dt>
          <dd>{task.id || '-'}</dd>
          <dt>created</dt>
          <dd>{task.created || '-'}</dd>
          {task.tags.length > 0 && (
            <>
              <dt>tags</dt>
              <dd className="facts">
                {task.tags.map((tag) => (
                  <span key={tag}>#{tag}</span>
                ))}
              </dd>
            </>
          )}
          {task.attrs.map((a, i) => (
            <Fragment key={`${a.label}:${i}`}>
              <dt>{a.label}</dt>
              <dd>
                {a.label === 'deadline' ? (
                  <Due on={a.value} today={today} />
                ) : a.label === 'series' && task.id ? (
                  <Series repo={repo.name} id={task.id} rule={a.value} />
                ) : (
                  a.value
                )}
              </dd>
            </Fragment>
          ))}
          {task.blocked && (
            <>
              <dt>Blocked</dt>
              <dd className="blocked">blocked</dd>
            </>
          )}
        </dl>
        {task.description && <p className="said">{task.description}</p>}
        {subtasks.length > 0 && (
          <>
            <h3 className="heading">
              Subtasks <small>{counts(repo.tasks, at)}</small>
            </h3>
            <ul className="list">
              {subtasks.map((t) => (
                <li key={t.line}>
                  <button
                    type="button"
                    className="parent"
                    disabled={!t.id}
                    onClick={() => onOpen(t.id)}
                  >
                    <span className="parent-title">{t.title}</span>
                    <span className="facts">{named(t.state)}</span>
                  </button>
                </li>
              ))}
            </ul>
          </>
        )}
        {task.id && (
          <History repo={repo.name} id={task.id} version={task.version} />
        )}
        {task.id && (
          <Box
            repos={[repo.name]}
            narrowing={WIDE}
            offline={offline}
            amends={{ repo: repo.name, task }}
            onDraft={onAmend}
          />
        )}
      </div>
    </>
  )
}

/**
 * Everything below the Task at `at` in a flat, file-order read: the Tasks
 * after it, up to the next one no deeper than it.
 */
function under(tasks: Task[], at: number): Task[] {
  const depth = tasks[at]?.depth ?? 0
  const end = tasks.findIndex((t, i) => i > at && t.depth <= depth)
  return tasks.slice(at + 1, end < 0 ? undefined : end)
}

/** How many leaves under the Task at `at` sit in each State, zeros left out. */
function counts(tasks: Task[], at: number): string {
  const leaves = under(tasks, at).filter((t) => t.leaf)
  return COUNTED.map(
    (state) => [state, leaves.filter((t) => t.state === state).length] as const,
  )
    .filter(([, n]) => n > 0)
    .map(([state, n]) => `${named(state)} ${n}`)
    .join(' · ')
}

function named(state: State): string {
  return state[0]?.toUpperCase() + state.slice(1)
}
