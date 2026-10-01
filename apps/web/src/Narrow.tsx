// The controls over the board: Repo chips, Tag chips, one search box, one
// sort picker and one Blocked picker. Nothing here filters anything. Each control sets one field of
// the Narrowing and the next read asks the server again, so the lanes hold
// what `tasks list` would print under the same flags rather than what this
// side sifted.

import type { Board, Narrowing } from './state'

export function Narrow({
  narrowing,
  wide,
  onChange,
}: {
  narrowing: Narrowing
  /**
   * The read narrowed by nothing. The chips are drawn from it rather than from
   * the narrowed read, so narrowing never takes away the chip that undoes it.
   */
  wide: Board
  onChange: (narrowing: Narrowing) => void
}) {
  const tags = [
    ...new Set(wide.repos.flatMap((r) => r.tasks.flatMap((t) => t.tags))),
  ].sort()

  return (
    <div className="narrow" role="search">
      <input
        type="search"
        className="control"
        aria-label="Search"
        placeholder="Search"
        value={narrowing.search}
        onChange={(event) =>
          onChange({ ...narrowing, search: event.target.value })
        }
      />
      {/* A native select, because a phone already knows how to open one under
          a thumb. File order is the read's own, and the bare path. */}
      <select
        className="control"
        aria-label="Sort"
        value={narrowing.sort || 'file'}
        onChange={(event) =>
          onChange({ ...narrowing, sort: event.target.value })
        }
      >
        {wide.sorts.map((sort) => (
          <option key={sort} value={sort}>
            {sort}
          </option>
        ))}
      </select>
      {/* Blocked Tasks with the rest, alone, or left out: `tasks list` with
          neither flag, `-blocked` or `-unblocked`. */}
      <select
        className="control"
        aria-label="Blocked"
        value={narrowing.blocked ? 'only' : narrowing.unblocked ? 'out' : ''}
        onChange={(event) =>
          onChange({
            ...narrowing,
            blocked: event.target.value === 'only',
            unblocked: event.target.value === 'out',
          })
        }
      >
        <option value="">with Blocked</option>
        <option value="only">Blocked only</option>
        <option value="out">without Blocked</option>
      </select>

      {/* One Repo at a time, because a read names one Repo or every Repo:
          tapping another switches to it, and tapping the one on lets go. */}
      <div className="chips" role="group" aria-label="Repos">
        {wide.repos.map((repo) => {
          const on = narrowing.repo === repo.name
          return (
            <button
              type="button"
              className="chip"
              key={repo.name}
              aria-pressed={on}
              style={repo.color ? { borderColor: repo.color } : undefined}
              onClick={() =>
                onChange({ ...narrowing, repo: on ? '' : repo.name })
              }
            >
              {repo.name}
              {repo.problems.length > 0 && (
                <span className="flag">
                  <span aria-hidden="true"> !</span>
                  <span className="unseen">, has problems</span>
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* Any of them: a second Tag widens what is shown rather than narrowing
          twice, as `-tag` repeated does. */}
      {tags.length > 0 && (
        <div className="chips" role="group" aria-label="Tags">
          {tags.map((tag) => {
            const on = narrowing.tags.includes(tag)
            return (
              <button
                type="button"
                className="chip"
                key={tag}
                aria-pressed={on}
                onClick={() =>
                  onChange({
                    ...narrowing,
                    tags: on
                      ? narrowing.tags.filter((t) => t !== tag)
                      : [...narrowing.tags, tag],
                  })
                }
              >
                #{tag}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
