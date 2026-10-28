import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { rupiah, timeAgo } from './format'

describe('rupiah', () => {
  it('formats with Indonesian grouping and no decimals', () => {
    expect(rupiah(1234567).replace(/\s/g, ' ')).toBe('Rp 1.234.567')
    expect(rupiah(0).replace(/\s/g, ' ')).toBe('Rp 0')
  })
})

describe('timeAgo', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-10T12:00:00Z'))
  })
  afterEach(() => vi.useRealTimers())

  it('handles null', () => {
    expect(timeAgo(null)).toBe('-')
  })
  it('formats minutes, hours and days', () => {
    expect(timeAgo('2026-01-10T11:59:40Z')).toBe('just now')
    expect(timeAgo('2026-01-10T11:15:00Z')).toBe('45m ago')
    expect(timeAgo('2026-01-10T10:00:00Z')).toBe('2h ago')
    expect(timeAgo('2026-01-07T12:00:00Z')).toBe('3d ago')
  })
})
