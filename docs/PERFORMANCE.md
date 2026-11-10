# Performance

Speed and size work, newest first (see the global rules). Each entry: **Symptom**,
**Finding** (measured, or say it wasn't), **Change**, and **Not done** if relevant.

## Known slow spots

Measured or reported slow things that are not fixed yet. Nothing recorded yet.

## Entries

### 2026-11-10 · Expense, loan and transfer photos loaded slowly every time

**Symptom.** Opening Expenses, Loans or Transfers, or a record's detail, waited on kashi for every thumbnail and photo, each session; offline they never showed.

**Finding.** Every photo was a round trip to kashi (behind Cloudflare), kept only in memory for the session; reopening the app fetched them all again. Not timed on a till.

**Change.** `photo_cache.go`, `record_photos.go`: every photo is saved, encrypted, in one hidden SQLite file and shown from it after the first time; kashi is asked in the background (ETag, 304 when unchanged) and the page redraws only if it changed (`frontend/src/lib/recordPhotos.ts`, the three pages). Works offline for photos seen before.

**Not done.** Fetching recent records' thumbnails ahead of time, before anyone opens the page (in `FEATURE_IDEAS.md`).

### 2026-11-07 · Change Client and Change Salesperson felt slow on the invoice list

**Symptom.** Both dialogs opened empty and took a moment to become usable; after picking a client, Save stayed greyed out for a beat.

**Finding.** The backend isn't the cost: on the Centro till DB (64,326 clients) the first page of `SearchClients` takes 0.03 ms and a search matching nobody, the worst case (a full scan), 48 ms. `ListSalespersons` returns 7 rows. The waiting came from the frontend: Save was disabled during every load, picking a client set the search text to their name and started another search 200 ms later (disabling Save again), each open fetched from scratch, and the client field stayed blank until the first page arrived. Not timed on a till.

**Change.** `frontend/src/components/InvoiceChangeDialog.vue` and `frontend/src/lib/invoiceEditOptions.ts`: the invoice list prefetches both lists on mount and dialogs open with the last copy, then refresh it; the current client or salesperson is named from the list row before anything loads; Save depends only on the choice (off until it changes), not on a load in flight; picking a client no longer starts a search; typing searches after 150 ms instead of 200 ms; a slower, older search can't overwrite a newer one.

**Not done.** An index for client search: `LIKE '%…%'` can't use one, and 48 ms worst case isn't worth an FTS table.

### 2026-11-06 · A new cart line showed its deal chip a beat late

**Symptom.** Adding an item made the line appear first; its B2G1 chip and the other lines' updates came noticeably later.

**Finding.** The Go quote (`QuoteInvoiceTotal`) takes about 0.5 ms for a one-line cart on a dev machine, so the backend isn't the cost. The frontend started the request from a `setTimeout(0)`, which on a busy webview can run after a repaint. Not measured on a till.

**Change.** `frontend/src/pages/home.vue`: the quote is queued with a microtask in the same tick as the cart change, so the request leaves before the line's first paint.

**Not done.** Timing `FindProductsForScan` and the per-scan re-render on a real till, if it still feels slow.
