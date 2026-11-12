# Fixed bugs

One entry per bug fixed in kashi-pos, newest first: what went wrong, why,
and what changed. Add an entry in the same change as the fix (see AGENTS.md).

## 2026-11-12 · In-place search: the code grid still ran behind the keyboard

**Symptom.** Searching in place on the sale screen, the product grid's last rows
(and its "N products" line) sat behind the search keyboard, so they couldn't be
scrolled into view. The 2026-11-11 fixes for this didn't help on a real till.

**Cause.** The results' height comes from home.vue's `.home-search-results`
(`height: calc(100% - keyboard)`), but their root is ProductSearchResults'
`.product-list` (`height: 100%`). Scoped, that one weighs two classes, the same as
home.vue's rule, and the two are in different CSS chunks, so load order decided.
In the built app `.product-list` won and the list was the whole pane tall.

**Fix.** home.vue's rule is now `.home-page .home-right-pane .home-search-results`,
outweighing `.product-list[data-v]` whatever the order (`pages/home.vue`). Test:
`lib/searchResultsCss.test.ts` reads both stylesheets and checks the weights,
counting the scoped attribute.

## 2026-11-12 · The sync light's busy badge never moved on a till

**Symptom.** The new busy badge on the nav rail's sync light stood still on tills,
so "Updating" looked the same as a stuck state.

**Cause.** `styles/settings.scss` kills every animation with `!important`; only
the kit's spinners opt back in, and the badge's animations didn't.

**Fix.** The badge's breathe/drift/spin animations carry `!important` like the
spinners, since they mean "working" (`components/SyncIndicator.vue`); reduced
motion still stops them. `docs/dev/ui-components.md` lists it with the spinners.

## 2026-11-12 · Settings tabs past the edge of the screen couldn't be reached

**Symptom.** On a till whose screen is narrower than the Settings tab bar, the
last tabs ("Cash & shifts" and on) were cut off, and there was no way to scroll to
them.

**Cause.** The kit's `k-tabs` was `overflow: hidden`, so anything past its width
was clipped with no scroll.

**Fix.** `k-tabs` now holds its tabs in a track that scrolls sideways (swipe or
mouse wheel), shows edge arrows while there is more on that side, and brings the
chosen tab into view (`ui/KTabs.vue`, `ui/ui.css`, rules in `ui/tabsScroll.ts`).
Tests: `ui/tabsScroll.test.ts`, `ui/KTabs.test.ts` ("Settings tabs wider than the
screen can be scrolled to").

## 2026-11-12 · Setup screen showed the central API key in plain text

**Symptom.** On a new till's "Set up this branch" screen, the pasted central API
key and server address were readable by anyone looking at the screen.

**Cause.** Both were plain text fields.

**Fix.** The setup screen is now its own component (`components/BranchSetup.vue`)
with the rules in `lib/branchSetup.ts`: the key and the server address are masked
like passwords with an eye to check them, the code is upper-cased as it's typed,
and a line says whether the till will sync, run on its own, or is missing half of
the central pair. Tests: `lib/branchSetup.test.ts`, `components/BranchSetup.test.ts`
("the API key and server address are masked until the eye is tapped").

## 2026-11-12 · The sync light counted 100, 200, 300… and its ring meant nothing

**Symptom.** During a long pull the nav rail's sync light read "Updating ·
Clients · 200 …", then 300, 400 and on, cut off, with a thin ring around a tiny
icon that filled and emptied for no visible reason.

**Cause.** The rail showed the whole work line, including the running "N
changes" count; central never says how many rows are coming, so the number only
climbs. The ring's percentage was the pull's step (entity 3 of 21), not rows.

**Fix.** The rail now shows only the step's name while pulling ("Clients") and
"3 of 12" while sending; the full line stays in the tooltip and the dialog's
"Right now" row (`lib/linkStatus.ts`, `railLine`). Only sending has a real
percentage. The icon sits in a round badge in the state's colour that breathes
while busy and drifts down (Updating) or up (Sending) (`components/SyncIndicator.vue`).
Tests: `lib/linkStatus.test.ts` ("a long clients catch-up never shows a climbing
count"), `components/SyncIndicator.test.ts`.

## 2026-11-11 · Some notifications weren't grey like the rest

**Symptom.** Most notifications were the see-through grey, but some weren't: the
WhatsApp and cash-machine toasts were white, the clients page's was solid, the
clock warning was orange and the update banner blue, and sale errors red.

**Cause.** The grey look lived in a copy-pasted `.app-snackbar` rule on three
pages and in AttendanceToast; every other snackbar fell back to the base look or
painted its own background, and a `color` filled the whole background.

**Fix.** The grey is now the base `.k-snackbar__wrapper` in `ui/ui.css`, and a
`color` only tints the icons and the leading edge; the per-screen copies are gone
and a sale error reads in red on the same grey (`pages/home.vue`). Test:
`ui/snackbarLook.test.ts` (no screen may set a snackbar background again).

## 2026-11-11 · Checkout: "custom item discounts are disabled" on a discounted line

**Symptom.** With custom item discounts off, checking out a cart showed "custom
item discounts are disabled" and the sale didn't go through. It happened when a
line had a discount the cashier didn't type into the discount field: one central
set on the live cart, one from editing the price (discount mode), or a discount
turned off.

**Cause.** Before checkout, `flushChanges` saves every line's discount onto its
product (`UpdateProductDiscount`) when it differs from what the line started
with. It did this for all lines, not just ones whose discount field is editable,
and the backend refuses any save while the setting is off. With the setting on,
the same path silently made central's per-sale discount the product's own.

**Fix.** `persistDiscount` saves only a discount the cashier could type
(`discountEditable`) on a line whose discount isn't turned off; every other
discount stays with this sale (`components/InvoiceItems.vue`). Tests:
`components/InvoiceItems.discountSave.test.ts`.

## 2026-11-11 · Search results: the last rows can't be scrolled into view

**Symptom.** Searching in place (the product list over the sale screen), the
list scrolled to its end but the last row of cards stayed half under the search
bar at the bottom.

**Cause.** The results box (`.home-search-results`) is positioned with `top: 0;
bottom: <keyboard height>`, but the list inside it sets `height: 100%`, and an
explicit height beats `bottom`. So the list was as tall as the whole pane and its
last rows were behind the keyboard, past the end of the scroll. (The first fix
for this, measuring the inset from the real positions, didn't touch that; it is
kept, since it is still the right number.)

**Fix.** `.home-search-results` now gives its height outright:
`calc(100% - var(--home-search-keyboard-height))` (`pages/home.vue`). Tests:
`lib/searchInset.test.ts`.

## 2026-11-11 · Unknown-item QR forgotten after leaving the sale screen

**Symptom.** With an unknown item in the cart and its pictures already taken,
going to another page and back showed a new QR code with "0 of 1 pictured".

**Cause.** The pictures session's ref lived only in memory on the sale
screen, so coming back opened a brand-new session while the draft cart (and
its unknown item) was restored.

**Fix.** The ref now rides in the cart draft (`unknownClientRef` in
`pages/home.vue`), is saved when it changes and restored with the cart; a
finished or cleared sale still forgets it. Test:
`pages/homeUnknownSession.test.ts`.

## 2026-11-10 · The calculator showed the total twice ("$30.00" over "$30")

**Symptom.** On a sale of exactly $30 the calculator's total card read
"$30.00" with "$30" right under it, the same amount twice.

**Cause.** The line under the total is the cash split: whole dollars plus the
rest in LBP ("$30 + LL 45,000"). With nothing left over the LBP half is
dropped, leaving only "$30", which repeats the total above it.

**Fix.** The split line is hidden when it is a single amount in the same
currency as the total shown (`splitRepeatsTotal`, `lib/totalSplit.ts`;
`Calculator.vue` now says which currency the lone half is in, and
`calculator/CalculatorStatus.vue` uses it). It still shows with two halves,
or in the other currency (a lone "LL 450,000" under a USD total, or "$30"
when the total is flipped to LBP). Tests: `lib/totalSplit.test.ts` and
`components/calculator/CalculatorStatus.test.ts`.

## 2026-11-07 · An exchange called a free item's credit "unused" before anything was bought

**Symptom.** With every item of a deal sale returned and nothing bought yet,
the summary under the cart said "Unused free item credit (Buy 1 Get 1): $9.00"
next to "Add $41.00 more in new items", as if the credit were already lost.

**Cause.** The summary showed the deal's forfeited credit whenever there was
any. While the new items don't cover the returned ones, none of it is lost
yet: it is used as soon as new items are added.

**Fix.** The "unused" note shows only once the exchange asks for nothing
more (`exchangeSummaryRows`, `lib/exchangeQuote.ts`). Test: "says what is
missing, and not that a free item's credit is lost, while more is needed" in
`lib/exchangeQuote.test.ts`.

## 2026-11-07 · An exchange asked for more than its lines were short of

**Symptom.** Returning a whole Buy 1 Get 1 sale except one kept free unit
showed the lines as $0, −$15 and −$26 (−$41) but asked "Add $44.00 more in new
items". The free item said "no credit".

**Cause.** What the replacements must cover was what was paid for the returned
units ($44). It left out the $3 the deal charged back, which the lines already
take off. The free unit's note only showed the swap credit used, which is $0
until something is bought.

**Fix.** The replacements must now cover what was paid less what was charged
back: exactly the credit the lines show, so "add $X more" is always what the
lines are short of (`deal_exchange.go`). The quote also sends each free unit's
allowance, and its note reads "Free item · up to $9.00 off new items"
(`exchange_quote_lines.go`, `lib/exchangeQuote.ts`). The backend and the
screen both check the new rules: a shortfall equals the lines' shortfall, a
quote that asks for nothing more is never a refund, and swap credit never
exceeds its allowance. Tests: `TestExchangeQuoteShortfallIsTheLinesShortfall`,
`TestExchangeQuoteFreeUnitAllowance`, the random-exchange test, and
`lib/exchangeQuote.test.ts`.

## 2026-11-07 · The exchange screen's lines didn't add up to its total

**Symptom.** An exchange of a $30 unknown item for a returned $12 Lady Winter
Vizon showed the lines as $30 and −$12, but the total as $24.33, with nothing
on screen to explain the other $6.33.

**Cause.** The cart showed every returned line at its shelf price. Checkout
credits what was paid for it, less any deal saving charged back when the
return breaks the deal (`deal_exchange.go`). The quote only sent the total,
so the screen couldn't show either. The quote and checkout also worked the
total out in two copies of the same code.

**Fix.** `QuoteExchangeTotal` now returns each cart line's own amount (paid,
swap credit, charged back) and a breakdown with net, tax and rounding
(`exchange_quote_lines.go`), and refuses one that doesn't add up. The quote and
checkout share `computeExchangeTotals`. The cart shows each line's amount,
with a note under a returned line explaining its credit, and a summary under
the cart that adds up to "To pay" (`InvoiceItems.vue`, `ExchangeSummary.vue`,
`lib/exchangeQuote.ts`). Tests: `exchange_quote_lines_test.go` (including 1,200
random exchanges, each quoted and checked out), `TestExchangeQuoteGolden`,
`lib/exchangeQuote.test.ts`, `ExchangeSummary.test.ts`.

## 2026-11-07 · Change Salesperson showed the salesperson's number, not their name

**Symptom.** On the invoice list, Change Salesperson opened with a number (the
salesperson's id) in the field instead of their name; for some invoices it never
turned into a name.

**Cause.** The field names its value from the options list, and the list was
empty until `ListSalespersons` answered, so it fell back to the bare id. A
salesperson the list leaves out (attendance-only staff, a till-only leftover once
Central manages the roster) stayed a number for good.

**Fix.** The dialog is now `frontend/src/components/InvoiceChangeDialog.vue`. It
names the invoice's own salesperson from the list row at once, keeps them on offer
even when the roster leaves them out ("Former salesperson" if the name is gone),
and opens with the roster as last loaded (`frontend/src/lib/invoiceEditOptions.ts`).
Tests: `frontend/src/components/InvoiceChangeDialog.test.ts` ("Change Salesperson").

## 2026-11-07 · A remote return of a deal product refused a quantity the sale had

**Symptom.** From central, returning 3 units of a product sold on a deal as 2 paid
and 1 free failed with "requested quantity 3 ... exceeds the original line
quantity 2", and asking for the paid line after the free one failed the same way.

**Cause.** `executeRemoteReturnInvoiceCommand` still assumed a product sits on
one line of a sale (true before `000061_invoice_products_surrogate_key`). It
matched every requested item to the product's first line and capped it there.

**Fix.** Each requested item is spread over all of its product's lines, at the
unit price central now sends with it, and never past what they hold between them
(`commands.go`). Test: `TestExecuteRemoteReturnInvoiceCommand_DealLinesOfOneProduct`.

## 2026-11-07 · Returning a buy-1-get-1 pair failed with "item has already been returned"

**Symptom.** Returning both shoes of a Buy 1 Get 1 sale (remotely from central, or
at the till) failed with "checkout return: item has already been returned",
though nothing had been returned.

**Cause.** A deal sale stores alike units as two lines, one paid and one free.
The return request merges alike items into one quantity of 2, and the check
wanted a single sold line holding all 2, so it found none.

**Fix.** A returned quantity is now taken from as many sold lines as it needs,
the free one first (`exchange_validation.go`, `validateAndPriceExchange`). Test:
`TestDealRefundOfAlikeUnitsSpansThePaidAndFreeLines`.

## 2026-11-06 · Spaces in the scan field flashed "not found" or missed the product

**Symptom.** Pressing Enter on a field holding only spaces cleared it and
flashed the red "not found" feedback. A code or barcode with a stray space
inside ("AB 123") found nothing.

**Cause.** The empty check in the scan handler ran before trimming, and the
lookup only trimmed the ends of the value.

**Fix.** `frontend/src/pages/home.vue` trims first and ignores a blank entry.
`products.go` (`FindProductsForScan`, `lookupForScan`) retries without inner
whitespace when the value as typed matches nothing. Covered by
`TestFindProductsForScan` in `products_test.go`.

## 2026-11-06 · Long Arabic event names lost their beginning, not their end

**Symptom.** On the dashboard's "Coming up" card, a long Arabic event name was
cut with "..." on its left, hiding the start of the name ("...مام علي").

**Cause.** The name sat in a left-to-right box, so the Arabic text was laid out
right-to-left from the box's left edge and the clipped edge was its start.

**Fix.** `dir="auto"` on the name in `frontend/src/components/shifts/EventsCard.vue`
and on the calendar cells in `frontend/src/pages/events.vue`: Arabic names now run
right-to-left and the ellipsis comes at their end. No automated test (visual).

## 2026-11-06 · A turned-off deal chip was much wider than the live one

**Symptom.** On a cart line, turning off a "B1G1 else 30%" deal made its chip
far wider than the chip had been while the deal was on.

**Cause.** The live chip splits the label into two lines ("B1G1" / "else 30%"),
but the "off" chip printed the whole label struck through on one line.

**Fix.** `frontend/src/components/InvoiceItems.vue`: the off chip now uses the
same split (`offDealParts`): short name struck through, then "else 30% · off".

## 2026-11-05 · Shutting down showed "You're about to be signed out"

**Symptom.** After closing a shift and choosing "Yes, shut down", Windows put up
a "You're about to be signed out" notice over the till before turning off.

**Cause.** The till ran `shutdown.exe /s /t 3`, giving itself three seconds to
close cleanly; any delay above 0 makes Windows show that notice.

**Fix.** A hidden helper waits the two seconds instead and then runs
`shutdown /t 0`, which shows nothing (`startPowerCommand` in `kiosk_windows.go`).
Windows-only, so no automated test: try it on a till.

## 2026-11-05 · Payment icons sat right against their names

**Symptom.** On the Shifts page's income card the wallet icon touched "Cash $$"
and "Cash LL", with no space between them. Every list row starting with a round
icon did the same.

**Cause.** When the till's own k-* kit replaced Vuetify, list rows kept a gap
after a plain icon but not after an avatar (the round icon); Vuetify had given
it 16px.

**Fix.** `.k-list-item__prepend > .k-avatar` gets a 16px gap again
(`frontend/src/ui/ui.css`). A row that sets its own gap (attendance) keeps it.

## 2026-11-05 · Rounded dialogs showed a grey square around them

**Symptom.** Dialogs with rounded corners (devices, options) had a grey square
edge around them, so they didn't look rounded.

**Cause.** The dialog's outer box drew the shadow with 4px corners, while the
card inside was rounded far more (`rounded="xl"`); the square shadow showed at
the corners.

**Fix.** The shadow moved from the outer box to what it holds, so it follows the
card's own corners (`frontend/src/ui/ui.css`).

## 2026-11-06 · The updating screen's spinner was an empty ring

**Symptom.** "Updating GB Cloud" showed a bare spinning ring, without the update
icon meant to sit in it. The ring on the Conversion page lost its percentage
the same way.

**Cause.** The till's own spinner (`KProgressCircular`, from the move off
Vuetify) had no slot, so whatever was put inside it was dropped.

**Fix.** It shows its content centred in the ring (`frontend/src/ui/KProgressCircular.vue`,
`ui.css`), and the update icon pulses slowly (`AppUpdateBanner.vue`). Checked by
rendering the updating screen.

## 2026-11-05 · Lebanese numbers starting with 0 showed wrong on the client forms

**Symptom.** 03 927 807 showed as (39) 278-07 on New Client, and on editing a
client. Landlines such as 01 did the same.

**Cause.** The forms cut +961 off the stored number and showed what was left. +961
drops the leading 0, so the formatter split 3927807 in the wrong place. Only the
display was wrong: the number saved correctly.

**Fix.** `localLebanesePhone` puts the 0 back (`frontend/src/lib/phone.ts`, used by
`ClientDialog.vue` and `ClientEditDialog.vue`); covered by `phone.test.ts`.

## 2026-11-05 · Tapping a line's deal chip could open the line's details

**Symptom.** The deal chip sat inside the block that opens a line's details, so
the block lit up with it, and a tap on the chip could open the details too.

**Cause.** The chips were nested inside the details button.

**Fix.** The chips sit beside it (`frontend/src/components/InvoiceItems.vue`).

## 2026-11-05 · AnyDesk's minimized bar showed on top of the till in kiosk mode

**Symptom.** With AnyDesk's session window minimized, its little title bar stayed
in the bottom-left corner, over the till's menu.

**Cause.** Asking Windows not to draw minimized windows (`hideMinimizedWindows`)
didn't cover it.

**Fix.** While kiosk mode is on, a minimized AnyDesk window is hidden, and comes
back when kiosk mode ends (`kiosk_windows.go`). Not yet tried on Windows.

## 2026-11-05 · Changing one row of a list rewrote the whole list on the till

**Symptom.** Adding a rule that changed one product's discount made the till apply
the whole list again, slowly.

**Cause.** A list arrives whole, and the till cleared and refilled it, moving the
pricing version once per row.

**Fix.** Only the rows that differ are written (`sync_pull_lists.go`). Test:
`TestApplySyncedDiscountListTouchesOnlyWhatChanged`.

## 2026-11-05 · Unknown items stayed on Buy 2 Get 1 after a branch changed deal

**Symptom.** With Centro on Buy 1 Get 1, unknown items still made sets of three
among themselves, and their picker still said "B2G1".

**Cause.** The deal they join only looked for a "buy 2 get 1", and the till keeps
deals after their list has gone, so it kept finding the old one. The picker's text
was fixed.

**Fix.** They join the free-units deal the most products are on in the lists in
force, and the picker names it (`unknown_item_deal.go`, `frontend/src/lib/deals.ts`).
Tests: `TestPickUnknownItemDeal`, `TestUnknownItemFollowsTheBranchToANewDeal`.

## 2026-11-05 · A branch moved to another list never got it

**Symptom.** Centro was moved to a new discount list in central, but the till kept
selling on the old one, with no error on screen.

**Cause.** A list central switches off was deleted here. Once it has sales the delete
fails, because `invoice_products` points at it. That rolled back the whole pull, so
the new list was lost too and the cursor never moved.

**Fix.** The list's row stays, switched off and emptied (`sync_pull_lists.go`).
Tests: `TestApplySyncedPriceListEmptiesOnDeactivation`,
`TestUnknownItemFollowsTheBranchToANewDeal`.

## 2026-11-01 · The backspace key on the keypads showed a tiny icon

**Symptom.** The backspace key on the number pad, login PIN pad, return/exchange
pad, on-screen keyboard and phone pad showed a very small ⌫ symbol, hard to see
on a touch screen.

**Cause.** The key's label was the ⌫ text character, so its size depended on
whichever system font supplied the glyph, which draws it small.

**Fix.** All five keys use `mdi-backspace-outline` (already registered in
`lib/icons.ts`) at 20px: `NumberPadMenu.vue`, `LoginForm.vue`,
`ReturnInvoiceMenu.vue`, `InlineKeyboard.vue` and `PhonePad.vue`. No test
covers it (it is a drawing change); the frontend build and tests pass.

## 2026-10-31 · Invoices, Expenses and Loans stopped loading more rows on scroll

**Symptom.** Since the move off Vuetify, scrolling to the bottom of the
Invoices list (and of Expenses and Loans) no longer loaded older rows: the
list stopped at its first page.

**Cause.** The scroll handlers only reacted to scrolling of an element with
Vuetify's table class, `v-table__wrapper`. The new `k-data-table` scrolls a
`k-table__wrapper` instead, so the check never matched and the next page was
never asked for.

**Fix.** Both handlers look for `k-table__wrapper`:
`frontend/src/pages/invoices-list.vue` and
`frontend/src/composables/useInfiniteList.ts` (Expenses, Loans). No other
script or style still refers to a Vuetify class. No automated test: the
frontend has no DOM test setup.

## 2026-10-31 · Branch picker on the login screen shrank to its text

**Symptom.** Since the move off Vuetify, the branch picker above the PIN pad
was only as wide as the branch name, not the 280px it had before.

**Cause.** `KSelect` (and `KAutocomplete`) had two top-level elements: the
field and its dropdown menu. Vue only puts a page's scoped-style marker on a
component with a single root, so the page's `.pin-branch-select { width:
280px }` never matched. Any page's scoped class on a `k-select` or
`k-autocomplete` was ignored the same way.

**Fix.** The menu now sits inside the field (it teleports to the body
anyway, leaving only a hidden marker), so both components have one root and
scoped classes on them apply again. `frontend/src/ui/KSelect.vue`,
`frontend/src/ui/KAutocomplete.vue`. No automated test: the frontend has no
DOM test setup (no `@vue/test-utils`/jsdom).

## 2026-10-31 · Country names in the phone dialog drawn in the flag's colours

**Symptom.** In the client phone dialog (macOS window), the country names
next to the flags were coloured like the flag (green for Iraq, red for
Palestine) and invisible when the flag's colour was white (France, Indonesia,
Saudi Arabia, Switzerland). The selected country in the field at the top was
nearly invisible too. Windows tills were not affected.

**Cause.** WebKit (the macOS webview) lets the colour of a colour-font flag
glyph (the bundled Twemoji flags) leak into the text drawn after it in the
same layer. Hiding the flags, or giving the text another font, made no
difference; isolating the flag's painting did.

**Fix.** `.country-flag` is `contain: paint`, so the flag is painted on its
own and its colour can't carry over (`components/ClientDialog.vue`). Checked
by rendering the dialog in WebKit before and after; there is no automated
test for this.

## 2026-10-31 · Keyboard and scanner typed Windows shortcuts after AnyDesk

**Symptom.** After connecting to a till with AnyDesk from a Mac, the till's
keyboard and scanner stopped working on the whole computer: typing 1 did
Win+1 (opened the first taskbar program), and a scan fired a string of
shortcuts. It lasted after AnyDesk disconnected, until someone pressed the
Windows key on the till. Turning off AnyDesk's "Transmit keyboard
shortcuts" didn't stop it.

**Cause.** AnyDesk sends the Mac's ⌘ as the Windows key. When the Mac keeps
a key's release to itself (⌘+Tab away from the AnyDesk window), the till
hears Win go down but never up, so Windows holds it until a real Win press.
Not kashi-pos's doing, but only the till can notice it.

**Fix.** The device listener, which already sees every key with the device
it came from, tracks which modifiers a keyboard or scanner on the till is
holding. Every second it checks the ones Windows holds, and lets go of any
(Win, Ctrl, Alt, Shift) held by nothing on the till for 3 seconds
(`stuck_keys.go`, `devices_windows.go`). The kiosk keyboard hook lets that
release through (`kiosk_windows.go`). Tests: `TestModifierWatch*` in
`stuck_keys_test.go`.

## 2026-10-30 · Punches made while the till was off could be missed

**Symptom.** An employee's attendance in central had almost only arrival
punches (Ahmad Robaie at Centro: one punch a day all October), while
colleagues on the same device had their exits too.

**Cause.** After a punch the device pings the till, which then asked the
device for that day's punches only, and nothing pulled when the till
started. A punch made while the till was off (leaving after closing) was
never asked for again if the device honours the dates. Not confirmed on the
device itself; the fix is safe either way.

**Fix.** Every pull asks for the past 7 days (already imported punches are
skipped as before) and the till pulls once when it starts
(`attendance.go`, `attendance_push_server.go`). Test:
`TestSyncAttendanceAsksForThePastWeek`.

## 2026-10-30 · In-place product search squeezed the cart to a sliver

**Symptom.** With the sale screen's in-place product search open, the cart
showed only the top of its first line (the name, no code, one price), as
if something hid the rest.

**Cause.** The search keyboard was meant to lie over the bottom of the
screen (`position: absolute` in `home.vue`), but its own component style
(`position: relative` on `.product-search-keyboard`, scoped, so more
specific) won. It stayed in the page's flow and took its height from the
panes above; the cart, the only part that shrinks, was left about 20px.

**Fix.** `home.vue`'s rule is now `.home-page .home-search-keyboard`, which
outranks the component's, so the keyboard lies over the screen and the
cart keeps its full height behind it. Checked in Chrome on the sale screen
by measuring the cart with the search open and closed.

## 2026-10-30 · Product search: tapping beside the field lost its focus

**Symptom.** In the products search dialog, a tap anywhere outside the
search field (beside the keyboard, on the results count, an empty spot)
took the focus off the field, and with it the caret.

**Cause.** Pressing on any part of the dialog is a mousedown, and the
browser moves the focus away from the field on mousedown. Only the
keyboard's keys guarded against it.

**Fix.** `ProductSearchDialog.vue`'s card cancels the mousedown
(`keepSearchFocus`) everywhere but the field itself and the results'
scrollbar, so the field keeps its focus, caret and selection; taps and
clicks still work. Checked by touch and mouse in Chrome.

## 2026-10-30 · Product search: no way to edit the middle of a search by touch

**Symptom.** In the products search dialog, tapping inside the search text
didn't put the caret there (a finger drag only scrolled the text), nothing
could be selected, and the on-screen keys always typed and deleted at the
end, wherever the caret was.

**Cause.** `InlineKeyboard` only appended to or chopped the end of its
value, and pressing a key took the focus off the field, losing its caret.
WebView2's own touch handling of a one-line input doesn't place a caret
or select on a drag.

**Fix.** `InlineKeyboard.vue` takes the field (`input` prop) and types,
deletes and replaces at its caret or selection, keeps it focused, and adds
caret ◀ ▶ and select-all keys. `composables/useTouchSelection.ts` takes
touch over in the search field: tap places the caret, drag selects, double
tap selects all. Its field is `inputmode="none"` so Windows' own touch
keyboard stays away. Tests: `lib/caret.test.ts`; checked by touch in
Chrome.

## 2026-10-30 · Country flags show as letters in the phone dialog on Windows

**Symptom.** On a Windows till, the client phone dialog's country cards and
country picker showed two letters ("LB") instead of the flag emoji again.

**Cause.** The bundled flag font was only registered when
`country-flag-emoji-polyfill`'s canvas check said so: a smiley had to draw
in colour *and* a flag had to fail. Where the till's WebView2 can't draw
colour emoji on a canvas (no colour emoji font, or software rendering),
the check failed, the font was never registered, and Windows fell back to
the letters.

**Fix.** `frontend/src/styles/typography.scss` declares the bundled
Twemoji Country Flags `@font-face` itself, always, limited to the flag
characters; `frontend/src/main.ts` only preloads it. The polyfill package
is removed. macOS now shows the same Twemoji flags as Windows. No
automated test: the frontend tests have no DOM; checked the built CSS.

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
(`deal_fallback_percent`, central 000216, till 000133). The till pools every
unit on the deal and gives a unit left outside a set its row's fallback
(`unitDiscounts` and `applyDeals` in `deals.go`). Sold lines keep the
fallback they were sold with, so an exchange judges them by it
(`deal_exchange.go`). A till on an older build ignores the new field and
gives the deal's own 20%; it never stacks a discount on the deal. Tests:
`TestDealPoolsUnitsWithTheirOwnFallback`, `TestDealTermsIsRewardUnit`.

## 2026-10-30 · Salespersons shared colours on the picker

**Symptom.** Three salespersons were orange and two pink on the till's
salesperson picker.

**Cause.** central picked each new salesperson's colour at random from 12.
The till shows the colour central sets.

**Fix.** In central (its `docs/FIXED_BUGS.md`): a new salesperson gets the
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
