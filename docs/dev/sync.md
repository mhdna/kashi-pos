# Sync with central

Central is the central server (`~/Code/kashi`, Go + Postgres). A till works
alone when no central URL is set (standalone mode) and syncs with central once
one is. Code: `sync_*.go`, `commands.go`, `live_cart.go`, `standalone.go`,
`clock.go`; the screen side is `SyncIndicator.vue`, `lib/linkStatus.ts`,
`composables/useLinkStatus.ts` / `useSyncStatus.ts`, `stores/resync.ts`,
`components/shifts/CloseShiftSyncDialog.vue` and the Sync card in
`pages/settings.vue`.

## Turning it on

| Setting (Settings → General → Central sync) | Notes |
|---|---|
| Central server URL | e.g. `https://cloud.grandbz.com`. Empty = standalone. |
| Branch code + Central API key | Sent as the ==`X-Branch-Code` / `X-Branch-Key` headers== on every request (central's `branchAuthMiddleware`: a non-expiring branch credential, not a user token). |

- Settings are ==read fresh on every pass== (`syncClient`), so turning sync on or off, or changing the key, needs no restart. Saving kicks the push loop and the websocket, which redials at once; a change made another way (a command, a DB edit) is noticed within ==10 s== (`wsConfigRecheckInterval`).
- Sync is "configured" only when URL, branch code and key are all set (`GetSyncStatus.configured`).
- Every request has a ==10 s timeout== (`newCentralClient`). The close-shift flush uses its own transport with more connections.

## Who owns what

| Data | Owner | On the till |
|---|---|---|
| Products, variants, prices, price lists, discount lists, deals, invoice discounts | central | Read-only mirror. ==Products are never pushed up.== |
| Clients | central | Mirror. While connected the till can still create a client and change its name and phone (sent up), but deleting or deactivating one is refused. |
| Salespersons, branch users, categories, coupon types, coupons, currencies, payment methods, branches, branch stock, calendar | central | Mirror. |
| Invoices, exchanges, settlements, return edits, shifts, expenses, loans, visitor counts, attendance, transfers requested here | the till | Pushed up. |
| Settings | both | The till reports them; central can push changes with `update_settings`. |

In standalone mode the till owns products and clients and edits them
itself. Every product and client carries a `uid` either way, so a standalone
till that connects later adopts central's identities without collisions.

**Identity rule:** sync matches rows on ==`uid` (UUID), never on integer ids==;
the ids on the till and on central are separate number spaces. Phone (clients)
and barcode (products) are only the fallback for adopting central's uid on old
rows. Lookup tables without a uid (currencies, cashbox accounts) match on
their natural key (code, name). Records the till creates carry a `client_ref`
(`shift-12`, the invoice's ref …), and central is idempotent on
`(branch, client_ref)`: ==sending the same row twice is harmless==.

**Never delete what history points at.** A product, client, payment method
or salesperson that an invoice, coupon, transfer, stock row or account entry
refers to is deactivated, not deleted. Only a row nothing refers to is really
removed. While connected, only central deactivates.

## The background loops

All start at launch (`startSyncWorker`, `startWSClient`) and do nothing,
cheaply, while sync isn't configured.

| Loop | Runs | Does |
|---|---|---|
| Push | On a ==kick right after every local write==, plus a ==5 s backstop== tick | Drains `sync_outbox`. Nothing else, so a sale is never stuck behind a catch-up. |
| Pull | Every ==15 s== tick, but see the next table | Settings report, commands, the pull, targets, clock, receipt photos, the completeness check. |
| Failed-row retry | At start and ==every hour== | Puts every `failed` outbox row back to `pending`. |
| Websocket | Always, reconnecting ==2 s, 4 s … at most 60 s== | Central's push channel; a separate worker runs what it asks for. |
| Internet probe | Every ==20 s==, only while central is silent (or not proven for 40 s) | TCP to 1.1.1.1 / 8.8.8.8 / 9.9.9.9, 3 s each. |
| WhatsApp pending | Every minute | Retries thank-you messages through central: 1, 2, 4 … at most 30 min apart, for 6 h or 30 tries. |
| Updates | Every 30 min, and on central's `app_update` nudge | See releases-and-builds. |

What one pull tick does depends on the link (`runPullTick`):

| State | Every 15 s tick |
|---|---|
| Push link backing off (central unreachable) | ==Nothing at all==: every request would only sit through its timeout. |
| Websocket up and central said `sync_changed` in its hello | Settings report, salesperson ids and receipt photos every tick; ==targets every 1 min==; the full pull and commands only as a ==5 min backstop== (pushes already deliver them within a second). |
| Otherwise (no live link, or an older central) | Everything, every tick. |

Clock check: ==every 10 min== (`clockCheckInterval`, central's time and zone
ride on `/branch/health`). The till adopts the branch's time zone and warns the
cashier when the PC is ==more than 2 min off==.

## Pushing up: the outbox

::: diagram outbox

- Every write that central must know about queues a `sync_outbox` row ==in the
  same transaction== as the write. A crash can't leave a sale without its
  sync row, or the other way round.
- A row is `pending`, `synced` (stamped with ==`synced_to`, the server that took it==) or `failed`.
- **Order** (`ListDueSyncOutboxEntries`), FIFO inside each tier:
  1. sales, returns, settlements;
  2. shift closes, expenses, loans, visitor events, attendance;
  3. everything else (clients, categories, transfers …).
- One pass takes up to ==200 due rows==. A row queued while a pass runs
  ==preempts it==: the pass stops after its current row and the next one
  starts from the top, so a fresh sale goes first. A full page kicks another
  pass at once.
- **Batching:** all due visitor-counter presses go in ==one POST==
  (`/branch/visitor_events_batch`); two or more client rows go as one
  `/branch/clients_batch` (an older central without it gets them one by one).
  A lone client row (a buyer added at checkout) goes the plain way. Both
  answer with central's canonical uid for the client, which the till adopts.
- Expense, loan and transfer answers carry a ==one-time photo-upload token==:
  the screen gets it as an event and shows the QR code.

**What happens when a push fails** (`recordSyncOutboxError`, `isRowRejection`):

| Answer | What happens |
|---|---|
| No answer (no route, DNS, refused, ==timeout==) | ==Not about the row==: it stays due, keeps its attempt count, the pass stops, and the push link backs off ==5 s, 10 s, 20 s … at most 1 min== (`pushLink`). The next try first calls `/branch/health`; any answer (==even a 401==) releases ==every pending row at once==. |
| 5xx, 408, 429 | Only that row backs off: ==5 s × 2^attempts, at most 1 h==. The others carry on. |
| 401 / 403 | Retried like a 5xx: it is the credentials (or central's database starting), ==not the row==; parking it would strand every sale rung up meanwhile. |
| 404 on `expense_cancel`, `loan_cancel`, `expense_from_central` | Retried: central answers 404 until the record it points at has arrived. |
| Any other 4xx, or an entity type with no route | `failed`. ==Retried hourly== anyway, in case central was fixed. |

**Upload before closing** (`close_shift_requires_sync`, ==off by default==,
central can push it): a shift can't close, and its receipt isn't printed,
while anything is still `pending`. The close screen runs `FlushSyncOutbox`
and shows its progress:

- it ==ignores every back-off== (the connection's and each row's);
- sends ==8 rows at a time==, in dependency waves: sales, then returns and
  settlements, then the other transactional rows, then the rest;
- keeps the live push loop out, so nothing is sent twice;
- retries rows central answered with a 5xx for 4 rounds, then reports them.

**Cancel** stops after the rows in flight; nothing is lost. A genuinely
offline branch can close with the ==admin password and a reason==
(`BypassCloseShiftSyncGate`, logged); the rows upload later.

## Pulling down

- A separate loop, so an outgoing sale never waits behind a catch-up.
- Entities, in this order (`pullEntities`): currencies, cashbox_accounts, products, price_lists,
  discount_lists, clients, branch_targets, salespersons, branch_users,
  expense_categories, loan_categories, coupon_types, coupons, branches,
  transfer_requests, incoming_transfer_requests, branch_stock, quick_access,
  invoice_discounts, calendar_events, calendar_settings.
- Each follows central's `updated_at` cursor (`sync_cursors`), which ==only
  advances after a whole successful walk==, in the last page's transaction. So every apply must be
  idempotent: a failure mid-walk re-applies the same pages next time. One
  entity failing doesn't stop the others.
- Products and clients come in ==pages of 500==, each page its own transaction,
  so a first pull of a big catalogue isn't one long write lock.
- **Whole-set entities** (currencies, cashbox accounts, branches, quick access,
  calendar events and settings, expense and loan categories) always ask with an
  empty cursor and arrive complete; ==what central didn't send is retired==. Only a real difference in the
  local table counts as "changed", so open pages don't reload every 15 s.
- Price and discount lists arrive **whole** (the list with all its items), and
  the till writes only the rows that differ. Whole is the only way a removed item
  can disappear, since a delta can't say "this item is gone".
- Payment methods and currencies: a row central didn't send is ==deactivated,
  never deleted== (payments, expenses and shift balances point at it). Two
  things central can never touch: the ==`On Account` and `On Coupon`== placeholder methods (the code finds them by
  exact name), and the ==default currency==, which every stored amount is in.
- `calendar_events` (the Events page) arrive as the whole set too, and what central
  drops is deleted. The built-in retail calendar is not synced: it is a copy in
  `frontend/src/lib/retailCalendar.ts`, to keep in step with central's. The same
  events also show as a bar on the Shifts page
  (`components/shifts/EventsCard.vue`).
- A first pull on a till that already holds central's clients or products (a
  copied database, a new register) ==doesn't re-download them==:
  `seedCursorFromLocalCatalog` runs the completeness check for that entity and
  starts the cursor from central's clock, read before it.
- After an entity changed, the screen gets `sync_applied:<entity>`.
- A price or discount list central switches off keeps its row here, off and
  emptied, because sold lines point at it. Deleting one with sales fails and
  ==rolls back the whole page==, so nothing else in it arrives.

**The completeness check** (`sync_full_check.go`): the cursor only sees
changes, so a row the till never got would stay missing forever. ==Every 6 h==
(retried after 30 min if it failed) this pass compares all client and product
uids with central's (`/branch/sync/ids`, `/branch/sync/fetch`) and fetches
anything missing or older. Local products central doesn't have are
deactivated ==only once full-mirror mode was confirmed== in the Sync
everything dialog. Its first run also adopts central's uids for clients this
branch linked before uids existed.

## Live link (WebSocket)

`/branch/ws` (`sync_ws.go`). Push only, and every message is ==a pointer, never a
payload== ("go re-fetch X"), so a push and a poll can never disagree on how a
change is applied, only on how fast it is noticed.

| Message | The till |
|---|---|
| `hello` | Notes central's features: ==`sync_changed`== (it pushes every change, so polling slows down) and `quick_access`. |
| `sync_changed` (with `entities`), `products_updated`, `client_updated` … | Pulls just those entities. |
| `pull_all` | A full pull pass. |
| `command` | Fetches and runs pending commands. |
| `app_update` | Checks for an update now. |
| `expense_images_uploaded`, `loan_images_uploaded` | Closes that record's QR dialog. |
| `review` | Shows a client's rating of a sale. |

- On every connect the till ==pulls everything once== (it may have missed
  pushes while away) and re-sends its settings report.
- Both sides ping: ==ping every 30 s, dead after 60 s without a pong==, so a
  silently dropped connection is noticed within a minute, not whenever the OS
  gives up.
- The session only reads, pings and queues; pulls and commands run on a
  ==separate worker==. A pull can wait minutes behind Sync everything, and a
  session stuck behind it would stop answering pings, so central would show the
  branch offline exactly while it is busiest.
- Losing the websocket ==doesn't stop sync==: the 15 s poll does everything,
  only slower to notice.

## What the till shows of the connection

::: diagram linkGrace

The nav-rail light (`SyncIndicator.vue`, tap for the detail) keeps four facts
apart, from one tracker (`sync_link_status.go`, `GetLinkStatus`, event
`link_state_changed`, at most ==~4 per second==):

| Fact | Where it comes from |
| --- | --- |
| Internet | Any answer from central proves it. While central is silent, a TCP probe every 20 s tells "no internet" from "central down". |
| central | `linkTransport` wraps every central request: any HTTP answer (even 500) is "up", only no answer is "down". ==The screen shows "down" only after 30 s of silence== (`linkDownGrace`), for central and the internet alike, so a Wi-Fi blip never turns the light red. |
| Upgrade | 502/503/504, or Cloudflare's 521-523, mean central's proxy is up but the app behind it is being swapped. The light reads **Upgrading** (blue, spinning) instead of "Not live"/"No central"; the Live link row says "Waiting for the upgrade". Any normal answer ends it, and a gateway answer not renewed for ==10 min== stops being believed. |
| Live link | The websocket. This is what central's Branches page calls connected, so a till can be fully synced yet not live: it reads "Not live" (amber), not "Synced". |
| Activity | The rail shows only the pull step's name ("Clients") or "N of M" records being sent. The full line is in the tooltip and the dialog. Only a push has a real percentage. After work stops, the calm states (Synced, and Waiting with records queued) wait ==2.5 s== (`ACTIVITY_HOLD_MS`) before they show, so passes in a row read as one; a problem shows at once. |

**The 30 s grace** (`linkDownGrace`, `shownKashi`, `shownInternet`):

- The tracker records a failure ==at once== (the probe and the push back-off
  need it); only what `snapshot` hands the screen waits.
- Inside the grace the last good state stands: "up", or "unknown"
  (connecting) if central never answered since start. No error text and no
  "down for" until the grace is over.
- The outage starts at ==whichever went silent first==, so a real one goes
  straight to "No internet", never "No central" on the way.
- A timer asks the screen to refetch when the grace ends, so the red shows on
  time even if no other request fails. The first answer clears it at once.
- Why: shop Wi-Fi stalls for a few seconds now and then (measured: TCP connect
  to Cloudflare from 0.02 s to over 10 s), and one lost request used to paint
  the light red.

Precedence, top wins (`lib/linkStatus.ts`, tested over every combination):

| Light | When |
|---|---|
| Off | Sync not configured. |
| No internet (red) | Internet and central both down (after the grace). |
| No central (red) | Central down, internet fine. |
| Upgrading (blue) | Central answering with gateway errors. |
| Syncing / Updating / Sending (blue) | Sync everything, a pull, a push running. |
| Check (amber) | Rows central refused (`failed`); they're safe on the till. |
| Waiting (calm) | Rows queued. |
| Not live (amber) | Websocket down while central answers. |
| Connecting (blue) | Central hasn't answered yet since start, and no live link. |
| Synced (green) | None of the above. |

The settings card's "fully synced" (`GetSyncStatus`) is stricter: configured,
==nothing pending, nothing failed, and nothing still stamped for a previous
server==.

`sync_verbose` (off by default, central can push it): the light shows every
pull step, even one that finds nothing new. Quiet shows a step only once it
really received something different, and keeps it up at least 1.5 s.

## Commands from central

Central queues commands per branch (`branch_commands`, status `pending`). The
till fetches them (`/branch/commands/pending`), runs them and acks each one
(`/branch/commands/{id}/ack`, success, or failed with the error);
==one failing doesn't block the others==. Central's table is the only queue, so
nothing can get stuck on the till. They arrive on the `command` push, or on the
pull tick.

| Command | Does |
|---|---|
| `update_settings` | Change settings/flags. ==Every field is optional==: an absent one is left as it is (central sends partial payloads, and old queued commands lack newer fields). |
| `remote_return_invoice` | A return or exchange started in central. `print_receipt: false` never prints it; absent or true follows **Print return receipts**. |
| `edit_return_invoice` | Change a return made earlier. |
| `set_branch_user_pin` | Set a user's login PIN. |
| `print_shift_report`, `reprint_invoice` | Print on the till. |
| `sync_now` | Kick the push loop now. |
| `transfer_decision` | A transfer approved/rejected. |
| `cancel_record`, `set_expense_from_central` | Expense/loan changes made in central (applied without echoing back). |
| `review_attendance_request` | Approve/reject an attendance request. |
| `replace_shift_balance` | Correct a closed shift's balances. |
| `cart_override` | Central's changes to the sale on screen (see Live cart below). |

`update_settings` can **set** a few secrets it can never **read** back:
the attendance device password, the approver password and the local admin
password. ==It can't touch the branch code, API key, theme or register
number.==

## Settings report

The till sends its settings to central (`PUT /branch/settings`, `pushSettings`)
on the pull tick, but ==only when the report's hash changed==, on every new
websocket session, and ==once an hour anyway== so central's copy heals itself.
Central stores it per register to show what each till runs.
==Never sent:== the API key, theme, register number, the attendance device
password and the manager password hash. Central can't change what it can't name.

Flags that default to on must be read with ==`flagEnabledDefaultOn`== when
building the report. Reading them with `flagEnabled` reports a never-touched
till as off while it behaves as on, and central then pushes the wrong value
back on the next save.

Some settings are deliberately till-only and not in the report at all, for
example `release_stuck_keys` (see `windows-till.md`).

## Live cart

The sale screen reports its open sale (`ReportLiveCart`, `live_cart.go`) on
every change and every 20 s, and the till forwards it to central's
`POST /branch/live_cart` (8 s timeout). ==It's a view, not a record==: nothing goes through the
outbox, and a failed report is just replaced by the next. Central keeps only
the latest per register, in memory.

From central's app bar (the cart button) someone with pricing rights can send a
`cart_override` for that sale: turn every deal off, or give a line (by
barcode, or an unknown item by its line ref) a price, a discount or no deal. These go ==past the branch's
settings==: `validateAndPriceItems` applies them itself whatever
`custom_item_prices`, `custom_item_discounts`, `price_edit_restricted` and
the toggles say, so the quote and the checkout both charge them. The screen
marks those lines "kashi" and locks them.

- Overrides belong to one sale, named by a ref the screen makes per sale (kept
  in the draft). A report of another sale drops them; a command for a sale
  that's gone fails, and central shows it.
- Only a plain sale takes them, never an exchange or settlement.
- Unknown items all share one barcode, so their overrides go by the line's
  `line_ref` (the cart's `unknownRef`, sent in the report as `lineRef`). Central
  can give one a price, a discount, or a deal choice (`unknown_deal`:
  `b2g1_30`, `b2g1_20`, `none`); a discount still takes it off the deal.
  A line with no ref (an old draft or hold) can't be named.
- They live in memory: ==a till restart drops them== and the lines go back to
  their own prices on the next report.
- A held sale keeps them: `HoldInvoice` stores them with the hold, and
  `ResumeHoldInvoice` puts them back for the same ref, which the restored sale
  reports again. The hold list never shows them.

## Other things that talk to central

All go through the same client, so they all feed the light.

| What | Route | When |
|---|---|---|
| Health and clock | `GET /branch/health` | First push after start or after a failure; the clock every 10 min. |
| Target progress | `GET /branch/targets/progress` | Every tick, or every minute when central pushes changes. |
| Shift receipt photo | `POST /branch/shift_receipt_images` | Pull tick, ==only after the shift close itself has synced==; never gives up, resumes after a restart. |
| Expense / loan / transfer photos | `/branch/expense_photos`, `/branch/loan_photos`, `/branch/transfer_photo`, `…/upload_status` | Viewing a record's photos; the upload status is polled while its QR is up. |
| Unknown-item pictures | `/branch/unknown_sessions…` | The sale screen's picture corner. |
| WhatsApp thank-you / welcome | `/branch/whatsapp/thank_you`, `/welcome` | After a sale or a new client; pending ones retry (see the loops table). |
| Salesperson ids of past sales | `POST /branch/invoice_salespersons` | Once per till, in batches. |
| Quick access import | `POST /branch/quick_access/import` | Once, with a non-empty list, when central supports it; central keeps the first one per branch. |
| Sync report | `POST /branch/sync/report` | After a pull pass that had something to say. |
| Sync everything start/end | `/branch/sync/full_sync/start`, `/end` | During Sync everything. |

## Sync report

After each pull pass the till sends `POST /branch/sync/report` (`sync_report.go`), for central's Commands page: each entity that failed or applied something, an error that has cleared, and its outbox counts. A pass with nothing new sends nothing, and a long paged pull also sends a running count. ==It never fails a sync==, and an older central just answers 404.

It also sends a fingerprint of each list in force, whenever one changes: its row count and a hash of its rows, with the same recipe as central's `api/list_fingerprint.go`, so central can check what the till really holds. ==Change one and you must change the other==; a test in each repo pins the same values.

## Sync everything

Settings → Central sync → **Sync everything** (`ResyncAll`, `stores/resync.ts`).
Brings the till to full parity with the server it points at, both ways. ==Safe to re-run, and
the thing to run after changing the central URL.== It owns the database meanwhile: the
pull tick stands down and the websocket's pulls wait, because SQLite has one
writer ("database is locked (517)" otherwise). The push loop keeps going, so a
live sale still reaches central.

1. Tell central it started (a ==2-minute lease, renewed every 30 s==, so a run that dies stops showing as syncing).
2. Adopt central's uids for this branch's clients.
3. Clear the cursors and pull everything. Against the ==same server== as last time, clients and products keep their cursors (the completeness check covers them); a different server clears all.
4. Products central doesn't have: the ==first time== any are found, the run stops and lists them for confirmation; once confirmed (full-mirror mode, `full_mirror_confirmed_at`) later runs deactivate silently.
5. Ask central which clients, invoices, expenses, loans, shift closes, visitor events and attendance it ==doesn't== have (`/branch/sync/reconcile`, in batches) and queue only those. Old sales go up as historical records that ==don't move central's stock==, unless **Also apply stock movements** is ticked.
6. Re-arm outbox rows ==a previous server== had confirmed, so they go to this one.
7. Run the completeness check, stamp the target, kick the push loop.

**Dry run** reports all of it without writing anything.

## Moving a till to another server

Rows are stamped with the server that took them (`synced_to`). Point the till
at a new URL and run Sync everything: cursors from the old server mean
nothing there and are cleared, and rows confirmed only elsewhere are re-armed.
Until then ==the settings card is never "fully synced"== (`confirmedElsewhere` > 0).

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
  outbox, in one fixed row, so ==only the latest flip== is sent.

## Things that rely on central's version

Newer features wait for central to announce them in its `hello` (for example
`quick_access`, `sync_changed`), and an older central answering 404 on a route (like
`/branch/app_update`, `/branch/sync/report`, `/branch/clients_batch`) makes that feature fall back or stay quiet instead of erroring.

## Re-provisioning a branch

`PullBranchHistory` (Settings → Import branch history from central) imports a branch's own past invoices, expenses, loans
and shift closes from central into a ==fresh, empty== database (it refuses if
any of those tables have rows), seeding central's refs so nothing is pushed back.
It's best effort: payment-account identity and lines without any barcode or
uid can't be reconstructed exactly.

## Salesperson ids

Central used to get only the salesperson's name on a sale, so renaming one split
them in two. Sales now carry central's salesperson id, and each till reported
the ids of all its past sales once in bulk.

## Debugging a till that won't sync

1. Tap the light: the dialog names which fact is down and since when.
2. `sqlite3 <db> "select entity_type, status, attempt_count, last_error, next_attempt_at, synced_to from sync_outbox where status != 'synced' order by id"`: what is stuck, and why.
3. `select * from sync_cursors` (pull progress) and `select * from sync_state` (target, last Sync everything and completeness check).
4. Central's Commands page shows the till's sync reports: which pull entity fails and why.
5. Log lines start `sync outbox:`, `sync pull <entity>:`, `ws client:`, `sync commands:`. (A Windows till has no log file; see the overview.)

## Tests

| Area | Tests |
|---|---|
| End to end against a fake central | `e2e_sync_test.go`, `e2e_branch_history_test.go` |
| Outbox, push link, flush | `sync_outbox_test.go`, `sync_push_link_test.go`, `sync_worker_test.go`, `sync_flush_test.go`, `sync_clients_batch_test.go` |
| Pull and lists | `sync_pull*_test.go`, `sync_full_check_test.go`, `sync_reconcile_payment_test.go` |
| Websocket | `sync_ws_test.go` |
| The light | `sync_link_status_test.go`, `sync_link_quiet_test.go`, `sync_link_grace_golden_test.go` (writes `frontend/src/lib/testdata/link_grace.json`), `frontend/src/lib/linkStatus.test.ts` |
| Report, settings, commands | `sync_report_test.go`, `commands_*_test.go` |
