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
  in a price list that central decides and the sync mirrors down.
  `products.price` and `products.discount` still exist but are frozen: nothing
  reads them.
- A product in no active price list **has no price**, and checkout refuses a
  zero price. That's deliberate: central decides what can be sold. "Can't sell
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
  makes lines start with the discount off. Off by default; central can push them.
- Price edits at the till can be restricted to certain product codes
  (`price_edit_restricted` + the code list), and custom prices/discounts are
  separate switches (`custom_item_prices`, `custom_item_discounts`,
  `per_unit_item_prices`, `price_change_manual_override_mode`).
- **central's live cart overrides** (`live_cart.go`) beat all of the above for
  the one sale they were made for: a price, a discount or no deal per line
  (an unknown item's deal choice too), or every deal off. See `sync.md`, Live cart.

## Stock

- **Checkout doesn't take stock off.** The deduction block in `salesInvoiceTx`
  is disabled; stock is central's job, from the synced sales.
- `invoices.inventory_id` is only a label. An unset or unknown sales
  inventory must never stop a sale; the worst case is a wrongly labelled
  invoice. Only a database with no inventories at all can't sell. If stock
  deduction is ever turned back on, this needs revisiting.
- **Selling beyond stock** (`settings.oversell_mode`, set by central per
  branch): `allow` (sell; central flags the negative stock), `warn` (the
  cashier confirms first) or `block` (refuse). "Stock" is central's on-hand for
  the branch minus this till's own sales central hadn't received yet. A product
  Central reports nothing for counts as 0. If the till has no stock data for the
  branch at all, nothing is checked: a till must never refuse a sale for lack
  of data.

## Payment methods

- Payment methods are cashbox accounts, owned by central. Retiring one
  deactivates it; it's never deleted, because invoices and shift balances
  point at it.
- `On Account` (client credit) and `On Coupon` are internal placeholders that
  the code finds by exact name. Don't rename them.
- `overchange_confirmation` (on by default): ask before giving more change than
  makes sense. `settings.max_overpayment_cents` caps overpayment.
- Revenue is `SUM(invoices.grand_total)`, not the sum of payments, because
  payment entries include change rounding and unrecorded cash overpayment.
- **LL is netted in LL.** The calculator sends each LL entry's LL as counted
  (`amountUnits`) beside its cents. Checkout (and every payment quote) adds
  an LL account's payments and change up in LL and converts once, each entry
  recording its step in the running total (`settleCashEntryCents`,
  `tx_invoice.go`; the calculator mirrors it in `lib/cashPayments.ts`, checked
  against `lib/testdata/cash_payment_netting.json`). So LL 200,000 paid less
  LL 60,000 back records the 156¢ of LL 140,000, not 222¢ − 67¢. A change
  entry without its LL (an older till, a remote command) keeps its own cents.

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
- **A hold keeps the whole sale** (`lib/holdSnapshot.ts`): every line as it
  stood (custom and per-unit prices, discounts, toggles, notes, central's
  marks), client, salesperson, calculator, notes, the sale's `saleRef` and
  its unknown-item picture session. Restoring puts the lines back as held,
  never through `addItem`. `HoldInvoice` stores central's live-cart overrides
  of that sale with the hold; `ResumeHoldInvoice` puts them back in force and
  deletes the hold.
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
  outlet (CA, EOC5, EF6) and Bangladesh (BOC5, BNV, N) are on, as central's
  discount list says. The cashier guide with these cases is
  `tools/kashi-admin/guides/gb-buy1get1-returns-guide.png`.
- **A return's refund can be edited from central** (Edit refund →
  `edit_return_invoice`, `return_edit.go`). It sets a returned line's credit,
  up to its price. The total, the payments of the main account and the shift
  drawer move by the difference, the deal's recorded charge back drops by what
  was given back, a note is added, and the edit syncs to central
  (`/branch/return_invoice_edits`). Lines are named by position, as central lists
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
  then by its own choice (`receiptChoice` in `tx_exchange.go`): central's
  `print_receipt: false` on a remote return, or else the till's
  `print_return_receipts` display pref, which only central sets (branch settings,
  **Print return and exchange receipts**). The till has no switch or tick for it.
  An unprinted one can be reprinted from Invoices or central's **Print on till**.
  The rule is shared with the screen through
  `frontend/src/lib/testdata/return_receipt_rule.json` (`TestReturnReceiptRuleGolden`).
- An expense or loan entered by mistake is **cancelled, not deleted**
  (`records_cancel.go`): it stays listed as cancelled and stops counting.

## Unknown items

Something with no code here (`unknown_items.go`): the cashier sells it as a
placeholder product at a typed price; before payment a phone photographs
every unit (QR link from central), and the sale is held until each unit has a
picture, unless a central user with the right permission lets it through. Someone
in central names the items afterwards. `unknown_pictures_at_checkout` controls
the photo step.

The cashier types the price, then **names the item** on the on-screen keyboard
(`UnknownPriceDialog.vue`; a name is required, up to 80 letters). The price can
be retyped afterwards from the price cell's pad, even with custom prices off; the
typed price simply is the price, it never becomes a discount. The name is
saved on the line (`invoice_products.item_name`, only for the placeholder
product; a name equal to "Unknown item" counts as none) and is what the invoice
detail and list, and the receipt (including a reprint), call it. It is local to
the till: sync still sends the placeholder, and central names the picture its
own way. An exchange's list of what can be returned uses the same name.

**A scanned barcode the catalog doesn't know** opens the same dialog when it
is a real retail barcode (EAN-8, UPC-A, EAN-13 or GTIN-14 whose check digit
holds, `lib/unknownScan.ts`); anything else is just "not found". The dialog
says "Not in the catalog: <barcode>", the line keeps it (`unknownBarcode`) and
every unit of that line sends it to central as `barcode` on its slot. Central's
label reader then reads the picture's price label and, once the sale lands,
makes the product under that exact barcode (tagged "From photo", at the
cashier's price), so the next scan at any till finds it. See central's
`api/label_reader.go`.

Unknown items join the shop's deal (`unknown_item_deal.go`): the free-units
deal the most products are on in the lists in force, whatever its numbers,
pooled with every other unit on it. The salesperson picks one of three on each
line by tapping its deal chip: **deal else 30%**, **deal else 20%** (the default)
or **No deal**, with the picker naming the real deal; the
percent is what a unit outside a set gets, like a discount row's own fallback
(central 000216). This choice works whatever `deal_toggle` says. A
discount can be typed only with the branch's custom discounts on (its price can
always be typed), and it keeps the line off the deal (a deal never stacks on another saving),
and picking a deal clears it. With no product on such a deal, unknown items
sell at their typed price and show no chip. The cart sends the line's
`unknownRef` as `lineRef`, and the quote echoes it with the choice in force
(`unknownDeal`), since every unknown item is the same product.

## Coupons and client accounts

- Coupons and coupon types come from central.
- A client's receivable balance is built from the invoices that created it
  and the payments applied to them (`client_accounts.go`).
- `RoundInvoice` re-applies the current rounding rule to an existing invoice
  and fixes its payments, account entries and shift balance with it (admins
  only).

## Referrals

- Every client has a ==referral code== central gave them (`clients.referral_code`,
  e.g. `K7M4QH`). The alphabet leaves out characters that read like another
  (no 0/O, 1/I/L, 2/Z, 5/S, 8/B, U/V), and the last character is a ==check
  character==, so the till catches any one wrong character or two swapped
  neighbours before looking anything up (`referrals.go`, `lib/referralCode.ts`).
- The receipt prints the customer's code with a line saying what it gets them,
  only while central has referrals on at this branch (central switches them on
  per branch and feeds each branch its own `enabled`).
- A till that was too old to read codes when central handed them out kept its
  older clients without one; migration 000145 makes it fetch those clients
  again (it forgets their version and the clients cursor, and the next pull's
  completeness check fetches them).
- **The friend's sale:** the cashier types the code with the Referral button
  in the cart header. The till takes it only for a customer (not the walk-in)
  with ==no earlier sale here==, holding no reward, using someone else's code.
  ==The friend gets nothing.== The code goes to central on the sale
  (`referral_code`), and central checks again across every branch.
- **The owner's reward:** central sends it down (`referral_rewards`): a percent,
  maybe a cap and dates. On the owner's next sale it ==competes with the
  invoice discounts==: the one that takes more off wins, a tie goes to the
  rule, and the reward is ==used up only when it wins== (the sale's
  `referral_reward_uid`, `referral_rewards.used_invoice_id`). A pull can't
  make a reward used here usable again; cancelling the sale that used it does.
- Central's rules (on or off, percent, minimum spend, days, cap) live in kashi
  (`referral_program` on the till holds what it needs).

- ==Who a code rewards== is central's choice (`reward_to`): the client who
  shared it (`owner`, a percent off a later sale, as above), the new customer
  (`friend`, `friend_percent` off the sale they use it on) or `both`. The new
  customer's discount is one more invoice discount, "Referral 10%"
  (`friendReferralRule`): it competes with the deals or adds up with them
  (deals_stack), and its minimum is the program's minimum spend. The screen
  sends the code among the picks (`referral:CODE`) so the quote and the
  payment check price it; checkout uses only the sale's own code
  (`withReferralPick`), checked by `checkReferralCode`, so the discount never
  comes without the code being recorded. `TestReferralDiscountRandomized`.

- ==A code is good once==: a client's code prints on their receipts until a
  friend uses it (a referral central has for them, or a sale here that named
  it: `referralCodeUsed`), and the till refuses a used code; central refuses
  a second use too. The note's default wording follows who the program
  rewards (`referralNoteDefault`, `arabicReferralNote`), and the code's block
  prints white on black (`invertRect`).

## The Rewards button

- One button in the cart header, **Rewards**, holds a sale's rewards: the deals
  ==the cashier picks== (an invoice discount central marks `cashier_picks`, like
  "Students 15%"), the referral code, and the invoice discounts that ==apply by
  themselves==, listed for reference only (central or an admin changes them).
  The number by it counts what is on; it turns orange then.
- A picked deal is an ordinary invoice discount that counts only when its uid
  is in the sale's picks: it keeps its minimum and dates, and ==competes== with
  the automatic ones and the referral reward, the biggest winning
  (`bestInvoiceDiscount`). The picks go to the quote, the payment check and
  checkout alike (`QuoteInvoiceTotalPicked`, `QuotePaymentStatusPicked`,
  `CheckoutInvoicePicked`); `TestPickedDiscountsRandomized` checks they agree.
- The cashier picks ==one deal at a time==: picking another replaces it (it
  takes over), since only the biggest discount ever comes off the invoice.
- Unless the branch lets ==deals add up== (central's `deals_stack`, a feature
  flag here): then every deal that applies comes off, one after the other
  (`invoiceDiscountsStacked`). Percents first, biggest first, each on what is
  left; then fixed amounts, biggest first, never past what is left; same size
  by name. Each minimum is judged on the sale before any deal. The cashier can
  pick several. The invoice keeps one deal: the names joined with " + " and the
  total off, so returns, exchanges, sync and central's reports work as for one
  deal. A referral reward still competes with that total. On the receipt the
  names break onto their own lines when they don't fit beside the amount
  (`splitDealWords`). `TestStackedDealsRandomized` checks quote, checkout and
  what is stored against the rule written plainly.
- A new sale starts with nothing picked; a deal central takes away drops out
  of the picks.

## Receipt wording

- Central's Receipt Designer sets `settings.receipt_texts` (JSON, `receipt_texts.go`):
  the payment, exchange policy, referral and "Follow us on:" titles, the referral
  line (`{percent}` is the reward) and the thank-you line, plus the ==parts left
  out== (payments, policy, referral, socials, thanks). Empty is every default.
- The designer can't run this renderer, so `make receipt-previews` draws every
  look in five ==modes== (sale, referral, client, exchange, return) and writes
  where each part landed (`receipt_marks.go`) into the catalog. Central lays its
  editable text over those bands and closes a band when a part is left out.
- Beyond those, ==every line is an element== (`receipt_design.go`): store name,
  branch, phone, invoice title per kind, website, the customer, salesperson and
  date labels, the column titles, every totals label, the payment, policy,
  referral, socials and thank-you lines. Each can take its own words, capitals
  (upper, lower, First Letters), size, bold, alignment and be left out, as far
  as `receiptElements` allows for it; `date_format` picks the date. All of it
  lives in `receipt_texts.elements`; nothing set prints exactly as before
  (checked against every preview picture).
- ==What each item row prints== is four elements of the Items part:
  `item_name`, `item_variant` (colour and size) and `item_price_note` ("Was
  $18.00, now $14.40") print unless left out (`hide`); `item_code` (the code,
  or the barcode when there's none, under the name) prints only when switched
  on (`show`, the `canShow` ability). `itemsAsPrinted` applies them once, before
  any layout or the text receipt draws, so every look obeys them alike; a row
  whose name is left out shows its code in its place when the code is on. The
  Supermarket "With codes" look always shows codes. Amounts never change.
- A whole ==part== can take capitals, size, bold and alignment at once
  (`groups`); a line's own setting wins. Each part can have a line above, its
  divider swapped or taken away (`below: "none"`) and a box, dotted, dashed,
  solid or double, 1 to 4 thick, square or rounded (`decor`,
  `receipt_decor.go`). The parts under the totals print in `order`. Nothing set
  is the look exactly as before.
- ==Fonts==: any line that can change size, the column titles and TOTAL can
  also take a `font` from `receiptFonts` (`receipt_fonts.go`): the looks' own
  fonts plus 16 extra open-licence families (sans, serif, mono, display,
  script; Latin only, in `assets/fonts/extra`). A part's `font` sets all its
  lines, a line's own wins, a banner's sets its title. Words on a row (column
  titles, TOTAL) find their font through `pendingFont` when measured and
  drawn. Every line mark says which font it was drawn in, so central types
  over it in the same letters (it serves the files from `ReceiptFontFile`).
- ==Arabic==: a line with words that can change size can be English (default),
  Arabic (`lang: "ar"`) or both (`"both"`, the Arabic under the English),
  per line or for a whole part (`groups`). Its Arabic is the branch's own
  (`text_ar`) or `arabicDefaults`, in Lebanese Arabic, swapped in for the
  English words so a name, date or code after them stays; `{percent}` prints
  in Arabic digits (١٥٪); the shop's own policy has no default Arabic. Arabic
  lines take the look's alignment mirrored and an Arabic font (Vazirmatn
  unless the line's font is an Arabic one); rows put the amount on the left.
  The drawing places one glyph per character, so `receipt_arabic.go` joins
  the letters (Presentation Forms-B, lam-alef) and orders the line right to
  left, Latin words and numbers inside kept left to right; vowel signs are
  left out. The text receipt stays English (its code page has no Arabic).
- ==Banners== (`banners`, `receipt_banner.go`, up to 4) are display lettering
  the till draws in black and white so they print crisp: ribbon, burst, stamp,
  frame, ornament or stars, a title and a line under it, in the looks' own
  fonts, shrunk to fit the paper. `when` is empty (always), `item_deal` (a line
  has a buy X get Y deal), `deal` (a deal on the invoice) or `deal:<name>` (that
  deal, on the invoice or a line); `place` is under the shop's name or
  `bottom`. A branch keeps up to 12; ==one prints== per receipt
  (`chosenBanner`): the banner for the biggest deal on it (its place in a
  stacked deal name), then one for a deal on the items, buy X get Y, any
  invoice deal, then always; the first of equals.
  More conditions: `item_discount` (an item on sale with its own discount)
  and `referral` (a customer with a code to share); a till leaves out a
  banner whose condition it doesn't know (older tills printed it always).
  `{deal}` in a banner's words prints the deal's name (the title in capitals).
  `receiptBannerPresets` are the designer's ready-made ones, and
  `RenderBannerPreview` draws one alone for its gallery.
- The renderer builds on its own (`scripts/receipt-render-files.txt`,
  `TestReceiptRendererBuildsAlone`): central copies it to draw the designer's
  live receipt with the same code.
- That only holds if leaving a part out prints ==exactly the receipt with its
  band cut==: `TestReceiptMarksHidingCutsTheBand` checks it pixel for pixel on
  every look. A hidden thank-you line takes the room under it along.

## The unfinished sale (draft)

The cart, client, note and calculator state are kept in `localStorage`
(`kashi-pos.invoice-draft`) so a restart or crash restores the sale. The write
is coalesced (`lib/draftSaver.ts`): about 0.4 s after the last change, or at
once when the page is hidden or closed. Emptying the cart (or finishing the
sale) deletes the draft immediately and cancels any save still waiting. Settlement
mode (paying off an old invoice) never saves a draft.
