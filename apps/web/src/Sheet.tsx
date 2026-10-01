// The form: a Task to add, or one open to edit. A bottom sheet on a phone and a
// side panel at a desk, as the panel is. Nothing is written before submit, and
// whoever opens it says what submitting does, which is what lets one form be
// the add form, the edit form and the sheet a dump opens filled in.

import { useState } from 'react'

import { sentence } from './api'
import {
  COLORS,
  ESTIMATES,
  fetchFiles,
  type Files,
  LEVELS,
  named,
  STATES,
} from './state'
import { useRead } from './read'
import type { Draft } from './write'

/** The attributes this form takes as typed text. */
type Said =
  | 'title'
  | 'description'
  | 'why'
  | 'acceptance'
  | 'deadline'
  | 'priority'
  | 'impact'
  | 'estimate'
  | 'color'
  | 'reason'
  | 'until'

export function Sheet({
  draft,
  repos,
  existing,
  offline,
  action,
  onSubmit,
  onCancel,
}: {
  draft: Draft
  /** The Repos a new Task can be written to. An edit's Repo is fixed. */
  repos: string[]
  /**
   * Whether the Task already exists. An edit keeps its Repo and its State,
   * since a Task changes State by a move, which holds the rules for it.
   */
  existing: boolean
  /** Whether the API is out of reach, so submitting could write nothing. */
  offline: boolean
  /** The word on the button, which is what submitting it does. */
  action: string
  onSubmit: (draft: Draft) => Promise<void>
  onCancel: () => void
}) {
  const [body, setBody] = useState<Draft>(draft)
  // Tags and Blocked by are typed as text and read into sets on submit, so a
  // half-typed word is not split under the cursor.
  const [tags, setTags] = useState(draft.tags.map((t) => `#${t}`).join(' '))
  const [blockers, setBlockers] = useState(draft.blockedBy.join(', '))
  const [error, setError] = useState<string | null>(null)
  const [writing, setWriting] = useState(false)

  const say = (name: Said, value: string) =>
    setBody((was) => ({ ...was, [name]: value }))

  const deferred = body.state === 'deferred'
  const reasoned = deferred || body.state === 'declined'

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    setWriting(true)
    setError(null)
    try {
      await onSubmit({
        ...body,
        tags: words(tags).map((t) => t.replace(/^#/, '')),
        blockedBy: words(blockers).map((id) => id.replace(/^\^/, '')),
        // A new Task carries no value the form stopped showing when its State
        // was changed: nobody can correct what is not on the screen. An edit
        // sends only what changed, so a hidden one stays as it was.
        ...(existing
          ? {}
          : {
              reason: reasoned ? body.reason : '',
              until: deferred ? body.until : '',
            }),
      })
    } catch (caught) {
      // The API's sentence is the one the CLI would have printed, and the
      // value it could not read is still in its field to be corrected.
      setError(sentence(caught))
      setWriting(false)
    }
  }

  return (
    <form className="sheet" onSubmit={(event) => void submit(event)}>
      {error && <p className="message">{error}</p>}

      <label className="field">
        <span>Title</span>
        <input
          value={body.title}
          onChange={(event) => say('title', event.target.value)}
          required
          autoFocus
        />
      </label>

      {existing ? (
        <p className="aside">
          {body.repo} · {named(body.state)}
        </p>
      ) : (
        <div className="pair">
          <label className="field">
            <span>Repo</span>
            <select
              value={body.repo}
              onChange={(event) =>
                setBody((was) => ({ ...was, repo: event.target.value }))
              }
            >
              {repos.map((repo) => (
                <option key={repo} value={repo}>
                  {repo}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span>State</span>
            <select
              value={body.state}
              onChange={(event) =>
                setBody((was) => ({
                  ...was,
                  state: event.target.value as Draft['state'],
                }))
              }
            >
              {STATES.map((state) => (
                <option key={state} value={state}>
                  {named(state)}
                </option>
              ))}
            </select>
          </label>
        </div>
      )}

      {reasoned && (
        <label className="field">
          <span>Reason</span>
          <input
            value={body.reason}
            onChange={(event) => say('reason', event.target.value)}
          />
        </label>
      )}
      {deferred && (
        <Day
          name="Until"
          value={body.until}
          onChange={(value) => say('until', value)}
        />
      )}

      <label className="field">
        <span>Description</span>
        <textarea
          value={body.description}
          onChange={(event) => say('description', event.target.value)}
          rows={3}
        />
      </label>

      <label className="field">
        <span>Why</span>
        <input
          value={body.why}
          onChange={(event) => say('why', event.target.value)}
        />
      </label>

      <label className="field">
        <span>Acceptance</span>
        <input
          value={body.acceptance}
          onChange={(event) => say('acceptance', event.target.value)}
        />
      </label>

      <label className="field">
        <span>Tags</span>
        <input
          value={tags}
          onChange={(event) => setTags(event.target.value)}
          placeholder="#errand #van"
        />
      </label>

      <Day
        name="Deadline"
        value={body.deadline}
        onChange={(value) => say('deadline', value)}
      />

      <Choice
        name="Estimate"
        options={ESTIMATES}
        value={body.estimate}
        onPick={(value) => say('estimate', value)}
      />
      <Choice
        name="Priority"
        options={LEVELS.map((value) => ({ value, label: value }))}
        value={body.priority}
        onPick={(value) => say('priority', value)}
      />
      <Choice
        name="Impact"
        options={LEVELS.map((value) => ({ value, label: value }))}
        value={body.impact}
        onPick={(value) => say('impact', value)}
      />
      <Choice
        name="Color"
        options={COLORS.map((value) => ({ value, label: value }))}
        value={body.color}
        onPick={(value) => say('color', value)}
      />

      <label className="field">
        <span>Blocked by</span>
        <input
          value={blockers}
          onChange={(event) => setBlockers(event.target.value)}
          placeholder="m3qa, v9t1"
        />
      </label>

      <Pointers
        on={body.attach}
        onChange={(attach) => setBody((was) => ({ ...was, attach }))}
      />

      <div className="buttons">
        <button type="button" onClick={onCancel} disabled={writing}>
          Cancel
        </button>
        <button type="submit" disabled={writing || offline}>
          {writing ? '…' : action}
        </button>
      </div>
    </form>
  )
}

/** Words typed with spaces or commas between them. */
function words(text: string): string[] {
  return text.split(/[\s,]+/).filter(Boolean)
}

/**
 * A date, typed and picked beside being typed. The box is the field: a day
 * this program cannot read stays in it and the API says so in its own words.
 * The picker writes into the box and never reads it, never writes an empty
 * one (a native date input fires "" when it is cleared), and blanks itself
 * after each pick so the same day can be picked again.
 */
function Day({
  name,
  value,
  onChange,
}: {
  name: string
  value: string
  onChange: (value: string) => void
}) {
  return (
    <fieldset className="field">
      <legend>{name}</legend>
      <div className="pair">
        <input
          aria-label={`${name} as typed`}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder="2026-03-04"
        />
        <input
          type="date"
          aria-label={`Pick ${name.toLowerCase()}`}
          onChange={(event) => {
            if (!event.target.value) return
            onChange(event.target.value)
            event.target.value = ''
          }}
        />
      </div>
    </fieldset>
  )
}

/**
 * One attribute picked from a list. A value that is none of them, typed into
 * the file by hand or said by the Broker, is offered as one more rather than
 * dropped; the API refuses it in its own words. Empty clears the attribute.
 */
function Choice({
  name,
  options,
  value,
  onPick,
}: {
  name: string
  options: { value: string; label: string }[]
  value: string
  onPick: (value: string) => void
}) {
  const shown =
    value === '' || options.some((one) => one.value === value)
      ? options
      : [...options, { value, label: value }]
  return (
    <label className="field">
      <span>{name}</span>
      <select value={value} onChange={(event) => onPick(event.target.value)}>
        <option value="">—</option>
        {shown.map((one) => (
          <option key={one.value} value={one.value}>
            {one.label}
          </option>
        ))}
      </select>
    </label>
  )
}

/**
 * The Task's Attachments, one `attachment:` line each. An edit opens on the
 * ones it carries, so Remove takes one off and Attach adds one; submitting
 * sends the difference.
 *
 * A pointer is text and nothing else. Nothing is uploaded and nothing fetched,
 * so one naming a file names it on the machine `tasks api` runs on rather than
 * on the phone it was typed into.
 */
function Pointers({
  on,
  onChange,
}: {
  on: string[]
  onChange: (on: string[]) => void
}) {
  const [target, setTarget] = useState('')
  const [browsing, setBrowsing] = useState(false)

  const add = () => {
    const pointer = target.trim()
    if (pointer === '' || on.includes(pointer)) return
    onChange([...on, pointer])
    setTarget('')
  }

  return (
    <fieldset className="field">
      <legend>Attachments</legend>
      {on.map((pointer) => (
        <div className="pair pointer" key={pointer}>
          <span>{pointer}</span>
          <button
            type="button"
            onClick={() => onChange(on.filter((other) => other !== pointer))}
          >
            Remove
          </button>
        </div>
      ))}
      <div className="pair">
        <input
          aria-label="New attachment"
          placeholder="https://… or /a/path"
          value={target}
          onChange={(event) => setTarget(event.target.value)}
          onKeyDown={(event) => {
            // Enter here collects the pointer; it would otherwise submit the
            // form without it. An Enter that picks an IME's candidate is the
            // IME's.
            if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
            event.preventDefault()
            add()
          }}
        />
        <button type="button" onClick={() => setBrowsing(!browsing)}>
          {browsing ? 'Close' : 'Browse'}
        </button>
        <button type="button" onClick={add} disabled={target.trim() === ''}>
          Attach
        </button>
      </div>
      {browsing && (
        <Machine
          onPick={(path) => {
            setTarget(path)
            setBrowsing(false)
          }}
        />
      )}
    </fieldset>
  )
}

/**
 * The machine `tasks api` runs on, one directory at a time. It is here because
 * a pointer naming a file names it on that machine: the browser's own file
 * input answers with a bare filename and no directory, so a file chosen on a
 * phone would be a path the host cannot resolve.
 *
 * It fills the box and never reads it back, which is the rule the deadline's
 * picker follows: a pointer half typed is not a path this could show, and the
 * box stays the field that is submitted.
 *
 * It lists and never opens. Nothing is fetched and nothing is copied in, so
 * what a name points at is as unknown here as it is to the store.
 */
function Machine({ onPick }: { onPick: (path: string) => void }) {
  const [at, setAt] = useState<string | undefined>(undefined)
  // The same read and the same guard the other screens make theirs through,
  // rather than a second copy of both written out here.
  const { value: files, error } = useRead<Files | null>(
    () => fetchFiles(at),
    null,
    [at],
  )

  if (!files) return <p className="aside">{error ?? '…'}</p>
  return (
    <div role="group" aria-label="Files">
      {/* Over the listing rather than instead of it: a directory that cannot
          be read is one tap from where somebody already was, and a picker
          replaced by a sentence has no Up button left to take it. */}
      {error !== null && <p className="aside">{error}</p>}
      <p className="aside">{files.path}</p>
      {files.parent !== '' && (
        <button
          type="button"
          className="row"
          onClick={() => setAt(files.parent)}
        >
          <span>Up a directory</span>
        </button>
      )}
      {files.entries.map((one) => {
        // `tasks api` is a Unix service, so a join on `/` is the separator its
        // paths are spelled with rather than a guess at the host's; every path
        // the route answers is absolute, and the route is the only thing that
        // names a directory here. The root is the one path already ending in
        // the separator, and what goes in the box is what somebody reads.
        const path = `${files.path === '/' ? '' : files.path}/${one.name}`
        return (
          <button
            key={one.name}
            type="button"
            className="row"
            onClick={() => (one.dir ? setAt(path) : onPick(path))}
          >
            <span>{one.dir ? `${one.name}/` : one.name}</span>
          </button>
        )
      })}
      {files.entries.length === 0 && <p className="aside">Nothing here.</p>}
    </div>
  )
}
