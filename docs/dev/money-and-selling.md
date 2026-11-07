# Money, prices and selling

Code: `money.go`, `currencies.go`, `pricing_overrides.go`, `tx_invoice.go`,
`invoice_quote.go`, `deals.go`, `invoice_discounts.go`, `oversell.go`,
`tx_exchange.go`, `deal_exchange.go`, `invoice_lbp.go`.

## How amounts are stored

- **Every amount is an integer in cents of the default currency (USD).**
  Invoice totals, line prices, payments, shift balances: all USD cents, whatever
  the customer paid with. There is no floating-point money.
- Exchange rates are `currencies.units_per_usd_micros`: how many millionths of
  the currency make one USD, so rates both above and below the dollar fit an
  integer.
- Never compare currency codes to a literal. `resolveCurrency` looks a
  currency up by code and falls back to the default currency, so a currency
  added in Settings works everywhere without code changes.
- `products.currency_code` is unreliable legacy data (thousands of USD
  products are tagged `LBP`). Never convert with it.
- Expenses and loans are the exception: stored in **their own currency, x100**,
  as entered.

### LBP

The till shows and prints an LBP total next to USD: the LBP rate, rounded to
the LBP cash unit with the branch's rounding mode (`invoiceLBPTotal`, the same
rounding the receipt uses). A refund rounds its size and keeps its sign.
`auto_append_lbp_zeros` (off by default) adds the trailing zeros when typing
LBP amounts.

## Where a price comes from

- **Since migration 000097 a price is not part of the product.** It's an entry
  in a price list that kashi decides and the sync mirrors down.
  `products.price` and `products.discount` still exist but are frozen: nothing
  reads them.
- A product in no active price list **has no price**, and checkout refuses a
  zero price. That's deliberate: kashi decides what can be sold. "Can't sell
  this item" at a till usually means it's missing from the branch's price list.
- Price and discount lists are read-only on the till.

## Pricing a cart

- `priceInvoiceTotals` is the one pricing path. Checkout (`salesInvoiceTx`) and
  the live preview (`InvoiceQuote`, the calculator's total) both call it, so
  the total shown can't differ from the total charged.
- **Deals** ("Buy 2 get 1", "2nd 50% off", "3 for $50", "3 or more 20% off"):
  a discount list row names the deal a variant is on; every unit in the cart on
  the same deal is pooled whatever its product, and the **cheapest** units get
  the reward. A deal replaces a discount: deal units are priced from the list
  price. The line stores what the deal took off (`deal_discount`) and the
  invoice keeps a copy of the deal's terms (`invoice_deals`), so an exchange is
  judged by what was sold, not by today's deal.
- **Invoice discounts** ("$150 or more, 15% off"): checked on the total after
  item discounts and deals, before tax, and taken off on top of them. When
  several apply, the one that saves most wins; they never add up. The discount
  is spread over the lines in proportion to their cost
  (`invoice_products.invoice_discount`), so a return credits what the line was
  actually paid. Exchanges never earn one.
- **Discount/deal toggles** (`discount_toggle`, `deal_toggle`): let the
  salesperson turn a line's discount or deal off. `discount_toggle_default_off`
  makes lines start with the discount off. Off by default; kashi can push them.
- Price edits at the till can be restricted to certain product codes
  (`price_edit_restricted` + the code list), and custom prices/discounts are
  separate switches (`custom_item_prices`, `custom_item_discounts`,
  `per_unit_item_prices`, `price_change_manual_override_mode`).
- **kashi's live cart overrides** (`live_cart.go`) beat all of the above for
  the one sale they were made for: a price, a discount or no deal per line,
  or every deal off. See `sync.md`, Live cart.

## Stock

- **Checkout doesn't take stock off.** The deduction block in `salesInvoiceTx`
  is disabled; stock is kashi's job, from the synced sales.
- `invoices.inventory_id` is only a label. An unset or unknown sales
  inventory must never stop a sale; the worst case is a wrongly labelled
  invoice. Only a database with no inventories at all can't sell. If stock
  deduction is ever turned back on, this needs revisiting.
- **Selling beyond stock** (`settings.oversell_mode`, set by kashi per
  branch): `allow` (sell; kashi flags the negative stock), `warn` (the
  cashier confirms first) or `block` (refuse). "Stock" is kashi's on-hand for
  the branch minus this till's own sales kashi hadn't received yet. A product
  kashi reports nothing for counts as 0. If the till has no stock data for the
  branch at all, nothing is checked: a till must never refuse a sale for lack
  of data.

## Payment methods

- Payment methods are cashbox accounts, owned by kashi. Retiring one
  deactivates it; it's never deleted, because invoices and shift balances
  point at it.
- `On Account` (client credit) and `On Coupon` are internal placeholders that
  the code finds by exact name. Don't rename them.
- `overchange_confirmation` (on by default): ask before giving more change than
  makes sense. `settings.max_overpayment_cents` caps overpayment.
- Revenue is `SUM(invoices.grand_total)`, not the sum of payments, because
  payment entries include change rounding and unrecorded cash overpayment.

## Returns and exchanges

- An exchange records the returned lines and the replacements; the customer
  only pays the positive difference.
- Returned lines are stored with **negative** quantity and line total;
  replacements stay positive. Sum them as stored; don't flip signs. Return
  and exchange invoices have the `EX/` and `RS/` prefixes.
- Only quantities not already returned can be returned again.
- **What an exchange returns can change while it's on screen**: in exchange
  mode the toolbar's exchange button opens "Returned items"
  (`ExchangeReturnsDialog.vue`, `GetExchangeInvoiceByID`). It shows the sale's
  items with a count each, from 0 ("not returned", left on the sale) up to what
  the sale has left. At least one unit stays returned (cancel the exchange to
  return nothing). Returned cart lines are built in one place
  (`lib/exchangeReturns.ts`), for the dialog and the picker alike.
- **An exchange can be held** like a sale (a settlement can't). The hold
  snapshot carries `exchange: {originalInvoiceId, originalInvoiceCode}`, and
  restoring puts the lines back as held: never through `addItem`, which would
  turn a returned line into a unit for sale (`frontend/src/lib/holdSnapshot.ts`).
  A hold's returned lines without a sale are dropped. Nothing is reserved while
  it's held, so the quote re-checks the sale when it comes back.
- **A product can sit on several lines of one sale**: a deal keeps its paid
  and free units apart, and the same product scanned at two prices stays
  apart (`invoice_products` has had its own id since 000061). Never match a
  returned item to "the" line of its product; spread the quantity over every
  matching line (`validateAndPriceExchange`, `executeRemoteReturnInvoiceCommand`).
- **Deal exchanges** (`deal_exchange.go`) have their own rules: a paid unit is
  credited what was paid; a free/reduced unit is credited what was paid plus
  "swap credit" up to its shelf price, which only buys replacements in the same
  exchange and is never paid out; and if the customer's remaining items no
  longer earn the deal saving they still enjoy, the difference is charged back.
  The amounts ride on the exchange's own lines, so lines still add up to the
  invoice.

  Worked examples. They come from a real Centro sale (SA/CT/2026/02923, Buy 1
  Get 1, 8 items, $71 paid; free: $12, $12, $9, $9) and were run through the
  till's code:

  | Customer returns | Takes | Lines | Pays |
  | --- | --- | --- | --- |
  | Jacket $29 + Hoodie $15 (paid) | Men Hoodie $19 + Jacket $29, **on the deal** | −29, −15, +19, +29 | $4: full credit, the new items keep the free ones paired |
  | Same | Skechers $35 + Crocs $19, **shoes, off the deal** | −17 (charged back $12), −15, +35, +19 | $22 |
  | Jacket, Hoodie, free Pants $9 (refund, RS/CT/2026/00068) | nothing | −29, −12 (charged back $3), 0 | −$41 cash |
  | Free Pants $9 | Men Hoodie $19 | −9 (swap credit), +19 | $10 |
  | Free Pants $9 | $5 socks | −5, +5 | $0, the other $4 is lost |

  The charge back is what the customer still enjoys free (kept free units,
  plus returned free units, which still count) minus what the deal earns on
  what they keep plus replacements on the deal. A returned free unit counting
  keeps it lenient: RS/CT/2026/00068 takes $3, where counting only the kept
  units would take $12. Covered by: shoes and Chinese items are off the deal,
  outlet (CA, EOC5, EF6) and Bangladesh (BOC5, BNV, N) are on, as kashi's
  discount list says. The cashier guide with these cases is
  `tools/kashi-admin/guides/gb-buy1get1-returns-guide.png`.
- **A return's refund can be edited from kashi** (Edit refund →
  `edit_return_invoice`, `return_edit.go`). It sets a returned line's credit,
  up to its price. The total, the payments of the main account and the shift
  drawer move by the difference, the deal's recorded charge back drops by what
  was given back, a note is added, and the edit syncs to kashi
  (`/branch/return_invoice_edits`). Lines are named by position, as kashi lists
  them.
- **The exchange screen shows the quote's amounts, never its own.**
  `QuoteExchangeTotal` returns one line per cart line sent, in order
  (`exchange_quote_lines.go`), and a breakdown. A returned line shows its
  credit, which is often not its shelf price: what was paid, plus any swap
  credit, minus any charge back. These always hold, and the quote errors
  rather than break them:

  | Identity | Checked in |
  | --- | --- |
  | `-credit = paid + swapCredit - chargedBack` per returned line | `exchangeQuoteLines`, `exchangeQuoteProblem` |
  | `sum(lines) = net = replacements - returned` | both |
  | `net + tax = unrounded`, `max(0, unrounded + rounding) = total` | both |

  Quote and checkout get the total from one function
  (`computeExchangeTotals`). The screen re-checks every quote
  (`frontend/src/lib/exchangeQuote.ts`) and shows an error, not a total, when
  one doesn't add up for the cart it was asked about. Real quotes are written to
  `frontend/src/lib/testdata/exchange_quotes.json` by `TestExchangeQuoteGolden`
  (rewrite with `KASHI_UPDATE_GOLDEN=1`) and checked by the frontend tests.
- **Return receipts.** A return or exchange prints only with receipts on, and
  then by its own choice (`receiptChoice` in `tx_exchange.go`): kashi's
  `print_receipt: false` on a remote return, or else the till's
  `print_return_receipts` display pref, which only kashi sets (branch settings,
  **Print return and exchange receipts**). The till has no switch or tick for it.
  An unprinted one can be reprinted from Invoices or kashi's **Print on till**.
  The rule is shared with the screen through
  `frontend/src/lib/testdata/return_receipt_rule.json` (`TestReturnReceiptRuleGolden`).
- An expense or loan entered by mistake is **cancelled, not deleted**
  (`records_cancel.go`): it stays listed as cancelled and stops counting.

## Unknown items

Something with no code here (`unknown_items.go`): the cashier sells it as a
placeholder product at a typed price; before payment a phone photographs
every unit (QR link from kashi), and the sale is held until each unit has a
picture, unless a kashi user with the right permission lets it through. Someone
in kashi names the items afterwards. `unknown_pictures_at_checkout` controls
the photo step.

Unknown items join the shop's deal (`unknown_item_deal.go`): the free-units
deal the most products are on in the lists in force, whatever its numbers,
pooled with every other unit on it. The salesperson picks one of three on each
line by tapping its deal chip: **deal else 30%**, **deal else 20%** (the default)
or **No deal**, with the picker naming the real deal; the
percent is what a unit outside a set gets, like a discount row's own fallback
(kashi 000216). This choice works whatever `deal_toggle` says. A
discount can be typed only with the branch's custom discounts on (its price can
always be typed), and it keeps the line off the deal (a deal never stacks on another saving),
and picking a deal clears it. With no product on such a deal, unknown items
sell at their typed price and show no chip. The cart sends the line's
`unknownRef` as `lineRef`, and the quote echoes it with the choice in force
(`unknownDeal`), since every unknown item is the same product.

## Coupons and client accounts

- Coupons and coupon types come from kashi.
- A client's receivable balance is built from the invoices that created it
  and the payments applied to them (`client_accounts.go`).
- `RoundInvoice` re-applies the current rounding rule to an existing invoice
  and fixes its payments, account entries and shift balance with it (admins
  only).

## The unfinished sale (draft)

The cart, client, note and calculator state are kept in `localStorage`
(`kashi-pos.invoice-draft`) so a restart or crash restores the sale. The write
is coalesced (`lib/draftSaver.ts`): about 0.4 s after the last change, or at
once when the page is hidden or closed. Emptying the cart (or finishing the
sale) deletes the draft immediately and cancels any save still waiting. Settlement
mode (paying off an old invoice) never saves a draft.
