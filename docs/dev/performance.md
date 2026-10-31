# Performance and older tills

The till runs on old Windows PCs: slow CPUs, little memory, spinning disks, and
often a GPU the WebView can't use well. Most "it feels slow" reports come from
there, not from a developer machine. This page records what we did about it,
why, and the traps that are easy to walk back into. The v1.1.80 move off Vuetify
(see [ui-components.md](ui-components.md)) was the big one; v1.1.82 added the
rest.

Numbers on this page come from a Mac, not a till. Treat them as ratios, not as
what a cashier will see. Anything marked **unmeasured** is reasoning, not data.

## The rules (read this before adding UI)

1. **No infinite animations, no transitions.** They force repaints on a WebView
   that draws on the CPU. Spinners are the only exception (see Animations).
2. **No `backdrop-filter`, no `filter: blur()`, no big or layered `box-shadow`.**
   With `--disable-gpu-rasterization` (see WebView2 flags) every one of them is
   computed on the CPU, on every repaint of whatever sits behind.
3. **Never write `localStorage` or `JSON.stringify` the cart on every key.**
   Use `lib/draftSaver.ts`.
4. **No deep watchers on the cart** unless they do almost nothing. The cart
   changes on every scan and every key.
5. **No unindexed `LIKE '%x%'` over a big table** (products, clients, invoices)
   on a keystroke. Products have a trigram index for this; nothing else does.
6. **Heavy libraries go in their own chunk**, loaded when needed
   (`defineAsyncComponent` / `import()`), not imported statically from the sale
   screen. libphonenumber is the example.
7. **Lists page.** Cursor or offset paging from Go, 20-100 rows at a time. Never
   load a whole table into the screen.
8. **Don't add a polling loop** without checking the table under Background work.
9. **Don't add a dependency** for something small. The bundle is parsed on every
   start; see the sizes below.
10. **Measure on throttled hardware** (Measuring, below) before and after.

## Sizes to keep an eye on

`pnpm build` in `frontend/`, at v1.1.82 (gzipped in brackets):

| Chunk | Size | Notes |
|---|---|---|
| `index-*.js` | 272 kB (95 kB) | Vue, router, pinia, the UI kit, the icon table. Loaded at launch. |
| `home-*.js` | 137 kB (42 kB) | The sale screen. It was 281 kB (79 kB) before libphonenumber moved out. |
| `ClientDialog-*.js` | 149 kB (39 kB) | Holds libphonenumber and its metadata. Loaded when idle or on first open. |
| `settings-*.js` | 122 kB (30 kB) | Opened rarely, so it stays a separate chunk. |
| `index-*.css` | 50 kB (10 kB) | The UI kit's CSS; `home-*.css` is another 23 kB. |

The pages are separate chunks (`router/index.ts`). `home`, `invoices-list` and
`shifts` are fetched while the login screen is up, so signing in doesn't wait on
them.

If `index-*.js` or `home-*.js` grows by tens of kB, find out why before merging.
`vite build` prints the table; compare with this one.

## Rendering

### WebView2 flags (`webview_windows.go`, `webview_args.go`)

WebView2 starts with `--disable-smooth-scrolling --disable-gpu-rasterization
--disable-gpu-vsync`. No commit message says why. The consequence: **everything
is rasterised on the CPU**, so shadows, opacity changes, blurs and large repaints
cost real time. Disabling GPU rasterisation is sometimes *slower* on a machine
whose GPU works; nobody has measured it on a real till.

`GB_CLOUD_WEBVIEW_ARGS` (user environment variable) replaces the flags for one
till, so the two can be compared without a rebuild. See
[windows-till.md](windows-till.md). The default has deliberately **not** been
changed: that decision waits for measurements on an old PC.

### Animations

- **Off by default on every build** (`lib/animationsDefault.ts`). Before v1.1.82
  the standard build defaulted to on, so the first start of v1.1.82 switches every
  till off **once** (marker `kashi-animations-reset-v1` in `localStorage`). After
  that a switch-on in Settings, or a push from kashi, is kept.
- `styles/settings.scss` already kills **every** transition and animation with
  `!important`, whatever the switch says. So "animations on" only brings back the
  few things that escape that rule on purpose.
- **Spinners always spin**, switch or no switch (`k-progress-circular`,
  `k-progress-linear`, `.print-spinner`, the startup loader's). They mean
  "working". They escape the global kill with their own `!important`.
- The switch now governs only the red status dot in the nav rail (solid red off,
  pulsing on) and the invoice salesperson menu's fade. It is scoped with
  `:root:not(.animations-disabled)`.
- **The `animations-disabled` class is on `<html>`**, not just on the app root,
  because dialogs and menus are teleported to `<body>`, outside the root.
- **Cascade-layer trap:** `ui/ui.css` lives in `@layer`s. For `!important`
  declarations the layer order **reverses**, so a layered `!important` beats an
  unlayered `!important`. That is why the spinner rules inside `ui.css` beat the
  global kill in `settings.scss`, and why an unlayered override can't turn them
  off again. Remove the `!important` or edit the layered rule itself.
- kashi's branch settings form defaults animations to **off** now too
  (`~/Code/kashi`, `branch-settings.vue`). The tri-state "Leave as is" in the
  default-branch settings is unchanged.

### No blur

The empty-cart overlay used to have `backdrop-filter: blur(4px)`. It was
repainted on the CPU whenever the calculator behind it changed. It is a plain
dim now (`rgba(0,0,0,0.18)`, still used in exchange and settlement modes). Don't
bring it back, and don't add a new one.

## Startup

The sequence, in order, and who pays for each step:

1. **Native splash** (`splash_windows.go`): a plain Win32 window with the logo,
   up within milliseconds. Covers the seconds before WebView2 can paint. Closes on
   `OnDomReady` or a safety timer.
2. **`index.html`'s `#startup-loader`**: static HTML and CSS, no JavaScript
   framework, covering the app until `WaitUntilReady` returns. It places the logo
   where the login page draws it (`startupLogoSpot` in `localStorage`) so there is
   no jump. Its spinner always spins. `index.html` is ~34 kB because the logo is
   embedded twice as base64; a small parse cost.
3. **Go: `WaitUntilReady`**: opens the database, runs migrations, starts the
   workers. This is the real wait. It happens whether or not a loader is up; the
   loader only hides it. If startup is slow on a till, look here first
   (**unmeasured**: add timestamps around it before guessing).
4. **Vue mounts**, `router-view` renders once startup is done, and the page chunks
   are prefetched in the background.

The loader is not what makes startup slow. Removing it would only show a blank or
half-built screen for the same time.

### Things loaded at startup, and how to keep them cheap

- **Fonts:** Roboto **latin and latin-ext only**, weights 400/500/700 (`main.ts`).
  The full packages also declare Cyrillic, Greek, Vietnamese and symbol faces the
  till never shows. The Twemoji flag font is fetched up front with
  `document.fonts.load` so the client dialog's first open doesn't wait for it.
- **Icons:** `lib/icons.ts` registers only the MDI icons the app uses, instead of
  shipping the whole set. The price is that **an unregistered `mdi-*` name renders
  as text**; `scripts/check-icons.mjs` (part of `pnpm build`) fails the build on
  one. See `AGENTS.md`.
- **libphonenumber** is the single biggest dependency. It is imported only by
  `ClientDialog.vue`, which `ClientButton.vue` and `AccountDialog.vue` load with
  `defineAsyncComponent`:
  - `ClientButton` prefetches the chunk when the browser is idle after mount
    (`requestIdleCallback`, 5 s timeout; 2 s `setTimeout` where it doesn't exist).
  - The dialog component is created on first open (`v-if` on a "loaded" flag) and
    then **kept**, so its close/reset behaviour is the same as before.
  - Don't import `libphonenumber-js` from anything on the startup path. We use
    `libphonenumber-js/mobile` (not `/min`) because mobile-type validation is
    needed, and `lib/phone.ts` has Lebanon-specific rules around it.
- **Sounds** (`lib/sound.ts`): imported as asset URLs; a `new Audio(url)` is made
  for every play. Cheap enough, but it is not a preloaded pool (**unmeasured**).

### The sale screen's pay area is built after first paint

The calculator and action bar are the heaviest part of the sale screen. While the
cart is empty they are hidden behind the "Ready to scan" panel, so they are not
built at startup (`home.vue`: `payAreaMounted`):

- They mount when the browser is idle after the first paint
  (`requestIdleCallback`, 1.5 s timeout; 300 ms `setTimeout` fallback), or **at
  once** when `showReady` goes false (an item is added, or the mode isn't `sale`).
- Once mounted they **stay** mounted and are only hidden with `v-show`. Don't
  switch that to `v-if`: a typed payment, the draft's calculator state and the
  `calculator` / `actionBar` refs would be lost every time the cart empties.
- **Rule: anything that talks to `calculator.value` while the cart may be empty
  must `await ensurePayArea()` first.** It sets the flag and waits a tick. Today
  that is `restoreDraft`, `onRestoreHold` and `onAccountClientSelect`. A new
  caller that forgets it silently does nothing at startup, because
  `calculator.value` is `null` until idle. (`clearAll()` calls are fine: there is
  nothing to clear if it was never built.)
- `canCheckout` reads `calculator.value?.checkoutReady`, so it is false until the
  area exists. That is correct: an empty cart can't check out.

This costs a few milliseconds once, when the till goes idle, instead of at first
paint. The gain is the time to first paint of the sale screen; it is **unmeasured**.

## The sale screen

### "Ready to scan" panel

In sale mode with nothing in the cart, the right pane shows a panel (icon, hint,
and Search product / Held invoices / Select client buttons) instead of the dimmed
calculator. Exchange and settlement modes keep the old dim, because they rarely
have an empty cart. "Select client" opens `ClientButton`'s dialog through
`useClient().requestClientDialog` (a counter the button watches).
The picture is our own inline SVG, `components/ReadyToScanArt.vue`: a barcode on a
card with its number printed under it (fitted to the bars with `textLength`), a red
laser line, grey scan-frame corners. It is static (no animation, no
filter) and uses theme colours, so it follows light/dark. Keep it symmetrical
about x=80 / y=60 in its 160x120 box when editing.
The buttons are white with a thin border and **no shadow** (shadows are CPU-drawn
on a till). "Held invoices" shows `· N` when there are held invoices: the count is
read with `ListHoldInvoices` each time the panel comes up. "Select client" shows
`Client: <first name>` once a client is chosen. The content sits a little above
the middle (more padding below than above).

It was added for clarity, not speed. Hiding the calculator with `display: none`
does skip its layout and paint while the cart is empty, but its components still
exist and their watchers still run.

### Pricing a sale

- Quotes are **not debounced**: `refreshQuote` runs on a 0 ms timer, only to fold
  changes made in the same tick into one request. `quoteRequestToken` drops a reply
  a newer request superseded. This is deliberate; see the comment in `home.vue`.
- A quote is cheap because Go keeps the active prices and discounts in memory
  (`pricing_overrides.go`). It re-reads them only when the `pricing_version` row
  has moved. **Triggers on every table that affects pricing bump that row**
  (migration 127), so a sync or an admin edit can never leave a stale price. A new
  table that affects a price needs the same triggers.
- The quote watcher is deep on the cart (`{ deep: true, immediate: true }`). It is
  the one place a deep watch is justified; keep its work to "start a request".

### The draft (`lib/draftSaver.ts`)

The cart, client, note and calculator state are kept in
`localStorage["kashi-pos.invoice-draft"]` so a crash or restart restores the sale.

- **Saved ~0.4 s after the last change**, built from the state *at that moment*,
  not when it was queued. Burst edits cost one write.
- **Removing is immediate** and cancels a pending save. If it didn't, a save queued
  just before checkout could write the finished sale back, and the till would offer
  to restore it after the next restart.
- Flushed on `visibilitychange` (hidden), `pagehide` and when `home.vue` unmounts,
  so closing the app doesn't lose the last edit.
- Settlement mode and the restore itself never save (`buildDraft` returns `null`;
  the watcher also cancels any pending save when entering them).
- If you add state to the draft, add it to `buildDraft()` and the `InvoiceDraft`
  type, and think about whether `restoreDraft` needs `ensurePayArea()` for it.

### Focus and observers

`home.vue` runs a `MutationObserver` on `document.body` (`subtree`, `class` and
`style` attributes) to re-focus the barcode field after dialogs close. It fires on
**any** class or style change anywhere in the document, debounced by 50 ms. It is a
known hotspot and **unmeasured**. Don't add more body-wide observers; if it shows
up in a profile, narrow it to overlay containers.

## Lists and search

| Where | How it pages | Size |
|---|---|---|
| Sale-screen product search | cursor (`ListProductsCursor`), count asked separately | 50 per batch (Go caps at 100) |
| Invoices page | cursor (`ListInvoicesFilteredPage`) | 20 |
| Expenses, Loans, others | `useInfiniteList` | 30 |
| Admin server product list | offset (`ListProductsPage`) | 200 |

- The result count is a **separate call** (`CountProductsForSearch`), asked beside
  the first page, so the list never waits for it.
- Searching waits **30 ms** after the last key, only to fold a paste or a barcode
  wedge's burst into one request. `latestRequest` drops stale replies.
- Scrolling for more rows depends on the scroll handler finding the table's scroll
  container. After the Vuetify swap it looked for `v-table__wrapper` and the lists
  stopped loading (see `FIXED_BUGS.md`); the kit's is `k-table__wrapper`.

### Product search index (migration 135)

`products_search` is an FTS5 table with the `trigram` tokenizer. `rowid` is
`products.id`; `all_text` is `name || ' ' || code || ' ' || barcode` and
`code_text` is `code || ' ' || barcode` (for the code-only switch). Three triggers
on `products` (insert, update of `id/name/code/barcode`, delete) keep it current,
so **every writer stays correct without Go code**: the sync pull, CSV import, edits
and an older build running on the same database.

- Terms of **3+ characters** use the index (`ListProducts*Indexed`,
  `CountProducts*Indexed`, generated by sqlc); **1-2 characters** use the old
  `LIKE` queries, because a trigram index can't answer them. `searchUsesIndex`
  (`products_search.go`) is the switch. Both must return the same rows; the tests in
  `products_search_test.go` compare them directly. If you change either side, the
  other must change with it.
- Matching is case-insensitive, like `LIKE`. The trigram tokenizer folds Unicode
  case a bit more broadly than `LIKE`'s ASCII-only rule; no practical difference for
  our data.
- `%` and `_` in a term are **not escaped**, same as before.
- Migration 135 backfills the index: ~0.8 s for 13.9k products, once.
- `ListProductsPage` (used by the admin server; no till page calls it now) still uses `LIKE`.

Timings, 13,851 products, Mac, list = first page of 21, count = total matches:

| Term | LIKE list / count | Index list / count |
|---|---|---|
| `bz-` (11 matches) | 5.2 / 5.3 ms | 0.24 / 0.05 ms |
| no match | 4.8 / 4.7 ms | 0.18 / 0.04 ms |
| `100` (189 matches) | 0.7 / 5.8 ms | 0.6 / 0.26 ms |
| `shirt` (2,406 matches) | 0.8 / 6.7 ms | **6.2** / 4.2 ms |

So it is a large win for the things cashiers type most (a code, a barcode, a rare
word) and **a loss for the first page of a very common word**: `LIKE` stops after
finding 21 rows, while the index has to gather all 2,406 before ordering. A known
possible fix: use `LIKE` when the count is large. Not done.

## Database settings (`app.go`, `databaseDSN`)

| Setting | Why |
|---|---|
| `journal_mode(WAL)` | Readers don't block the writer; the sync worker and the UI share the file. |
| `synchronous(NORMAL)` | With WAL the database can't corrupt, but power loss right after a commit can roll **that one** transaction back. In exchange a sale doesn't do a full `fsync`. This matters on spinning disks. |
| `cache_size(-65536)` | 64 MB page cache. |
| `temp_store(MEMORY)` | Sorts and `GROUP BY` don't touch disk. |
| `mmap_size(256 MB)` | Reads page the file in directly. |
| `busy_timeout(5000)` | Wait up to 5 s for a lock before failing. |
| `_txlock=immediate` | Write transactions take the lock up front, so a sync-worker commit can't land between a sale's reads and its write and fail the sale. Read-only transactions still begin deferred. |
| `foreign_keys(ON)` | Integrity. |

`PRAGMA optimize` runs once after the migrations at open, to refresh the planner's statistics after any schema change. The SQLite driver is
`modernc.org/sqlite` (pure Go, no cgo); FTS5 and the trigram tokenizer are built in.

## Background work

Everything here runs while the till is "idle", on the same slow CPU as the UI.

| What | Every | Cost |
|---|---|---|
| Attendance probe (`attendance_probe.go`) | 1 s | A DB read of the config, plus a TCP connect (500 ms timeout) when attendance is set up. Emits an event only when the state changes. |
| Device watch (`devices.go`) | 3 s, and on plug/unplug | Raw-input device list and printer status through Win32 calls, no PowerShell. Also releases stuck keys. |
| Sync worker (`sync_worker.go`) | 15 s, plus backstop and failed-retry timers | Outbox push and pulls. Stops at the first network error rather than backing off per row. |
| Cash machine sweep (`cash_machine_windows.go`) | 30 s | Only in cash machine mode. |
| Updater (`updater.go`) | check + idle tickers | Update checks. |
| Sync WebSocket (`sync_ws.go`) | ping + config re-check tickers | Live pushes from kashi. |
| WhatsApp retry (`whatsapp.go`) | retry ticker | Only when messages are pending. |
| Frontend `setInterval`s | various | `attendance.vue` 60 s while open; `transfers.vue` and `PhotoUploadDialog` poll while open; `CloseShiftCard` ticks a clock and polls receipts while open. They stop when the page closes; check that a new one does too. |

None of these has been profiled on a till (**unmeasured**). The attendance probe's
once-a-second DB read is the one most worth checking if idle CPU looks high.

## Measuring

- **Open the inspector:** start the app with `KASHI_DEBUG=1` (`main.go`). Use the
  **Performance** tab, record while scanning and while typing in product search, and
  set **CPU: 6x slowdown** to get closer to an old PC.
- **Bundle:** `cd frontend && npx vite build`; compare the chunk table with the one
  above. For what is inside a chunk, `grep -o` for a library marker in `dist/assets`
  (`countryCallingCodes` is libphonenumber's).
- **Queries:** copy a real database (`/bin/cp`, **never** work on the original, and
  note that `cp` may be aliased to `cp -i` and hang a script waiting for a "y"), then
  time the old and new query in a throwaway Go test, many repetitions each, list and
  count separately. Use `EXPLAIN QUERY PLAN` to confirm an index is used: the FTS
  plan reads `SCAN s VIRTUAL TABLE INDEX 0:L0`; a plain `SCAN products` means it isn't.
  `applyMigrations(ctx, db, path, true)` needs the real path (it makes a backup next
  to it).
- **On a till:** there is no log file (see README, item 1), so add one or use
  timestamps shown in the UI. Compare the WebView flags with `GB_CLOUD_WEBVIEW_ARGS`.
- Measure **before and after**, on the same data. Several of the changes above are
  cheap on a Mac and only matter on the slow PCs.

## Decisions and things deliberately not done

- **WebView2 flags unchanged.** Only made overridable. Needs a real till.
- **Calculator not unmounted when empty.** `v-if` would save more, and lose state.
  See the pay area section.
- **No `libphonenumber-js/min`.** Mobile validation needs `/mobile`.
- **Startup loader kept.** It hides the Go startup wait and costs little.
- **`LIKE` fallback for very common words** in product search: not built.
- **`ListProductsPage` not moved to the index** (offset paging, admin server only).
- **No virtual scrolling** in the invoice lines or lists; paging is enough so far.

## Ideas not tried

- Time `WaitUntilReady`'s parts (migrations, worker start) and log them.
- Narrow the body-wide `MutationObserver`.
- Preload the sounds instead of `new Audio()` per play.
- Slow down the attendance probe's DB read (cache the config until settings change).
- Drop the duplicate base64 logo from `index.html`.
- Try GPU rasterisation on a till that has a working GPU.
