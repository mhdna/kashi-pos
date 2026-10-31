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
- **Deal exchanges** (`deal_exchange.go`) have their own rules: a paid unit is
  credited what was paid; a free/reduced unit is credited what was paid plus
  "swap credit" up to its shelf price, which only buys replacements in the same
  exchange and is never paid out; and if the customer's remaining items no
  longer earn the deal saving they still enjoy, the difference is charged back.
  The amounts ride on the exchange's own lines, so lines still add up to the
  invoice.
- An expense or loan entered by mistake is **cancelled, not deleted**
  (`records_cancel.go`): it stays listed as cancelled and stops counting.

## Unknown items

Something with no code here (`unknown_items.go`): the cashier sells it as a
placeholder product at a typed price; before payment a phone photographs
every unit (QR link from kashi), and the sale is held until each unit has a
picture, unless a kashi user with the right permission lets it through. Someone
in kashi names the items afterwards. `unknown_pictures_at_checkout` controls
the photo step.

## Coupons and client accounts

- Coupons and coupon types come from kashi.
- A client's receivable balance is built from the invoices that created it
  and the payments applied to them (`client_accounts.go`).
- `RoundInvoice` re-applies the current rounding rule to an existing invoice
  and fixes its payments, account entries and shift balance with it (admins
  only).
