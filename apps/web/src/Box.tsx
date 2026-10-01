// The box: one field pinned to the bottom of the board, under the thumb. ＋
// hands what is typed to the Broker as a dump, and the add form opens filled
// in with what it read, the Repo it guessed included; ? asks about the Tasks
// in view and draws the answer above the field. Neither writes: the form's
// Add is the only thing that does, and a question writes nothing at all.
//
// In a Task's panel the box amends that Task instead: the dump carries it,
// the edit form opens on the whole Task the Broker answered, and there is
// nothing to ask.

import { useState } from 'react'

import { sentence } from './api'
import type { Narrowing, Task } from './state'
import { ask, blank, capture, type Draft, draftOf } from './write'

type Mode = 'dump' | 'ask'

/** What the Broker reads out of a dump, empty, so its answer replaces it. */
const UNSAID = {
  title: '',
  description: '',
  why: '',
  deadline: '',
  estimate: '',
  priority: '',
  impact: '',
  tags: [],
} satisfies Partial<Draft>

export function Box({
  repos,
  narrowing,
  offline,
  amends,
  onDraft,
}: {
  /** The Repos a guess may name. One it names that is not here is the first. */
  repos: string[]
  /** What the board is narrowed to, which is what a question is about. */
  narrowing: Narrowing
  /** Offline, the box neither reads a dump nor asks. */
  offline: boolean
  /** The Task a dump amends, when the box is in its panel. */
  amends?: { repo: string; task: Task }
  /** Opens the form on what the Broker read. */
  onDraft: (draft: Draft) => void
}) {
  const [mode, setMode] = useState<Mode>('dump')
  const [text, setText] = useState('')
  const [waiting, setWaiting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [answer, setAnswer] = useState<string | null>(null)

  const said = text.trim()

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (said === '' || waiting) return
    setWaiting(true)
    setError(null)
    setAnswer(null)
    try {
      if (mode === 'ask') {
        setAnswer(await ask(said, narrowing))
      } else if (amends) {
        const read = await capture(said, {
          repo: amends.repo,
          task: amends.task.id,
        })
        // The Broker answers the whole Task, so what it left out is cleared
        // rather than kept, and what it is never asked about is kept.
        onDraft({
          ...draftOf(amends.repo, amends.task),
          ...UNSAID,
          ...read,
          repo: amends.repo,
        })
      } else {
        const read = await capture(said)
        const repo =
          read.repo && repos.includes(read.repo) ? read.repo : (repos[0] ?? '')
        onDraft({ ...blank(repo), ...read, repo })
      }
    } catch (caught) {
      setError(sentence(caught))
    } finally {
      setWaiting(false)
    }
  }

  const asking = mode === 'ask'
  const placeholder = asking
    ? 'Ask about the list'
    : amends
      ? 'Say what changes'
      : 'Say the Task'
  return (
    <form className="box" onSubmit={(event) => void submit(event)}>
      {error && <p className="message">{error}</p>}
      {answer && <p className="answer">{answer}</p>}
      <div className="box-row">
        {!amends && (
          <button
            type="button"
            className="control"
            aria-label={
              asking ? 'Asking: switch to adding' : 'Adding: switch to asking'
            }
            disabled={offline || waiting}
            onClick={() => setMode(asking ? 'dump' : 'ask')}
          >
            {asking ? '?' : '＋'}
          </button>
        )}
        <input
          className="dump"
          aria-label={placeholder}
          placeholder={placeholder}
          value={text}
          disabled={offline}
          onChange={(event) => setText(event.target.value)}
        />
        <button
          type="submit"
          className="control"
          disabled={offline || waiting || said === ''}
        >
          {waiting
            ? asking
              ? 'Asking…'
              : 'Reading…'
            : asking
              ? 'Ask'
              : 'Read'}
        </button>
      </div>
    </form>
  )
}
