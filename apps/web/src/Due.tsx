/**
 * A Deadline, marked against the host's today as the read served it: `due
 * today` on the day, `overdue` once it has passed. Both are `YYYY-MM-DD`, so
 * comparing them as strings is comparing the dates.
 */
export function Due({ on, today }: { on: string; today: string }) {
  if (on === today) return <span className="today">due today</span>
  if (on < today) return <span className="overdue">overdue {on}</span>
  return <span>due {on}</span>
}
