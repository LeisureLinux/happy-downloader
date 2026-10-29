import { describe, expect, it } from 'vitest'
import { formatDateTime, formatTimestamp } from './utils'

// The zero time is what Go writes for a field nobody set: 0001-01-01. It is a
// valid date and renders as one, which is how an empty folder came to have a
// download time in the year one.
const zeroTime = '0001-01-01T00:00:00Z'

describe('formatDateTime', () => {
  it('shows nothing for a time that was never set', () => {
    expect(formatDateTime(zeroTime)).toBe('—')
  })

  it('shows nothing for a missing or unusable time', () => {
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime('')).toBe('—')
    expect(formatDateTime('not a date')).toBe('—')
  })

  it('shows a real time', () => {
    expect(formatDateTime('2026-10-29T15:40:07+08:00')).not.toBe('—')
  })
})

describe('formatTimestamp', () => {
  it('shows nothing for the zero time', () => {
    expect(formatTimestamp(zeroTime)).toBe('—')
  })

  it('is the same shape for a real time', () => {
    expect(formatTimestamp('2026-10-29T15:40:07+08:00')).toMatch(
      /^2026-10-29 \d\d:\d\d$/,
    )
  })
})
