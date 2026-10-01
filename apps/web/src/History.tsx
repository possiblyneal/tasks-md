// A Task's history in the panel: each write the server found for it in the
// Repo's tasks history, newest first, with its Actor. Which commits belong to
// the Task is the server's to decide, because only it reads `.tasks.git`.

import { fetchHistory, type Entry } from './state'
import { useRead } from './read'

export function History({
  repo,
  id,
  version,
}: {
  repo: string
  id: string
  /** The Task's tree as the read last saw it, so a write to it reads again. */
  version: string
}) {
  const { value, error } = useRead<Entry[] | null>(
    () => fetchHistory(repo, id),
    null,
    [repo, id, version],
  )

  return (
    <>
      <h3 className="heading">History</h3>
      {error && <p className="aside">{error}</p>}
      {value?.length === 0 && (
        <p className="aside">Nothing has happened to it yet.</p>
      )}
      {value && value.length > 0 && (
        <ul className="list" aria-label="History">
          {value.map((e) => (
            <li key={`${e.at}:${e.subject}`}>
              {e.subject}
              <span className="facts aside">
                {/* The host's own clock, as the commit wrote it. */}
                <span>{e.at.slice(0, 16).replace('T', ' ')}</span>
                <span>{e.actor || 'Actor unknown'}</span>
              </span>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
