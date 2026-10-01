// The box: one field pinned to the bottom of the board, under the thumb. ＋
// hands what is typed to the Broker as a dump, and the add form opens filled
// in with what it read, the Repo it guessed included; ? asks about the Tasks
// in view and draws the answer above the field. Neither writes: the form's
// Add is the only thing that does, and a question writes nothing at all.

import { useState } from 'react'

import { sentence } from './api'
import type { Narrowing } from './state'
import { ask, blank, capture, type Draft } from './write'

type Mode = 'dump' | 'ask'

export function Box({
  repos,
  narrowing,
  offline,
  onDraft,
}: {
  /** The Repos a guess may name. One it names that is not here is the first. */
  repos: string[]
  /** What the board is narrowed to, which is what a question is about. */
  narrowing: Narrowing
  /** Offline, the box neither reads a dump nor asks. */
  offline: boolean
  /** Opens the add form on what the Broker read. */
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
  return (
    <form className="box" onSubmit={(event) => void submit(event)}>
      {error && <p className="message">{error}</p>}
      {answer && <p className="answer">{answer}</p>}
      <div className="box-row">
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
        <input
          className="dump"
          aria-label={asking ? 'Ask about the list' : 'Say the Task'}
          placeholder={asking ? 'Ask about the list' : 'Say the Task'}
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
