// @vitest-environment jsdom

// The form's grammars: what each field sends, which fields an add and an edit
// show, and the pickers that write into a typed box.

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { Sheet } from './Sheet'
import * as state from './state'
import { blank, type Draft } from './write'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

/**
 * Renders the form on a draft and answers with what submitting it sent. It
 * opens on a Task that exists unless a test says otherwise.
 */
function opened(fields: Partial<Draft>, existing = true) {
  const sent: Draft[] = []
  render(
    <Sheet
      draft={{ ...blank('house-move'), ...fields }}
      repos={['house-move', 'work']}
      existing={existing}
      action="Save"
      onSubmit={(body) => {
        sent.push(body)
        return Promise.resolve()
      }}
      onCancel={() => {}}
    />,
  )
  return {
    /** What submitting sent, which is what every assertion below is about. */
    submit: () => {
      fireEvent.click(screen.getByRole('button', { name: 'Save' }))
      const body = sent.at(-1)
      if (!body) throw new Error('the form submitted nothing')
      return body
    },
    pick: (label: string, value: string) =>
      fireEvent.change(screen.getByLabelText(label), { target: { value } }),
  }
}

const EVERYTHING: Draft = {
  repo: 'work',
  title: 'Ship the report',
  state: 'deferred',
  tags: ['writing', 'q4'],
  description: 'All four sections.',
  why: 'The board asked.',
  acceptance: 'Sent to the board.',
  deadline: '2026-11-01',
  priority: 'high',
  impact: 'med',
  estimate: 'large',
  color: 'blue',
  blockedBy: ['m3qa', 'v9t1'],
  reason: 'waiting on numbers',
  until: '2026-10-15',
  attach: ['/home/n/report.md'],
}

test('every attribute the form opens on is what it submits', () => {
  const sheet = opened(EVERYTHING)
  expect(sheet.submit()).toEqual(EVERYTHING)
})

test('tags and blockers are typed as words and sent as sets', () => {
  const sheet = opened({ title: 'Ship it' })
  sheet.pick('Tags', '#writing q4,  #board')
  sheet.pick('Blocked by', '^m3qa, v9t1')
  const body = sheet.submit()
  expect(body.tags).toEqual(['writing', 'q4', 'board'])
  expect(body.blockedBy).toEqual(['m3qa', 'v9t1'])
})

test('an estimate is offered as S, M and L', () => {
  opened({ title: 'Ship it' })
  const picker = screen.getByLabelText('Estimate') as HTMLSelectElement
  expect([...picker.options].map((o) => o.textContent)).toEqual([
    '—',
    'S',
    'M',
    'L',
  ])
})

// A Reason is held while a Task is Deferred or Declined, and Until while it is
// Deferred, so the form asks for each only then.
test('reason and until are asked for only in the states that hold them', () => {
  const sheet = opened({ title: 'Ship it' }, false)
  expect(screen.queryByLabelText('Reason')).toBeNull()
  expect(screen.queryByLabelText('Until as typed')).toBeNull()

  sheet.pick('State', 'declined')
  expect(screen.getByLabelText('Reason')).toBeDefined()
  expect(screen.queryByLabelText('Until as typed')).toBeNull()

  sheet.pick('State', 'deferred')
  sheet.pick('Reason', 'after the move')
  sheet.pick('Until as typed', '2026-11-01')
  const body = sheet.submit()
  expect(body.state).toBe('deferred')
  expect(body.reason).toBe('after the move')
  expect(body.until).toBe('2026-11-01')
})

// What the form stopped showing is not sent: nobody could correct it.
test('a new task drops a reason and until its state no longer shows', () => {
  const sheet = opened({ title: 'Ship it', state: 'deferred' }, false)
  sheet.pick('Reason', 'after the move')
  sheet.pick('Until as typed', '2026-11-01')
  sheet.pick('State', 'backlog')
  const body = sheet.submit()
  expect(body.reason).toBe('')
  expect(body.until).toBe('')
})

// A Task changes State by a move, and its Repo is where its file is.
test('an edit shows its repo and state rather than offering them', () => {
  opened({ title: 'Ship it', repo: 'work', state: 'doing' })
  expect(screen.queryByLabelText('Repo')).toBeNull()
  expect(screen.queryByLabelText('State')).toBeNull()
  expect(screen.getByText('work · Doing')).toBeDefined()
})

test('an add picks the repo it is written to', () => {
  const sheet = opened({ title: 'Ship it' }, false)
  sheet.pick('Repo', 'work')
  expect(sheet.submit().repo).toBe('work')
})

test('the color picker offers the colors the API takes', () => {
  opened({ title: 'Ship it' })
  const color = screen.getByLabelText('Color') as HTMLSelectElement
  expect([...color.options].map((o) => o.value)).toEqual(['', ...state.COLORS])
})

// An edit opens on the attachments the Task carries, so Remove is a detach.
test('an attachment the task carries can be taken off', () => {
  const sheet = opened({ title: 'Ship it', attach: ['/a', '/b'] })
  fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0]!)
  expect(sheet.submit().attach).toEqual(['/b'])
})

// A value typed into the file by hand, or said by the Broker, is offered as one
// more rather than dropped on the way to the screen.
test('a level the client does not offer is on the picker and submitted', () => {
  const sheet = opened({ title: 'Ship it', priority: 'urgent' })
  const picker = screen.getByLabelText('Priority') as HTMLSelectElement

  expect([...picker.options].map((o) => o.value)).toContain('urgent')
  expect(picker.value).toBe('urgent')

  const body = sheet.submit()
  expect(body.priority).toBe('urgent')
})

test('a color the client does not offer is on the picker too', () => {
  opened({ title: 'Ship it', color: 'chartreuse' })
  const picker = screen.getByLabelText('Color') as HTMLSelectElement
  expect(picker.value).toBe('chartreuse')
})

test('emptying a picker clears the attribute rather than leaving it alone', () => {
  const sheet = opened({ title: 'Ship it', priority: 'high' })
  sheet.pick('Priority', '')
  const body = sheet.submit()
  expect(body.priority).toBe('')
})

// An Attachment is collected and not written: the sheet is the gate.
test('an attachment typed on the sheet comes back with the body', () => {
  const sheet = opened({ title: 'Buy milk' })
  fireEvent.change(screen.getByLabelText('New attachment'), {
    target: { value: '/home/neal/receipt.pdf' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  expect(sheet.submit().attach).toEqual(['/home/neal/receipt.pdf'])
})

test('a collected attachment is taken off before anything is written', () => {
  const sheet = opened({ title: 'Buy milk' })
  const typed = screen.getByLabelText('New attachment')
  fireEvent.change(typed, { target: { value: '/one' } })
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  fireEvent.change(typed, { target: { value: '/two' } })
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  fireEvent.click(screen.getAllByRole('button', { name: 'Remove' })[0]!)
  expect(sheet.submit().attach).toEqual(['/two'])
})

// The picker writes into the box, which stays the field. A day picked is the
// same text somebody could have typed, and is still editable afterwards.
test('a picked date lands in the deadline box as text', () => {
  const sheet = opened({ title: 'Buy milk' })
  fireEvent.change(screen.getByLabelText('Pick deadline'), {
    target: { value: '2026-03-04' },
  })
  expect(
    (screen.getByLabelText('Deadline as typed') as HTMLInputElement).value,
  ).toBe('2026-03-04')
  expect(sheet.submit().deadline).toBe('2026-03-04')
})

// The one thing the picker must not do. A phrase the API refuses is still the
// reason the box exists — the refusal comes back with the phrase still in the
// field — and a picker reaching into it would blank or guess at that phrase,
// which is the failure this shape was chosen to avoid.
test('a phrase the picker cannot show is left in the box', () => {
  const sheet = opened({ title: 'Buy milk', deadline: 'next Friday' })
  expect(
    (screen.getByLabelText('Deadline as typed') as HTMLInputElement).value,
  ).toBe('next Friday')
  expect(sheet.submit().deadline).toBe('next Friday')
})

// The picker browses the machine that will resolve the path, because a
// pointer typed on a phone names a file on the host and the browser's own file
// input cannot answer with a directory at all. It fills the box and never reads
// it back, which is the rule the deadline's picker follows.
test('a file picked off the machine fills the attachment box', async () => {
  vi.spyOn(state, 'fetchFiles').mockImplementation((path?: string) =>
    Promise.resolve(
      path === undefined
        ? {
            path: '/home/neal',
            parent: '',
            entries: [
              { name: 'papers', dir: true },
              { name: 'note.txt', dir: false },
            ],
          }
        : {
            path,
            parent: '/home/neal',
            entries: [{ name: 'deed', dir: false }],
          },
    ),
  )
  const sheet = opened({ title: 'Move house' })

  fireEvent.click(screen.getByRole('button', { name: 'Browse' }))
  // A directory is somewhere to go and a file is something to point at, so the
  // first tap lists and the second fills.
  fireEvent.click(await screen.findByRole('button', { name: 'papers/' }))
  fireEvent.click(await screen.findByRole('button', { name: 'deed' }))

  const typed = screen.getByLabelText('New attachment')
  expect((typed as HTMLInputElement).value).toBe('/home/neal/papers/deed')
  // Filling the box is not attaching: the box is still the field, and Attach
  // is still what collects what is in it.
  expect(screen.queryByRole('button', { name: 'Remove' })).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  expect(sheet.submit().attach).toEqual(['/home/neal/papers/deed'])
})

// The route refuses in its own words — a path outside the root it will look
// in, or one naming nothing — and the picker draws that rather than an empty
// directory, which would say the machine has nothing on it.
test('a refused listing is drawn in the API’s own words', async () => {
  vi.spyOn(state, 'fetchFiles').mockRejectedValue(
    new Error(
      '/etc is outside /home/neal, which is as far as this listener will look',
    ),
  )
  opened({ title: 'Move house' })

  fireEvent.click(screen.getByRole('button', { name: 'Browse' }))
  expect(
    await screen.findByText(
      '/etc is outside /home/neal, which is as far as this listener will look',
    ),
  ).toBeDefined()
})

// A refusal on the way down is one tap from where somebody already was, so the
// directory they were in stays drawn under the sentence. Replacing the picker
// with it takes the Up button away with it, and the only way back out is Close,
// which starts again at the root.
test('a refused listing leaves the directory it was refused from drawn', async () => {
  const listing = vi.spyOn(state, 'fetchFiles').mockResolvedValue({
    path: '/home/neal',
    parent: '',
    entries: [
      { name: 'papers', dir: true },
      { name: 'note.txt', dir: false },
    ],
  })
  opened({ title: 'Move house' })

  fireEvent.click(screen.getByRole('button', { name: 'Browse' }))
  expect(await screen.findByRole('button', { name: 'note.txt' })).toBeDefined()

  listing.mockRejectedValue(new Error('cannot list /home/neal/papers'))
  fireEvent.click(screen.getByRole('button', { name: 'papers/' }))

  expect(await screen.findByText('cannot list /home/neal/papers')).toBeDefined()
  expect(screen.getByRole('button', { name: 'note.txt' })).toBeDefined()
})

// The root is the one path that already ends in the separator, and what the
// join produces is what somebody reads in the box.
test('a file picked at the root of the machine has one separator', async () => {
  vi.spyOn(state, 'fetchFiles').mockResolvedValue({
    path: '/',
    parent: '',
    entries: [{ name: 'swap', dir: false }],
  })
  opened({ title: 'Move house' })

  fireEvent.click(screen.getByRole('button', { name: 'Browse' }))
  fireEvent.click(await screen.findByRole('button', { name: 'swap' }))

  expect(
    (screen.getByLabelText('New attachment') as HTMLInputElement).value,
  ).toBe('/swap')
})

// The same pointer twice is one pointer. It is also what keeps the rows keyed
// apart, since a row is keyed by the pointer it draws, and Remove filters by
// that same text: two rows of `/one` would be one key and one tap taking both.
test('the same attachment collected twice is collected once', () => {
  const sheet = opened({ title: 'Buy milk' })
  const typed = screen.getByLabelText('New attachment')
  fireEvent.change(typed, { target: { value: '/one' } })
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  fireEvent.change(typed, { target: { value: '/one' } })
  fireEvent.click(screen.getByRole('button', { name: 'Attach' }))
  expect(screen.getAllByRole('button', { name: 'Remove' })).toHaveLength(1)
  expect(sheet.submit().attach).toEqual(['/one'])
})

// The picker blanking the box is the same failure from the other side: a
// native date input fires a change carrying the empty string when a keystroke
// clears it, and the phrase in the box is not the picker's to take away.
test('a cleared picker leaves the box alone', () => {
  const sheet = opened({ title: 'Buy milk' })
  const picker = screen.getByLabelText('Pick deadline')
  fireEvent.change(picker, { target: { value: '2026-03-04' } })
  fireEvent.change(picker, { target: { value: '' } })
  expect(
    (screen.getByLabelText('Deadline as typed') as HTMLInputElement).value,
  ).toBe('2026-03-04')
  expect(sheet.submit().deadline).toBe('2026-03-04')
})

// The picker holds nothing of its own, which is the whole of "never reads the
// box". A control left holding the day it wrote would fire nothing when that
// same day is picked again, so somebody who typed over a picked date could not
// pick it back; empty after a pick is what makes the next one a change.
test('the picker holds nothing after it has written', () => {
  const sheet = opened({ title: 'Buy milk', deadline: 'next Friday' })
  const picker = screen.getByLabelText('Pick deadline') as HTMLInputElement
  expect(picker.value).toBe('')

  fireEvent.change(picker, { target: { value: '2026-03-04' } })
  expect(picker.value).toBe('')
  fireEvent.change(screen.getByLabelText('Deadline as typed'), {
    target: { value: 'next Friday' },
  })
  fireEvent.change(picker, { target: { value: '2026-03-04' } })
  expect(sheet.submit().deadline).toBe('2026-03-04')
})
