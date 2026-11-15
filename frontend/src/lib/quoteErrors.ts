// A failed quote shows Go's raw error where the total would be. Cashiers read
// `invalid custom price for item "HQ-CLZ-0005"` as the till being broken, so
// the known cart-input errors become a calm sentence saying what to check.
// An error we don't recognise is shown as it came: never a guessed reason.
const ITEM_ERRORS: Array<[RegExp, (code: string) => string]> = [
  [/^invalid custom price for item "(.*)"$/, (c) => `Check the price of ${c}`],
  [/^invalid unit price for item "(.*)"$/, (c) => `Check the prices of ${c}`],
  [/^unit prices do not match quantity for item "(.*)"$/, (c) => `Check the prices of ${c}`],
  [/^invalid unit discount for item "(.*)"$/, (c) => `Check the discount on ${c}`],
  [/^unit discounts do not match quantity for item "(.*)"$/, (c) => `Check the discount on ${c}`],
  [/^invalid or duplicate item "(.*)"$/, (c) => `Check the quantity of ${c}`],
]

export function friendlyQuoteError(raw: string): string {
  const message = raw.trim()
  for (const [pattern, say] of ITEM_ERRORS) {
    const m = pattern.exec(message)
    if (m) return say(m[1] || 'an item')
  }
  return message
}
