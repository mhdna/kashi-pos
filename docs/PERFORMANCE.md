# Performance

Speed and size work, newest first (see the global rules). Each entry: **Symptom**,
**Finding** (measured, or say it wasn't), **Change**, and **Not done** if relevant.

## Known slow spots

Measured or reported slow things that are not fixed yet. Nothing recorded yet.

## Entries

### 2026-11-06 · A new cart line showed its deal chip a beat late

**Symptom.** Adding an item made the line appear first; its B2G1 chip and the other lines' updates came noticeably later.

**Finding.** The Go quote (`QuoteInvoiceTotal`) takes about 0.5 ms for a one-line cart on a dev machine, so the backend isn't the cost. The frontend started the request from a `setTimeout(0)`, which on a busy webview can run after a repaint. Not measured on a till.

**Change.** `frontend/src/pages/home.vue`: the quote is queued with a microtask in the same tick as the cart change, so the request leaves before the line's first paint.

**Not done.** Timing `FindProductsForScan` and the per-scan re-render on a real till, if it still feels slow.
