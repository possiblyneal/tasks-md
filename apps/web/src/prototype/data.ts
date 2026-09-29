// PROTOTYPE — throwaway. Stub Tasks across four Repos for the all-Repos page
// prototype (issue #99). Nothing here reaches the API; moves live in memory.

export const STATES = [
  'Inbox',
  'Backlog',
  'Doing',
  'Deferred',
  'Done',
  'Declined',
] as const
export type PState = (typeof STATES)[number]
export const ENDED: PState[] = ['Done', 'Declined']

export type PTask = {
  id: string
  repo: string
  title: string
  state: PState
  tags: string[]
  deadline?: string
  estimate?: 'S' | 'M' | 'L'
  blockedBy?: string
  series?: string
  until?: string
  reason?: string
  description?: string
  subtasks: PTask[]
  parents?: string[]
}

let n = 0
const t = (
  repo: string,
  title: string,
  state: PState,
  more: Partial<PTask> = {},
): PTask => ({
  id: `t${++n}`,
  repo,
  title,
  state,
  tags: [],
  subtasks: [],
  ...more,
})

export const REPOS = ['todo', 'house-move', 'dotfiles', 'job']

export const TODAY = '2026-09-29'

export function seed(): PTask[] {
  n = 0
  return [
    t('todo', 'Parse tasks.md title line', 'Doing', {
      tags: ['store'],
      estimate: 'M',
      deadline: '2026-10-03',
      description: 'Split `- [ ] title | ^id state #tags` into fields.',
      subtasks: [
        t('todo', 'Checkbox wins over the word', 'Done'),
        t('todo', 'Round-trip the id', 'Doing'),
        t('todo', 'Fuzz with odd pipes', 'Backlog'),
      ],
    }),
    t('todo', 'Refuse a stale write', 'Backlog', {
      tags: ['store'],
      estimate: 'L',
      blockedBy: 'Parse tasks.md title line',
    }),
    t('todo', 'Google Tasks OAuth on the api host', 'Backlog', {
      tags: ['sync'],
      estimate: 'M',
    }),
    t('todo', 'look into whether orca worktrees double count', 'Inbox', {
      tags: [],
    }),
    t('todo', 'Retire the Lease', 'Done', { tags: ['store'] }),
    t('house-move', 'Book movers', 'Doing', {
      deadline: '2026-09-28',
      tags: ['errand'],
      subtasks: [
        t('house-move', 'Get three quotes', 'Done'),
        t('house-move', 'Pick one and pay deposit', 'Backlog'),
      ],
    }),
    t('house-move', 'Forward mail', 'Backlog', {
      deadline: '2026-10-15',
      tags: ['errand'],
    }),
    t('house-move', 'Change address with bank', 'Inbox', { tags: ['errand'] }),
    t('house-move', 'Sell the old couch', 'Deferred', {
      until: '2026-10-10',
      reason: 'wait until the new one arrives',
    }),
    t('house-move', 'Repaint the hallway', 'Declined', {
      reason: 'landlord will do it',
    }),
    t('dotfiles', 'Move nvim config to lua', 'Deferred', {
      reason: 'someday',
      tags: ['editor'],
    }),
    t('dotfiles', 'Weekly: update brew bundle', 'Backlog', {
      series: 'every Monday',
      deadline: '2026-10-05',
    }),
    t('dotfiles', 'fix tmux copy on wayland??', 'Inbox', { tags: ['editor'] }),
    t('job', 'Write Q4 review self-assessment', 'Backlog', {
      deadline: '2026-10-01',
      tags: ['writing'],
      estimate: 'M',
    }),
    t('job', 'Reply to recruiter', 'Inbox'),
    t('job', 'Expense the conference', 'Done', { tags: ['errand'] }),
  ]
}

export function overdue(task: PTask): boolean {
  return !!task.deadline && task.deadline < TODAY && !ENDED.includes(task.state)
}

export function mapTasks(tasks: PTask[], f: (t: PTask) => PTask): PTask[] {
  return tasks.map((x) => f({ ...x, subtasks: mapTasks(x.subtasks, f) }))
}

export function move(tasks: PTask[], id: string, state: PState): PTask[] {
  return mapTasks(tasks, (x) => (x.id === id ? { ...x, state } : x))
}

export function add(tasks: PTask[], title: string, repo: string): PTask[] {
  return [t(repo, title, 'Inbox', { id: `new${Date.now()}` }), ...tasks]
}

export function findTask(tasks: PTask[], id: string): PTask | undefined {
  for (const x of tasks) {
    if (x.id === id) return x
    const hit = findTask(x.subtasks, id)
    if (hit) return hit
  }
}

// What the Broker would propose for a breakdown — canned.
export const PROPOSALS = [
  'List what has to happen first',
  'Do the smallest piece',
  'Check it against the Acceptance',
]

export function findParent(tasks: PTask[], id: string): PTask | undefined {
  for (const x of tasks) {
    if (x.subtasks.some((c) => c.id === id)) return x
    const hit = findParent(x.subtasks, id)
    if (hit) return hit
  }
}
