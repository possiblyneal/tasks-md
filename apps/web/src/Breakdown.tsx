// The breakdown screen: a Task handed to the Broker, what it still needs to
// know, and the Subtasks it proposes.
//
// A turn writes nothing. Everything the Broker said lives here until somebody
// approves it, and approving is one write of every ticked proposal. A
// breakdown backed out of leaves nothing behind, which is the same gate the
// form is for a dump.

import { useEffect, useState } from 'react'

import { sentence } from './api'
import type { Task } from './state'
import { addSubtasks, breakdown, type Proposal, type QA } from './write'

export function Breakdown({
  repo,
  task,
  offline,
  onBack,
}: {
  repo: string
  task: Task
  /** Offline, what came back can still be read, but nothing is answered or approved. */
  offline: boolean
  onBack: () => void
}) {
  // The tree as it was when the breakdown opened. Approving is refused if it
  // has changed since, rather than writing under a Task nobody was shown.
  const [version] = useState(task.version)
  // Everything already asked and answered, carried into every turn: the Broker
  // holds nothing between calls, so this is the whole of the conversation.
  const [answered, setAnswered] = useState<QA[]>([])
  const [asking, setAsking] = useState<string[]>([])
  const [replies, setReplies] = useState<string[]>([])
  const [proposals, setProposals] = useState<Proposal[] | null>(null)
  // Which proposals are approved, by position. Two can come back saying the
  // same thing and only the order tells them apart, so a set of titles would
  // decline both halves of a pair when one was unticked.
  const [approved, setApproved] = useState<number[]>([])
  const [error, setError] = useState<string | null>(null)
  const [waiting, setWaiting] = useState(true)

  // One turn, and the screen is whatever came back. The wait is drawn rather
  // than left blank, because a turn is a call to the Broker and slow enough to
  // look like nothing happening. Whoever asks for a turn is what says the wait
  // has started: this screen opens waiting, so the first turn has nothing to
  // set before it is made.
  const turn = (said: QA[]) => {
    breakdown(repo, task.id, said)
      .then((step) => {
        // Proposals win over questions. `POST /api/breakdown` carries both
        // whole and ranks neither, so the order is this screen's to choose:
        // the two are alternatives, and a turn carrying both is the Broker
        // having answered oddly rather than asked a question; taking the
        // questions first would throw the proposals away and ask again for
        // what was already proposed.
        if (step.proposals.length > 0) {
          setAsking([])
          setReplies([])
          setProposals(step.proposals)
          // Every proposal starts ticked and unticking one is how it is
          // declined: the Broker was asked for these, so approving is the
          // default.
          setApproved(step.proposals.map((_, at) => at))
          setWaiting(false)
          return
        }
        const questions = step.questions ?? []
        setAsking(questions)
        setReplies(questions.map(() => ''))
        setProposals(null)
        setApproved([])
        setWaiting(false)
      })
      .catch((caught: unknown) => {
        setError(sentence(caught))
        setWaiting(false)
      })
  }

  useEffect(() => {
    turn([])
    // The first turn is made once, on the Task this screen was opened on.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [task.id])

  const answer = () => {
    const said = [
      ...answered,
      ...asking.map((question, at) => ({
        question,
        answer: replies[at] ?? '',
      })),
    ]
    setAnswered(said)
    setAsking([])
    setWaiting(true)
    setError(null)
    turn(said)
  }

  // The ticked ones, as one write: every one lands or none does, so a refusal
  // leaves the ticks as they were and pressing again cannot write one twice.
  const approve = async () => {
    setWaiting(true)
    setError(null)
    try {
      const ticked = (proposals ?? []).filter((_, at) => approved.includes(at))
      await addSubtasks(repo, task.id, version, ticked)
      onBack()
    } catch (caught) {
      setError(sentence(caught))
      setWaiting(false)
    }
  }

  return (
    <div className="detail">
      <div className="buttons">
        <button type="button" onClick={onBack} disabled={waiting}>
          Back
        </button>
      </div>

      <h1 className="title">{task.title}</h1>
      {error && <p className="message">{error}</p>}
      {waiting && <p className="message">Asking the broker…</p>}

      {!waiting && asking.length > 0 && (
        <form
          onSubmit={(event) => {
            event.preventDefault()
            answer()
          }}
        >
          <h2 className="heading">The broker asks</h2>
          {asking.map((question, at) => (
            <label key={`${at}-${question}`} className="field">
              <span>{question}</span>
              <input
                value={replies[at] ?? ''}
                onChange={(event) =>
                  setReplies((was) =>
                    was.map((reply, other) =>
                      other === at ? event.target.value : reply,
                    ),
                  )
                }
                autoFocus={at === 0}
              />
            </label>
          ))}
          <div className="buttons">
            <button type="submit" disabled={offline}>
              Answer
            </button>
          </div>
        </form>
      )}

      {!waiting && proposals !== null && (
        <>
          <h2 className="heading">Proposed</h2>
          {proposals.length === 0 && <p className="message">Nothing.</p>}
          <ul className="list">
            {proposals.map((proposal, at) => (
              <li key={at}>
                <label className="tick">
                  <input
                    type="checkbox"
                    checked={approved.includes(at)}
                    onChange={() =>
                      setApproved((was) =>
                        was.includes(at)
                          ? was.filter((other) => other !== at)
                          : [...was, at],
                      )
                    }
                  />
                  <span>{proposal.title}</span>
                </label>
                {/*
                  Everything the tick beside it would write, in the Broker's own
                  words and unparsed: a proposal is approved on what it says
                  rather than on what this side could make of it, and a gate
                  over attributes nobody is shown is not a gate. A priority that
                  is none of the three is drawn as the word that was used; the
                  store is what refuses it, and says so in its own sentence.

                  The estimate and the two levels are labelled because they are
                  single words that mean nothing alone — `high` says neither
                  which attribute it is nor that anybody chose it, and `30m` is
                  as much a deadline as an estimate to anybody reading it cold.
                  The line holding them is not drawn when the Broker answered
                  none of the three, the way the description and the why are
                  not.
                */}
                {proposal.description && (
                  <p className="lines">{proposal.description}</p>
                )}
                {proposal.why && <p className="marks">{proposal.why}</p>}
                {(proposal.estimate ||
                  proposal.priority ||
                  proposal.impact) && (
                  <p className="facts">
                    {proposal.estimate && (
                      <span>Estimate {proposal.estimate}</span>
                    )}
                    {proposal.priority && (
                      <span>Priority {proposal.priority}</span>
                    )}
                    {proposal.impact && <span>Impact {proposal.impact}</span>}
                  </p>
                )}
              </li>
            ))}
          </ul>
          <div className="buttons">
            <button
              type="button"
              onClick={() => void approve()}
              disabled={offline || approved.length === 0}
            >
              Add {approved.length}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
