// A long press, on touch only. A mouse has a right button and a drag for what
// a finger can only say by holding still, so a press from one is left alone.

import { type MouseEvent, type PointerEvent, useEffect, useRef } from 'react'

/** How long a finger stays down before the press is a hold. */
export const HOLD_MS = 450

/** How far a finger may wander and still be holding rather than scrolling. */
const SLOP_PX = 10

/**
 * A short buzz. Where `navigator.vibrate` is missing, as on iOS Safari, a
 * hidden `<input type="checkbox" switch>` is toggled through its label, which
 * iOS 18 answers with the system haptic. jsdom cannot feel either, so whether
 * the iPhone buzzes, from a timer rather than the touch itself, needs a device
 * to confirm.
 */
export function buzz() {
  if (navigator.vibrate) {
    navigator.vibrate(15)
    return
  }
  const label = document.createElement('label')
  label.style.display = 'none'
  label.ariaHidden = 'true'
  const toggle = document.createElement('input')
  toggle.type = 'checkbox'
  toggle.setAttribute('switch', '')
  label.append(toggle)
  // In the head, outside the root React listens on, so no handler on the board
  // sees the click.
  document.head.append(label)
  label.click()
  label.remove()
}

/**
 * The handlers that make an element answer a held finger by calling `onHold`.
 * The hold buzzes where the phone can, and the click a phone sends when the
 * finger lifts afterwards goes no further than the element, so whatever a tap
 * on it opens is not also opened. Spread them on the element.
 */
export function useHold(onHold: () => void) {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const start = useRef({ x: 0, y: 0 })
  const held = useRef(false)

  const cancel = () => clearTimeout(timer.current)
  useEffect(() => cancel, [])

  return {
    onPointerDown(event: PointerEvent) {
      held.current = false
      if (event.pointerType !== 'touch') return
      start.current = { x: event.clientX, y: event.clientY }
      cancel()
      timer.current = setTimeout(() => {
        held.current = true
        buzz()
        onHold()
      }, HOLD_MS)
    },
    onPointerMove(event: PointerEvent) {
      const moved = Math.hypot(
        event.clientX - start.current.x,
        event.clientY - start.current.y,
      )
      if (moved > SLOP_PX) cancel()
    },
    onPointerUp: cancel,
    onPointerCancel: cancel,
    onPointerLeave: cancel,
    // The menu a phone opens on a long press would cover the sheet.
    onContextMenu(event: MouseEvent) {
      event.preventDefault()
    },
    // Capture, so the click is stopped before anything under or around the
    // element sees it.
    onClickCapture(event: MouseEvent) {
      if (!held.current) return
      held.current = false
      event.preventDefault()
      event.stopPropagation()
    },
  }
}
