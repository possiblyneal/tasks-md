// PROTOTYPE — throwaway. Floating variant switcher for issue #109.
import { useEffect } from 'react'

export function Switcher({
  variants,
  current,
  onChange,
}: {
  variants: { key: string; name: string }[]
  current: string
  onChange: (key: string) => void
}) {
  const i = Math.max(
    0,
    variants.findIndex((v) => v.key === current),
  )
  const go = (d: number) =>
    onChange(variants[(i + d + variants.length) % variants.length]!.key)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement
      if (el.closest('input, textarea, [contenteditable]')) return
      if (e.key === 'ArrowLeft') go(-1)
      if (e.key === 'ArrowRight') go(1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })

  if (import.meta.env.PROD) return null
  return (
    <div className="p-switcher">
      <button type="button" onClick={() => go(-1)}>
        ←
      </button>
      <span>
        {variants[i]!.key} — {variants[i]!.name}
      </span>
      <button type="button" onClick={() => go(1)}>
        →
      </button>
    </div>
  )
}
