import type { Week } from './state'

/**
 * The weekly metrics as the read counted them. This week is the line in
 * sight; the twelve open under it, newest first.
 */
export function Metrics({ weeks }: { weeks: Week[] }) {
  const newest = [...weeks].reverse()
  const now = newest[0]
  if (!now) return null
  return (
    <details className="metrics" role="group" aria-label="Metrics">
      <summary>
        {`This week: ${now.added} added · ${now.done} done · ${now.declined} declined`}
      </summary>
      <table>
        <thead>
          <tr>
            <th scope="col">Week of</th>
            <th scope="col">Added</th>
            <th scope="col">Done</th>
            <th scope="col">Declined</th>
          </tr>
        </thead>
        <tbody>
          {newest.map((week) => (
            <tr key={week.start}>
              <th scope="row">{week.start}</th>
              <td>{week.added}</td>
              <td>{week.done}</td>
              <td>{week.declined}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </details>
  )
}
