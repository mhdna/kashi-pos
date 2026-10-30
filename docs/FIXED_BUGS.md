# Fixed bugs

One entry per bug fixed in kashi-pos, newest first: what went wrong, why,
and what changed. Add an entry in the same change as the fix (see AGENTS.md).

## 2026-10-30 · "Saved info" popup over the till's fields

**Symptom.** Tapping a field (the cart's quantity box, say) opened a
Windows "Saved info" popup listing old values like "0" on top of the cart.

**Cause.** WebView2, which the till runs in, remembers what was typed into
each field and offers it back. No field asked it not to, and Wails has no
option to turn WebView2's autofill off.

**Fix.** `frontend/src/lib/noAutofill.ts`, called from `main.ts`, marks
every input and textarea `autocomplete="off"`, including ones added later
(dialogs, menus), through a MutationObserver. No automated test: the
frontend tests have no DOM; checked with the build.

## 2026-10-30 · make bump tagged releases that couldn't be published

**Symptom.** `make bump; make publish-all-force` run twice made v1.1.66,
v1.1.67 and v1.1.68, and each publish failed to build, so none shipped.

**Cause.** `make bump` tagged whatever was committed, with no checks, while
`make publish` builds what's on disk, uncommitted work included. Work in
progress that didn't compile yet was in the tree, and a second run bumped
again with nothing new.

**Fix.** `make bump` now refuses, tagging nothing, when there are
uncommitted changes, when nothing was committed since the last release, or
when `go build` fails (`Makefile`). The three unpublished tags were removed.

## 2026-10-30 · Mixed carts got no free item on B2G1

**Symptom.** 1 Kids Boy Jeans (BOC5…) + 2 Kids Girl Jeans (CA…) at $15 on
"Buy 2 get 1 free" came to $33 with nothing free: the boy's at 20% off, the
girl's at 30%.

**Cause.** The outlet codes (CA, EOC5, EF6) were to get 30% on units left
outside a set and everything else 20%, and a deal has one fallback percent,
so they were put on two deals. The till pools units per deal, so the two
kinds never made a set together.

**Fix.** One deal again, with each list row able to carry its own fallback
(`deal_fallback_percent`, kashi 000216, till 000133). The till pools every
unit on the deal and gives a unit left outside a set its row's fallback
(`unitDiscounts` and `applyDeals` in `deals.go`). Sold lines keep the
fallback they were sold with, so an exchange judges them by it
(`deal_exchange.go`). A till on an older build ignores the new field and
gives the deal's own 20%; it never stacks a discount on the deal. Tests:
`TestDealPoolsUnitsWithTheirOwnFallback`, `TestDealTermsIsRewardUnit`.

## 2026-10-30 · Salespersons shared colours on the picker

**Symptom.** Three salespersons were orange and two pink on the till's
salesperson picker.

**Cause.** kashi picked each new salesperson's colour at random from 12.
The till shows the colour kashi sets.

**Fix.** In kashi (its `docs/FIXED_BUGS.md`): a new salesperson gets the
colour least used in the branch, and the existing duplicates were reassigned.
Nothing changed on the till.

## 2026-10-30 · Payment amount showed "NaN"

**Symptom.** The amount box above the keypad showed "NaN" instead of a
number.

**Cause.** Pressing "." with nothing typed made the entry ".", and the box
showed `Number(entry)`, which is NaN for ".". The same display also dropped
what was being typed after the point ("1.50" showed as "1.5", "12." as "12").

**Fix.** A point typed first starts the entry as "0.", and the box shows the
entry as typed with thousands grouped (`displayValue` and `appendDigit` in
`frontend/src/components/Calculator.vue`).

## 2026-10-30 · Summer/Winter picker said "No discount" on every option

**Symptom.** Scanning a product sold as Summer or Winter showed a
"No discount" chip on each option, which read as if the item had no deal
either, even when it was on B2G1.

**Cause.** The picker showed a chip for every option, "No discount" when
the option had no percent of its own.

**Fix.** The chip shows only when the option has a discount
(`frontend/src/pages/home.vue`).

## 2026-10-30 · Cart showed the wrong price on deal lines split by sale option

**Symptom.** The same product in the cart twice, once as Summer and once as
Winter, on a B2G1 deal: 1 Summer + 3 Winter Kids Girl Jeans at $15 showed
$0 and $25.5, and both chips said "1 free". The right figures are $0 (the
free unit) and $40.5 (two at $15, one at 30% off). The total charged was
always right ($40.50); only the per-line figures were wrong.

**Cause.** The cart takes each line's share of a deal from the till's quote
and matched quote lines to cart lines by barcode alone. Two cart lines of
one product with different sale options share a barcode, so each picked up
the deal's whole discount ($15 free + $4.50 leftover 30%) and took it off
its own price.

**Fix.** Quote lines now carry the sale option the cart sent
(`LineQuote.SaleOption`, kept even when the option no longer counts), and the
cart matches on barcode + sale option (`dealLineKey` in
`frontend/src/lib/deals.ts`). Tests: `TestDealQuoteLinesNameTheirSaleOption`,
`frontend/src/lib/deals.test.ts`.

## 2026-10-30 · Cart line delete button highlight too wide

**Symptom.** Hovering or pressing the trash icon on a cart line showed a wide
pill-shaped highlight that spilled out of its column.

**Cause.** It was a plain text `v-btn`, whose minimum width (about 50px) is
wider than the 36px column it sits in.

**Fix.** It is now an icon button of 32px (`InvoiceItems.vue`), so the
highlight is a circle that fits.
