import { describe, expect, it } from 'vitest'
import { friendlyQuoteError } from './quoteErrors'

describe('friendlyQuoteError', () => {
  it('turns the custom price error into a calm hint', () => {
    expect(friendlyQuoteError('invalid custom price for item "HQ-CLZ-0005"')).toBe('Check the price of HQ-CLZ-0005')
  })
  it.each([
    ['invalid unit price for item "A1"', 'Check the prices of A1'],
    ['unit prices do not match quantity for item "A1"', 'Check the prices of A1'],
    ['invalid unit discount for item "A1"', 'Check the discount on A1'],
    ['unit discounts do not match quantity for item "A1"', 'Check the discount on A1'],
    ['invalid or duplicate item "A1"', 'Check the quantity of A1'],
    ['invalid custom price for item ""', 'Check the price of an item'],
  ])('%s', (raw, want) => expect(friendlyQuoteError(raw)).toBe(want))
  it('leaves unknown errors untouched', () => {
    expect(friendlyQuoteError('  something else  ')).toBe('something else')
  })
})
