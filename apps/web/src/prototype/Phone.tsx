// PROTOTYPE — throwaway, issue #109: what the board looks like on a phone.
// Four variants of the page, switchable via `?variant=A|B|C|D`, over stub
// Tasks in memory. The desktop board (#99, variant D) has six State lanes;
// here they collapse four ways:
//   A = one lane at a time, State tabs on top, box pinned under the thumb
//   B = one list grouped by State, sticky headers, ended lanes folded
//   C = capture first: a tab bar of Dump / Board / Ask, Dump is home
//   D = the six lanes side by side, swiped with scroll-snap
// In every variant: a card is a Task with no Subtasks; a tap opens the panel,
// a press held opens "Move to…"; the dump opens the add form filled in; Ask
// answers in prose; Break down proposes Subtasks ticked in the panel.
import { useRef, useState } from 'react'

import {
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
import './phone.css'

const VARIANTS = [
  { key: 'A', name: 'One lane, State tabs' },
  { key: 'B', name: 'One list grouped by State' },
  { key: 'C', name: 'Capture first, tab bar' },
  { key: 'D', name: 'Swipe between lanes' },
]

type Card = { task: PTask; parents: string[]; state: PState }

// A parent's State, worked out from its Subtasks (#99).
function worked(t: PTask): PState {
  if (!t.subtasks.length) return t.state
  const s = t.subtasks.map(worked)
  for (const o of ['Doing', 'Backlog', 'Inbox', 'Deferred'] as PState[])
    if (s.includes(o)) return o
  return s.every((x) => x === 'Declined') ? 'Declined' : 'Done'
}

function leaves(ts: PTask[], parents: string[] = []): Card[] {
  return ts.flatMap((t) =>
    t.subtasks.length
      ? leaves(t.subtasks, [...parents, t.title])
      : [{ task: t, parents, state: t.state }],
  )
}

function guessRepo(text: string): string {
  const l = text.toLowerCase()
  if (/mov|house|couch|mail/.test(l)) return 'house-move'
  if (/nvim|tmux|brew|shell/.test(l)) return 'dotfiles'
  if (/review|recruit|expense|work/.test(l)) return 'job'
  return 'todo'
}

type Draft = { title: string; repo: string; tags: string; deadline: string }

type Ctx = {
  cards: Card[]
  open: (id: string) => void
  hold: (id: string) => void
  dump: (text: string) => void
  ask: (q: string) => void
  answer: string
}

export function Phone() {
  const [variant, setVariant] = useState(
    new URLSearchParams(location.search).get('variant') ?? 'A',
  )
  const [tasks, setTasks] = useState(seed)
  const [repo, setRepo] = useState<string | null>(null)
  const [openId, setOpenId] = useState<string | null>(null)
  const [holdId, setHoldId] = useState<string | null>(null)
  const [draft, setDraft] = useState<Draft | null>(null)
  const [answer, setAnswer] = useState('')
  const [last, setLast] = useState('')

  const change = (key: string) => {
    const url = new URL(location.href)
    url.searchParams.set('variant', key)
    history.replaceState(null, '', url)
    setVariant(key)
  }

  const cards = leaves(tasks).filter((c) => !repo || c.task.repo === repo)
  const ctx: Ctx = {
    cards,
    open: setOpenId,
    hold: (id) => {
      navigator.vibrate?.(15)
      setHoldId(id)
    },
    dump: (text) =>
      text.trim() &&
      setDraft({
        title: text.trim(),
        repo: repo ?? guessRepo(text),
        tags: '',
        deadline: '',
      }),
    ask: (q) =>
      q.trim() &&
      setAnswer(
        `${cards.filter((c) => c.state === 'Doing').length} Tasks are in Doing. ` +
          `“Book movers” is overdue since Sep 28; “Write Q4 review self-assessment” is due Oct 1. ` +
          `Start with the movers.`,
      ),
    answer,
  }

  const moveTo = (id: string, s: PState) => {
    setTasks((ts) => move(ts, id, s))
    setLast(`moved “${findTask(tasks, id)?.title}” → ${s}`)
    setHoldId(null)
  }

  const chips = (
    <div className="ph-chips">
      {[null, ...REPOS].map((r) => (
        <button
          key={r ?? 'all'}
          type="button"
          className={repo === r ? 'on' : ''}
          onClick={() => setRepo(r)}
        >
          {r ?? 'All Repos'}
        </button>
      ))}
    </div>
  )

  return (
    <div className={`ph ph-${variant}`}>
      {variant === 'A' && <VariantA ctx={ctx} chips={chips} />}
      {variant === 'B' && <VariantB ctx={ctx} chips={chips} />}
      {variant === 'C' && <VariantC ctx={ctx} chips={chips} />}
      {variant === 'D' && <VariantD ctx={ctx} chips={chips} />}

      {openId && (
        <Panel
          tasks={tasks}
          id={openId}
          full={variant === 'B' || variant === 'C'}
          onClose={() => setOpenId(null)}
          onOpen={setOpenId}
          onAdd={(parentId, titles) => {
            setTasks((ts) => addSubtasks(ts, parentId, titles))
            setLast(`added ${titles.length} Subtasks`)
          }}
        />
      )}
      {holdId && (
        <Sheet onClose={() => setHoldId(null)}>
          <p className="ph-sheet-title">
            Move “{findTask(tasks, holdId)?.title}” to…
          </p>
          {STATES.map((s) => (
            <button
              key={s}
              type="button"
              className="ph-row-btn"
              disabled={findTask(tasks, holdId)?.state === s}
              onClick={() => moveTo(holdId, s)}
            >
              {s}
            </button>
          ))}
        </Sheet>
      )}
      {draft && (
        <Sheet onClose={() => setDraft(null)} tall>
          <p className="ph-sheet-title">Add a Task — the Broker read it as:</p>
          <label>
            Title
            <input
              value={draft.title}
              onChange={(e) => setDraft({ ...draft, title: e.target.value })}
            />
          </label>
          <label>
            Repo (the Broker's guess)
            <select
              value={draft.repo}
              onChange={(e) => setDraft({ ...draft, repo: e.target.value })}
            >
              {REPOS.map((r) => (
                <option key={r}>{r}</option>
              ))}
            </select>
          </label>
          <label>
            Tags
            <input
              value={draft.tags}
              placeholder="errand, writing"
              onChange={(e) => setDraft({ ...draft, tags: e.target.value })}
            />
          </label>
          <label>
            Deadline
            <input
              type="date"
              value={draft.deadline}
              onChange={(e) => setDraft({ ...draft, deadline: e.target.value })}
            />
          </label>
          <p className="ph-dim">Lands in Inbox.</p>
          <button
            type="button"
            className="ph-primary"
            onClick={() => {
              setTasks((ts) => [
                {
                  id: `new${ts.length}${draft.title.length}`,
                  repo: draft.repo,
                  title: draft.title,
                  state: 'Inbox',
                  tags: draft.tags
                    .split(',')
                    .map((x) => x.trim())
                    .filter(Boolean),
                  deadline: draft.deadline || undefined,
                  subtasks: [],
                },
                ...ts,
              ])
              setLast(`added “${draft.title}” to ${draft.repo} Inbox`)
              setDraft(null)
            }}
          >
            Add
          </button>
        </Sheet>
      )}

      <div className="ph-state">
        {variant} · {cards.length} cards · {last || 'no change yet'}
      </div>
      <Switcher variants={VARIANTS} current={variant} onChange={change} />
    </div>
  )
}

function addSubtasks(ts: PTask[], id: string, titles: string[]): PTask[] {
  return ts.map((t) =>
    t.id === id
      ? {
          ...t,
          subtasks: [
            ...t.subtasks,
            ...titles.map((title, i) => ({
              id: `${id}s${t.subtasks.length + i}`,
              repo: t.repo,
              title,
              state: 'Backlog' as PState,
              tags: [],
              subtasks: [],
            })),
          ],
        }
      : { ...t, subtasks: addSubtasks(t.subtasks, id, titles) },
  )
}

// ---- shared pieces -------------------------------------------------------

function CardView({ card, ctx }: { card: Card; ctx: Ctx }) {
  const timer = useRef<number>(undefined)
  const held = useRef(false)
  const t = card.task
  return (
    <button
      type="button"
      className={`ph-card${overdue(t) ? ' overdue' : ''}`}
      onPointerDown={() => {
        held.current = false
        timer.current = window.setTimeout(() => {
          held.current = true
          ctx.hold(t.id)
        }, 450)
      }}
      onPointerUp={() => clearTimeout(timer.current)}
      onPointerLeave={() => clearTimeout(timer.current)}
      onPointerCancel={() => clearTimeout(timer.current)}
      onContextMenu={(e) => e.preventDefault()}
      onClick={() => !held.current && ctx.open(t.id)}
    >
      {card.parents.length > 0 && (
        <span className="ph-parents">{card.parents.join(' › ')}</span>
      )}
      <span className="ph-title">{t.title}</span>
      <span className="ph-meta">
        <span className="ph-repo">{t.repo}</span>
        {t.tags.map((g) => (
          <span key={g}>#{g}</span>
        ))}
        {t.deadline && <span className="ph-due">⏰ {t.deadline.slice(5)}</span>}
        {t.until && <span>until {t.until.slice(5)}</span>}
        {t.estimate && <span>{t.estimate}</span>}
        {t.blockedBy && <span>⛔ blocked</span>}
        {t.series && <span>↻</span>}
      </span>
    </button>
  )
}

function Box({
  ctx,
  big,
  mode: fixed,
}: {
  ctx: Ctx
  big?: boolean
  mode?: 'dump' | 'ask'
}) {
  const [text, setText] = useState('')
  const [mode, setMode] = useState<'dump' | 'ask'>(fixed ?? 'dump')
  const go = () => {
    if (mode === 'dump') ctx.dump(text)
    else ctx.ask(text)
    setText('')
  }
  return (
    <div className={`ph-box${big ? ' big' : ''}`}>
      {ctx.answer && mode === 'ask' && (
        <p className="ph-answer">{ctx.answer}</p>
      )}
      <div className="ph-box-row">
        {big ? (
          <textarea
            value={text}
            rows={5}
            placeholder={
              mode === 'dump'
                ? 'Say the Task the way you think of it…'
                : 'Ask about the list…'
            }
            onChange={(e) => setText(e.target.value)}
          />
        ) : (
          <input
            value={text}
            enterKeyHint={mode === 'dump' ? 'done' : 'send'}
            placeholder={
              mode === 'dump' ? 'Dump a Task…' : 'Ask about the list…'
            }
            onChange={(e) => setText(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && go()}
          />
        )}
        {!fixed && (
          <button
            type="button"
            className="ph-toggle"
            onClick={() => setMode(mode === 'dump' ? 'ask' : 'dump')}
            title="switch between dump and ask"
          >
            {mode === 'dump' ? '＋' : '?'}
          </button>
        )}
        <button type="button" className="ph-primary" onClick={go}>
          {mode === 'dump' ? 'Read' : 'Ask'}
        </button>
      </div>
    </div>
  )
}

function Sheet({
  children,
  onClose,
  tall,
}: {
  children: React.ReactNode
  onClose: () => void
  tall?: boolean
}) {
  return (
    <div className="ph-scrim" onClick={onClose}>
      <div
        className={`ph-sheet${tall ? ' tall' : ''}`}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="ph-grab" />
        {children}
      </div>
    </div>
  )
}

function Panel({
  tasks,
  id,
  full,
  onClose,
  onOpen,
  onAdd,
}: {
  tasks: PTask[]
  id: string
  full: boolean
  onClose: () => void
  onOpen: (id: string) => void
  onAdd: (parentId: string, titles: string[]) => void
}) {
  const t = findTask(tasks, id)!
  const parent = findParent(tasks, id)
  const [ticks, setTicks] = useState<boolean[] | null>(null)
  const body = (
    <>
      {parent && (
        <div className="ph-parent">
          <button
            type="button"
            className="ph-link"
            onClick={() => onOpen(parent.id)}
          >
            ‹ {parent.title}
          </button>
          <span className="ph-dim">
            {worked(parent)} ·{' '}
            {STATES.map(
              (s) =>
                [
                  s,
                  parent.subtasks.filter((c) => c.state === s).length,
                ] as const,
            )
              .filter(([, n]) => n)
              .map(([s, n]) => `${s} ${n}`)
              .join(' · ')}
          </span>
        </div>
      )}
      <h2>{t.title}</h2>
      <p className="ph-dim">
        {t.repo} · {worked(t)}
        {t.deadline && ` · due ${t.deadline}`}
        {t.until && ` · until ${t.until}`}
        {t.reason && ` · ${t.reason}`}
      </p>
      {t.description && <p>{t.description}</p>}
      {t.subtasks.length > 0 && (
        <ul className="ph-subs">
          {t.subtasks.map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className="ph-link"
                onClick={() => onOpen(c.id)}
              >
                {c.title}
              </button>{' '}
              <span className="ph-dim">{worked(c)}</span>
            </li>
          ))}
        </ul>
      )}
      {ticks ? (
        <div className="ph-proposals">
          <p className="ph-dim">The Broker proposes:</p>
          {PROPOSALS.map((p, i) => (
            <label key={p} className="ph-tick">
              <input
                type="checkbox"
                checked={ticks[i]}
                onChange={() =>
                  setTicks(ticks.map((x, j) => (j === i ? !x : x)))
                }
              />
              {p}
            </label>
          ))}
          <button
            type="button"
            className="ph-primary"
            onClick={() => {
              onAdd(
                t.id,
                PROPOSALS.filter((_, i) => ticks[i]),
              )
              setTicks(null)
            }}
          >
            Add ticked
          </button>
        </div>
      ) : (
        <button
          type="button"
          className="ph-primary"
          onClick={() => setTicks(PROPOSALS.map(() => true))}
        >
          Break down
        </button>
      )}
    </>
  )
  if (full)
    return (
      <div className="ph-full">
        <button type="button" className="ph-back" onClick={onClose}>
          ‹ Back
        </button>
        {body}
      </div>
    )
  return (
    <Sheet onClose={onClose} tall>
      {body}
    </Sheet>
  )
}

// ---- variants ------------------------------------------------------------

function StateTabs({
  cards,
  lane,
  setLane,
}: {
  cards: Card[]
  lane: PState
  setLane: (s: PState) => void
}) {
  return (
    <div className="ph-tabs">
      {STATES.map((s) => (
        <button
          key={s}
          type="button"
          className={lane === s ? 'on' : ''}
          onClick={() => setLane(s)}
        >
          {s} <b>{cards.filter((c) => c.state === s).length}</b>
        </button>
      ))}
    </div>
  )
}

function Lane({ cards, ctx }: { cards: Card[]; ctx: Ctx }) {
  if (!cards.length) return <p className="ph-dim ph-empty">Nothing here.</p>
  return (
    <div className="ph-lane">
      {cards.map((c) => (
        <CardView key={c.task.id} card={c} ctx={ctx} />
      ))}
    </div>
  )
}

// A — one lane at a time; State tabs on top; the box pinned at the bottom.
function VariantA({ ctx, chips }: { ctx: Ctx; chips: React.ReactNode }) {
  const [lane, setLane] = useState<PState>('Doing')
  return (
    <>
      <header className="ph-top">
        {chips}
        <StateTabs cards={ctx.cards} lane={lane} setLane={setLane} />
      </header>
      <Lane cards={ctx.cards.filter((c) => c.state === lane)} ctx={ctx} />
      <footer className="ph-bottom">
        <Box ctx={ctx} />
      </footer>
    </>
  )
}

// B — one list, a sticky header per State; Done and Declined folded.
function VariantB({ ctx, chips }: { ctx: Ctx; chips: React.ReactNode }) {
  const [unfold, setUnfold] = useState<PState[]>([])
  return (
    <>
      <header className="ph-top">
        <Box ctx={ctx} />
        {chips}
      </header>
      {STATES.map((s) => {
        const cs = ctx.cards.filter((c) => c.state === s)
        const folded = (s === 'Done' || s === 'Declined') && !unfold.includes(s)
        return (
          <section key={s}>
            <button
              type="button"
              className="ph-group"
              onClick={() =>
                setUnfold(
                  unfold.includes(s)
                    ? unfold.filter((x) => x !== s)
                    : [...unfold, s],
                )
              }
            >
              {s} <b>{cs.length}</b> {folded ? '▸' : ''}
            </button>
            {!folded && <Lane cards={cs} ctx={ctx} />}
          </section>
        )
      })}
    </>
  )
}

// C — capture first: Dump is the home tab; the board and Ask are one tap away.
function VariantC({ ctx, chips }: { ctx: Ctx; chips: React.ReactNode }) {
  const [tab, setTab] = useState<'dump' | 'board' | 'ask'>('dump')
  const [lane, setLane] = useState<PState>('Doing')
  return (
    <>
      {tab === 'dump' && (
        <div className="ph-home">
          <Box ctx={ctx} big mode="dump" />
          <p className="ph-dim">Doing now</p>
          <Lane
            cards={ctx.cards.filter(
              (c) => c.state === 'Doing' || overdue(c.task),
            )}
            ctx={ctx}
          />
        </div>
      )}
      {tab === 'board' && (
        <>
          <header className="ph-top">
            {chips}
            <StateTabs cards={ctx.cards} lane={lane} setLane={setLane} />
          </header>
          <Lane cards={ctx.cards.filter((c) => c.state === lane)} ctx={ctx} />
        </>
      )}
      {tab === 'ask' && (
        <div className="ph-home">
          {chips}
          <Box ctx={ctx} big mode="ask" />
        </div>
      )}
      <nav className="ph-tabbar">
        {(['dump', 'board', 'ask'] as const).map((k) => (
          <button
            key={k}
            type="button"
            className={tab === k ? 'on' : ''}
            onClick={() => setTab(k)}
          >
            {{ dump: '＋ Dump', board: '▦ Board', ask: '? Ask' }[k]}
          </button>
        ))}
      </nav>
    </>
  )
}

// D — the six lanes side by side, each nearly a screen wide, swiped.
function VariantD({ ctx, chips }: { ctx: Ctx; chips: React.ReactNode }) {
  return (
    <>
      <header className="ph-top">{chips}</header>
      <div className="ph-swipe">
        {STATES.map((s) => {
          const cs = ctx.cards.filter((c) => c.state === s)
          return (
            <section key={s} className="ph-col">
              <h3>
                {s} <b>{cs.length}</b>
              </h3>
              <Lane cards={cs} ctx={ctx} />
            </section>
          )
        })}
      </div>
      <footer className="ph-bottom">
        <Box ctx={ctx} />
      </footer>
    </>
  )
}
