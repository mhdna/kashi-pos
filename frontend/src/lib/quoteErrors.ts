import { t } from "../i18n";
// A failed quote shows Go's raw error where the total would be. Cashiers read
// `invalid custom price for item "HQ-CLZ-0005"` as the till being broken, so
// the known cart-input errors become a calm sentence saying what to check.
// An error we don't recognise is shown as it came: never a guessed reason.
const ITEM_ERRORS: Array<[RegExp, (code: string) => string]> = [
  [/^invalid custom price for item "(.*)"$/, (c) => t("Check the price of {code}", { code: c })],
  [/^invalid unit price for item "(.*)"$/, (c) => t("Check the prices of {code}", { code: c })],
  [/^unit prices do not match quantity for item "(.*)"$/, (c) => t("Check the prices of {code}", { code: c })],
  [/^invalid unit discount for item "(.*)"$/, (c) => t("Check the discount on {code}", { code: c })],
  [/^unit discounts do not match quantity for item "(.*)"$/, (c) => t("Check the discount on {code}", { code: c })],
  [/^invalid or duplicate item "(.*)"$/, (c) => t("Check the quantity of {code}", { code: c })],
]

export function friendlyQuoteError(raw: string): string {
  const message = raw.trim()
  for (const [pattern, say] of ITEM_ERRORS) {
    const m = pattern.exec(message)
    if (m) return say(m[1] || t("an item"))
  }
  return message
}
