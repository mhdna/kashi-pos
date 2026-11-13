# Sync with central

Central is the central server (`~/Code/kashi`, Go + Postgres). A till works
alone when no central URL is set (standalone mode) and syncs with central once
one is. Code: `sync_*.go`, `commands.go`, `standalone.go`.

## Who owns what

| Data | Owner | On the till |
|---|---|---|
| Products, variants, prices, price lists, discount lists, deals, invoice discounts | central | Read-only mirror. |
| Clients | central | Mirror. While connected the till can still create a client and change its name and phone (sent up), but deleting or deactivating one is refused. |
| Salespersons, branch users, categories, coupon types, currencies, payment methods | central | Mirror. |
| Invoices, exchanges, shifts, expenses, loans, attendance, transfers requested here | the till | Pushed up. |
| Settings | both | The till reports them; central can push changes with `update_settings`. |

In standalone mode the till owns products and clients and edits them
itself. Every product and client carries a `uid` either way, so a standalone
till that connects later adopts central's identities without collisions.

**Identity rule:** sync matches rows on `uid` (UUID), never on integer ids;
the ids on the till and on central are separate number spaces. Phone (clients)
and barcode (products) are only the fallback for adopting central's uid on old
rows. Lookup tables without a uid (currencies, cashbox accounts) match on
their natural key (code, name).

**Never delete what history points at.** A product, client, payment method
or salesperson that an invoice, coupon, transfer, stock row or account entry
refers to is deactivated, not deleted. Only a row nothing refers to is really
removed. While connected, only central deactivates.

## Pushing up: the outbox

- Every write that central must know about queues a `sync_outbox` row **in the
  same transaction** as the write. A crash can't leave a sale without its
  sync row, or the other way round.
- The push loop drains the outbox: every 15 s, plus a 5 s backstop, plus a
  "kick" right after a local write.
- **A network failure is about the connection, not the row** (`pushLink`): the
  pass stops at the first one and the whole loop waits (5 s, 10 s, … up to a
  minute). The first pass that gets through makes every pending row due at
  once. A real answer from central (5xx, 408, 429) backs off just that row.
  Rows that failed for good are retried hourly.
- Rows are stamped with the server they're meant for. The nav-rail sync dot
  is only green when sync is configured, nothing is pending, nothing failed,
  and nothing is still stamped for a previous server.
- `close_shift_requires_sync` (off by default, central can push it): a shift
  can't close, and its receipt isn't printed, while anything is still in the
  outbox. The close screen flushes it first.

## Pulling down

- A separate loop, so an outgoing sale never waits behind a catch-up.
- Entities, in order: currencies, cashbox_accounts, products, price_lists,
  discount_lists, clients, branch_targets, salespersons, branch_users,
  expense_categories, loan_categories, coupon_types, coupons, branches,
  transfer_requests, incoming_transfer_requests, branch_stock, quick_access,
  invoice_discounts, calendar_events.
- Each follows central's `updated_at` cursor, which only advances after a whole
  successful walk. So every apply must be idempotent: a failure mid-page
  re-applies the same payload next time.
- Price and discount lists arrive **whole** (the list with all its items), and
  the till writes only the rows that differ. Whole is the only way a removed item
  can disappear, since a delta can't say "this item is gone".
- `calendar_events` (the Events page) arrive as the whole set too, and what central
  drops is deleted. The built-in retail calendar is not synced: it is a copy in
  `frontend/src/lib/retailCalendar.ts`, to keep in step with central's. The same
  events also show as a bar on the Shifts page
  (`components/shifts/EventsCard.vue`: how many fall in the next 30 days and the
  next one; tapping it opens the calendar page).
- Payment methods and currencies also arrive as the whole set, so a row central
  didn't send is retired (deactivated). Two things central can never touch:
  the `On Account` and `On Coupon` placeholder methods (the code finds them by
  exact name), and the default currency, which every stored amount is in.
- **The completeness check** (`sync_full_check.go`): the cursor only sees
  changes, so a row the till never got would stay missing forever. This pass
  compares all client and product uids with central's and fetches anything
  missing or older.
- Backstop pull every 5 minutes; target progress every minute.

## Live link (WebSocket)

Central pushes nudges over a WebSocket (`sync_ws.go`): "something changed,
pull now", "a command is waiting", "an update is available". It's push only
and can sit idle for a long time, so both sides ping; a dead connection is
noticed within the pong wait instead of whenever the OS gives up.

## What the till shows of the connection

The nav-rail light (`SyncIndicator.vue`, tap for the detail) keeps four facts
apart, from one tracker (`sync_link_status.go`, `GetLinkStatus`, event
`link_state_changed`, at most ~4/s):

| Fact | Where it comes from |
| --- | --- |
| Internet | Any answer from central proves it. While central is silent, a TCP probe to 1.1.1.1 / 8.8.8.8 / 9.9.9.9 every 20 s tells "no internet" from "Central down". |
| central | `linkTransport` wraps every central request: any HTTP answer (even 500) is "up", only no answer is "down". |
| Upgrade | `linkTransport` (and the websocket handshake) also read the status of each answer: 502/503/504, or Cloudflare's 521-523, mean central's proxy is up but the app behind it is being swapped. That sets `updating` (`isUpdateStatus`, `centralStatus`) and the light reads **Upgrading** (blue, spinning) instead of "Not live"/"No central"; the Live link row says "Waiting for the upgrade". Any normal answer ends it, and a gateway answer not renewed for 10 min stops being believed. A refused connection or no answer is still "not connected". |
| Live link | The websocket. This is what central's Branches page calls connected, so a till can be fully synced yet not live: it reads "Not live" (amber), not "Synced". |
| Activity | The rail shows only the pull step's name ("Clients") or "N of M" records being sent, in a round badge that breathes while busy. The full line (entity N of 21 and its change count) is in the tooltip and the dialog. Only a push has a real percentage: a pull's step count is not rows. After work stops, the calm states (Synced, and Waiting with records queued) wait 2.5 s (`ACTIVITY_HOLD_MS`) before they show, so passes in a row read as one; only spinning work restarts that hold, and a problem shows at once. |

The words and colours are worked out in `frontend/src/lib/linkStatus.ts`
(tested over every combination). Being offline outranks every other state.

## Commands from central

Central queues commands per branch (`branch_commands`, status `pending`). The
till fetches, runs and acks each one; one failing doesn't block the others.
central's table is the only queue, so nothing can get stuck on the till.

| Command | Does |
|---|---|
| `update_settings` | Change settings/flags (only the ones central has a name for). |
| `remote_return_invoice` | A return started in central. `print_receipt: false` never prints it; absent or true follows **Print return receipts**. |
| `set_branch_user_pin` | Set a user's login PIN. |
| `print_shift_report`, `reprint_invoice` | Print on the till. |
| `sync_now` | Run a sync pass now. |
| `transfer_decision` | A transfer approved/rejected. |
| `cancel_record`, `set_expense_from_central` | Expense/loan changes made in central (applied without echoing back). |
| `review_attendance_request` | Approve/reject an attendance request. |
| `replace_shift_balance` | Correct a closed shift's balances. |
| `cart_override` | central's changes to the sale on screen (see Live cart below). |

## Live cart

The sale screen reports its open sale (`ReportLiveCart`, `live_cart.go`) on
every change and every 20 s, and the till forwards it to central's
`POST /branch/live_cart`. It's a view, not a record: nothing goes through the
outbox, and a failed report is just replaced by the next. Central keeps only
the latest per register, in memory.

From central's app bar (the cart button) someone with pricing rights can send a
`cart_override` for that sale: turn every deal off, or give a line (by
barcode, or an unknown item by its line ref) a price, a discount or no deal. These go **past the branch's
settings**: `validateAndPriceItems` applies them itself whatever
`custom_item_prices`, `custom_item_discounts`, `price_edit_restricted` and
the toggles say, so the quote and the checkout both charge them. The screen
marks those lines "kashi" and locks them.

- Overrides belong to one sale, named by a ref the screen makes per sale (kept
  in the draft). A report of another sale drops them; a command for a sale
  that's gone fails, and central shows it.
- Only a plain sale takes them, never an exchange or settlement.
- Unknown items all share one barcode, so their overrides go by the line's
  `line_ref` (the cart's `unknownRef`, sent in the report as `lineRef`). central
  can give one a price, a discount, or a deal choice (`unknown_deal`:
  `b2g1_30`, `b2g1_20`, `none`); a discount still takes it off the deal.
  A line with no ref (an old draft or hold) can't be named.
- They live in memory: a till restart drops them and the lines go back to
  their own prices on the next report.
- A held sale keeps them: `HoldInvoice` stores them with the hold, and
  `ResumeHoldInvoice` puts them back for the same ref, which the restored sale
  reports again. The hold list never shows them.

## Settings report

The till sends its settings to central (`pushSettings`), re-sending an unchanged
report every hour so central's copy heals itself. **Never sent:** the API key,
theme, register number, the attendance device password and the manager
password hash. Central can't change what it can't name.

Flags that default to on must be read with `flagEnabledDefaultOn` when
building the report. Reading them with `flagEnabled` reports a never-touched
till as off while it behaves as on, and central then pushes the wrong value
back on the next save.

Some settings are deliberately till-only and not in the report at all, for
example `release_stuck_keys` (see `windows-till.md`).

## Two writers for the same value

Where both the till and central can change something, the later change wins:

- **Display prefs** (quick access rows, text size, theme…): the screen keeps
  them in localStorage and mirrors them to `settings.display_prefs`; a push
  from central bumps `display_prefs_rev`, and the screen adopts the stored prefs
  whenever the rev is newer than the one it last applied.
  `print_return_receipts` is the one pref the backend reads too
  (`returnReceiptsOn`), so a return central sends follows it.
- **Quick access mode** (`till` or `central`): each side keeps when its value was
  chosen and only adopts a newer one. A flip made offline goes through the
  outbox.

## Things that rely on central's version

Newer features wait for central to announce them in its "hello" (for example
`quick_access`), and an older central answering 404 on a route (like
`/branch/app_update`) makes that feature stay quiet instead of erroring.

## Re-provisioning a branch

`PullBranchHistory` imports a branch's own past invoices, expenses, loans
and shift closes from central into a **fresh, empty** database (it refuses if
any of those tables have rows), seeding central's refs so nothing is pushed back.
It's best effort: payment-account identity and lines without any barcode or
uid can't be reconstructed exactly.

## Salesperson ids

Central used to get only the salesperson's name on a sale, so renaming one split
them in two. Sales now carry central's salesperson id, and each till reported
the ids of all its past sales once in bulk.

## Reporting to central

After each pull pass the till sends `POST /branch/sync/report` (`sync_report.go`), for central's Commands page: each entity that failed or applied something, an error that has cleared, and its outbox counts. A pass with nothing new sends nothing, and a long paged pull also sends a running count. It never fails a sync, and an older central just answers 404.

It also sends a fingerprint of each list in force, whenever one changes: its row count and a hash of its rows, with the same recipe as central's `api/list_fingerprint.go`, so central can check what the till really holds. Change one and you must change the other; a test in each repo pins the same values.

A price or discount list central switches off keeps its row here, off and emptied, because sold lines point at it. Deleting one with sales fails and rolls back the whole pull, so nothing else in it arrives.
