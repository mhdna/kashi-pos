# Fixed bugs

One entry per bug fixed in kashi-pos, newest first: what went wrong, why,
and what changed. Add an entry in the same change as the fix (see AGENTS.md).

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
