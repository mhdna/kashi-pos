# Sync with kashi

kashi is the central server (`~/Code/kashi`, Go + Postgres). A till works
alone when no central URL is set (standalone mode) and syncs with kashi once
one is. Code: `sync_*.go`, `commands.go`, `standalone.go`.

## Who owns what

| Data | Owner | On the till |
|---|---|---|
| Products, variants, prices, price lists, discount lists, deals, invoice discounts | kashi | Read-only mirror. |
| Clients | kashi | Mirror. While connected the till can still create a client and change its name and phone (sent up), but deleting or deactivating one is refused. |
| Salespersons, branch users, categories, coupon types, currencies, payment methods | kashi | Mirror. |
| Invoices, exchanges, shifts, expenses, loans, attendance, transfers requested here | the till | Pushed up. |
| Settings | both | The till reports them; kashi can push changes with `update_settings`. |

In standalone mode the till owns products and clients and edits them
itself. Every product and client carries a `uid` either way, so a standalone
till that connects later adopts kashi's identities without collisions.

**Identity rule:** sync matches rows on `uid` (UUID), never on integer ids;
the ids on the till and on kashi are separate number spaces. Phone (clients)
and barcode (products) are only the fallback for adopting kashi's uid on old
rows. Lookup tables without a uid (currencies, cashbox accounts) match on
their natural key (code, name).

**Never delete what history points at.** A product, client, payment method
or salesperson that an invoice, coupon, transfer, stock row or account entry
refers to is deactivated, not deleted. Only a row nothing refers to is really
removed. While connected, only kashi deactivates.

## Pushing up: the outbox

- Every write that kashi must know about queues a `sync_outbox` row **in the
  same transaction** as the write. A crash can't leave a sale without its
  sync row, or the other way round.
- The push loop drains the outbox: every 15 s, plus a 5 s backstop, plus a
  "kick" right after a local write.
- **A network failure is about the connection, not the row** (`pushLink`): the
  pass stops at the first one and the whole loop waits (5 s, 10 s, … up to a
  minute). The first pass that gets through makes every pending row due at
  once. A real answer from kashi (5xx, 408, 429) backs off just that row.
  Rows that failed for good are retried hourly.
- Rows are stamped with the server they're meant for. The nav-rail sync dot
  is only green when sync is configured, nothing is pending, nothing failed,
  and nothing is still stamped for a previous server.
- `close_shift_requires_sync` (off by default, kashi can push it): a shift
  can't close, and its receipt isn't printed, while anything is still in the
  outbox. The close screen flushes it first.

## Pulling down

- A separate loop, so an outgoing sale never waits behind a catch-up.
- Entities, in order: currencies, cashbox_accounts, products, price_lists,
  discount_lists, clients, branch_targets, salespersons, branch_users,
  expense_categories, loan_categories, coupon_types, coupons, branches,
  transfer_requests, incoming_transfer_requests, branch_stock, quick_access,
  invoice_discounts.
- Each follows kashi's `updated_at` cursor, which only advances after a whole
  successful walk. So every apply must be idempotent: a failure mid-page
  re-applies the same payload next time.
- Price and discount lists arrive **whole** (the list with all its items), and
  the till clears and refills them. That's the only way a removed item can
  disappear, since a delta can't say "this item is gone".
- Payment methods and currencies also arrive as the whole set, so a row kashi
  didn't send is retired (deactivated). Two things kashi can never touch:
  the `On Account` and `On Coupon` placeholder methods (the code finds them by
  exact name), and the default currency, which every stored amount is in.
- **The completeness check** (`sync_full_check.go`): the cursor only sees
  changes, so a row the till never got would stay missing forever. This pass
  compares all client and product uids with kashi's and fetches anything
  missing or older.
- Backstop pull every 5 minutes; target progress every minute.

## Live link (WebSocket)

kashi pushes nudges over a WebSocket (`sync_ws.go`): "something changed,
pull now", "a command is waiting", "an update is available". It's push only
and can sit idle for a long time, so both sides ping; a dead connection is
noticed within the pong wait instead of whenever the OS gives up.

## Commands from kashi

kashi queues commands per branch (`branch_commands`, status `pending`). The
till fetches, runs and acks each one; one failing doesn't block the others.
kashi's table is the only queue, so nothing can get stuck on the till.

| Command | Does |
|---|---|
| `update_settings` | Change settings/flags (only the ones kashi has a name for). |
| `remote_return_invoice` | A return started in kashi. |
| `set_branch_user_pin` | Set a user's login PIN. |
| `print_shift_report`, `reprint_invoice` | Print on the till. |
| `sync_now` | Run a sync pass now. |
| `transfer_decision` | A transfer approved/rejected. |
| `cancel_record`, `set_expense_from_central` | Expense/loan changes made in kashi (applied without echoing back). |
| `review_attendance_request` | Approve/reject an attendance request. |
| `replace_shift_balance` | Correct a closed shift's balances. |

## Settings report

The till sends its settings to kashi (`pushSettings`), re-sending an unchanged
report every hour so kashi's copy heals itself. **Never sent:** the API key,
theme, register number, the attendance device password and the manager
password hash. kashi can't change what it can't name.

Flags that default to on must be read with `flagEnabledDefaultOn` when
building the report. Reading them with `flagEnabled` reports a never-touched
till as off while it behaves as on, and kashi then pushes the wrong value
back on the next save.

Some settings are deliberately till-only and not in the report at all, for
example `release_stuck_keys` (see `windows-till.md`).

## Two writers for the same value

Where both the till and kashi can change something, the later change wins:

- **Display prefs** (quick access rows, text size, theme…): the screen keeps
  them in localStorage and mirrors them to `settings.display_prefs`; a push
  from kashi bumps `display_prefs_rev`, and the screen adopts the stored prefs
  whenever the rev is newer than the one it last applied.
- **Quick access mode** (`till` or `kashi`): each side keeps when its value was
  chosen and only adopts a newer one. A flip made offline goes through the
  outbox.

## Things that rely on kashi's version

Newer features wait for kashi to announce them in its "hello" (for example
`quick_access`), and an older kashi answering 404 on a route (like
`/branch/app_update`) makes that feature stay quiet instead of erroring.

## Re-provisioning a branch

`PullBranchHistory` imports a branch's own past invoices, expenses, loans
and shift closes from kashi into a **fresh, empty** database (it refuses if
any of those tables have rows), seeding kashi's refs so nothing is pushed back.
It's best effort: payment-account identity and lines without any barcode or
uid can't be reconstructed exactly.

## Salesperson ids

kashi used to get only the salesperson's name on a sale, so renaming one split
them in two. Sales now carry kashi's salesperson id, and each till reported
the ids of all its past sales once in bulk.
