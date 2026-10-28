import { describe, expect, it } from 'vitest'
import { buildOrderItems, parsePositiveInt, parsePrice } from './validate'

describe('parsePositiveInt', () => {
  it('accepts positive integers', () => {
    expect(parsePositiveInt('1')).toBe(1)
    expect(parsePositiveInt(' 25 ')).toBe(25)
  })
  it('rejects zero, negatives, decimals, text and empty', () => {
    for (const bad of ['0', '-3', '1.5', '1e3', 'abc', '', '  ']) {
      expect(parsePositiveInt(bad)).toBeNull()
    }
  })
  it('rejects values beyond int32', () => {
    expect(parsePositiveInt('2147483648')).toBeNull()
  })
})

describe('parsePrice', () => {
  it('allows zero but not negatives or decimals', () => {
    expect(parsePrice('0')).toBe(0)
    expect(parsePrice('12500')).toBe(12500)
    expect(parsePrice('-1')).toBeNull()
    expect(parsePrice('9.99')).toBeNull()
    expect(parsePrice('')).toBeNull()
  })
})

describe('buildOrderItems', () => {
  it('returns items for a valid order', () => {
    const r = buildOrderItems('Budi', [{ sku: ' SKU-001 ', qty: '2' }, { sku: 'SKU-002', qty: '1' }])
    expect(r.errors).toEqual([])
    expect(r.items).toEqual([{ sku: 'SKU-001', qty: 2 }, { sku: 'SKU-002', qty: 1 }])
  })
  it('requires a customer', () => {
    expect(buildOrderItems('  ', [{ sku: 'A', qty: '1' }]).errors).toHaveLength(1)
  })
  it('flags empty sku, bad qty and duplicates', () => {
    const r = buildOrderItems('x', [
      { sku: '', qty: '1' },
      { sku: 'A', qty: '0' },
      { sku: 'B', qty: '1' },
      { sku: 'B', qty: '2' },
    ])
    expect(r.errors).toHaveLength(3)
    expect(r.items).toEqual([{ sku: 'B', qty: 1 }])
  })
})
