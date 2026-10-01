// Move to: the six States as a sheet from the bottom of the screen, which is
// how a card is moved on a phone, where a drag between lanes scrolls instead.

import { named, STATES, type State } from './state'

export function MoveTo({
  title,
  from,
  onMove,
  onCancel,
}: {
  /** The Task being moved, so the sheet says which. */
  title: string
  /** Its State now, greyed: moving a Task to where it is moves nothing. */
  from: State
  onMove: (to: State) => void
  onCancel: () => void
}) {
  return (
    <div className="scrim" onClick={onCancel}>
      <div
        className="move-to"
        role="dialog"
        aria-label="Move to"
        onClick={(event) => event.stopPropagation()}
      >
        <h2>Move to</h2>
        <p className="lines">{title}</p>
        {STATES.map((state) => (
          <button
            type="button"
            key={state}
            disabled={state === from}
            onClick={() => onMove(state)}
          >
            {named(state)}
          </button>
        ))}
        <button type="button" className="cancel" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  )
}
