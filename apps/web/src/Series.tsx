// A Task's Series in the panel: the rule as the file says it, and the next
// dates it falls on, which the server works out because the rule arithmetic is
// its alone. Nothing here edits; `tasks repeat` is how a rule is set.

import { fetchSeries, type Series as Read } from './state'
import { useRead } from './read'

const NONE: Read = { rule: '', dates: [] }

export function Series({
  repo,
  id,
  rule,
}: {
  repo: string
  id: string
  /** The rule the read carries, drawn before the dates arrive. */
  rule: string
}) {
  const { value, error } = useRead(() => fetchSeries(repo, id), NONE, [
    repo,
    id,
    rule,
  ])

  return (
    <>
      {rule}
      {error && <p className="aside">{error}</p>}
      {value.dates.length > 0 && (
        <ul className="list facts" aria-label="Next dates">
          {value.dates.map((date) => (
            <li key={date}>{date}</li>
          ))}
        </ul>
      )}
    </>
  )
}
