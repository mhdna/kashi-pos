# Feature ideas

Things to maybe build later. Not built unless asked.

## Ideas

- Licensing: keep the "latest time this till ran" inside the encrypted database too, so restoring an old license.dat together with a clock set back can't stretch a lease (2026-11-19).
- Licensing: build releases with `garble` so the license checks and the lock are harder to find and patch out of the exe (2026-11-19).
- Licensing: a paid-until date per customer on the license server that turns them off by itself, with a warning on the owner's screen a week before (2026-11-19).
- Licensing: show "Licensed to ..." and the install code in Settings, for support calls (2026-11-19).
- Sync central's colour hex values to the till, so variant swatches use the colours set in central instead of reading the colour name (2026-11-07).
- Use the new colour/size picker (`VariantPicker`) for a scanned code shared by several variants too, instead of the plain list in `home.vue` (2026-11-07).
- Fetch the thumbnails of recent expenses, loans and transfers into the photo cache in the background, so even a first look at a page is instant (2026-11-10).
- Receipt: an "Itemized" Classic layout with colour, size and code under each name, and quantity × price with the old price struck through on discounts (2026-11-14).
- Receipt: a QR of the invoice number at the bottom, plus scanning it at the till to open that sale for an exchange or return (2026-11-14).
- Receipt: an item count line ("16 pcs · 8 lines") (2026-11-14).
- Receipt: a solid total box and a "You saved $X today" line instead of the Discount row; built on the `receipt-total-saved` branch, not merged (2026-11-14).
- Referral modes beyond the first one (a single 15% on the owner's next visit): e.g. the friend also gets a discount, other amounts or rules per program (2026-11-16).
- WhatsApp a gift message with the referral reward, and send the friend their own code so referrals keep spreading (2026-11-16).

## In progress

## Done
