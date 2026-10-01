// @vitest-environment jsdom

// The buzz a hold gives. jsdom has no haptics, so these check which way the
// buzz is asked for rather than that anything is felt.

import { afterEach, expect, test, vi } from 'vitest'

import { buzz } from './hold'

afterEach(() => {
  vi.restoreAllMocks()
  Reflect.deleteProperty(navigator, 'vibrate')
})

test('a phone that can vibrate is asked to', () => {
  const vibrate = vi.fn()
  Object.defineProperty(navigator, 'vibrate', {
    value: vibrate,
    configurable: true,
  })
  const click = vi.spyOn(HTMLLabelElement.prototype, 'click')
  buzz()
  expect(vibrate).toHaveBeenCalled()
  expect(click).not.toHaveBeenCalled()
})

test('without vibrate, a hidden switch is toggled through its label', () => {
  const toggled: HTMLInputElement[] = []
  vi.spyOn(HTMLLabelElement.prototype, 'click').mockImplementation(function (
    this: HTMLLabelElement,
  ) {
    const input = this.querySelector('input')
    if (input) toggled.push(input)
  })
  buzz()
  expect(toggled).toHaveLength(1)
  expect(toggled[0]!.type).toBe('checkbox')
  expect(toggled[0]!.hasAttribute('switch')).toBe(true)
  expect(document.querySelector('input[switch]')).toBeNull()
})
