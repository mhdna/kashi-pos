# Shifts, logins and people

Code: `shifts.go`, `shift_auto_swap.go`, `shift_upload_server.go`,
`sync_shift_receipts.go`, `spend_summary.go`, `app.go` (logins),
`salespersons.go`, `auth_credentials.go`.

## Shifts

- A sale needs an open shift. An expense or loan doesn't (paying a bill
  before the first shift opens is normal).
- **Stale shifts:** with `auto_close_stale_shifts` on (off by default), a shift
  opened on an earlier day is closed when the app starts, without counts or a
  closing person. One with no activity at all is **deleted** instead, so empty
  $0 shifts never show up or reach kashi.
- **Opening float** (`opening_float_enabled`, on by default): the cash put in
  the drawer at the start. Changing the default only affects shifts opened
  later; `ApplyOpeningFloatToCurrentShift` changes the open one. Turning the
  feature off removes the float from every total, receipt and balance.
- **Counting at close** (`count_opening_cash_on_close`, on by default): the
  cashier counts the whole drawer, float included. Off: they count only what
  came in during the shift, and the float is left out of both expected and
  printed figures so the difference still lines up.
- **Negative LBP at close** (`auto_cover_negative_lbp`, off by default): if
  more LBP went out as change than came in, the shortfall is swapped into USD
  cash so LBP ends at 0.
- **Close-shift receipt photo** (`settings.shift_receipt_upload_mode`, an enum,
  default `hybrid`): a QR code on screen; a phone photographs the printed
  shift receipt.
  - `central`: the till posts the close to kashi straight away, gets a one-time
    token, and the QR points at kashi.
  - `local`: the QR points at the till's own server (`0.0.0.0:8788`,
    `KASHI_SHIFT_UPLOAD_LISTEN`); the photo is forwarded to kashi later.
  - `hybrid`: central, falling back to local if kashi can't be reached.
- **Monthly total on the shifts page** (`GetMonthSales`): counted the way
  kashi's reports count net sales, so the two agree: every sale by the local
  day it was rung up (not the shift it fell in), returns netted, cancelled
  sales and their refunds left out, coupons counted. Under the total it says
  how much of it is "calculate on checkout" coupons and how much is on
  account. Each day's figure is that day's sales; a shift row under a day
  still shows its own whole total. Branch-target progress is different: it
  still leaves those coupons out.
- **Goodbye** (`CloseShiftCard.vue`): once the shift is closed and the photo
  is in (or a manager skipped it, or none was needed), the closing salesperson
  gets a thank-you by name and the choice to shut the
  computer down (`ShutDownComputer`, as in the Quit dialog; refused in a
  development build) or just log out.
- The shift receipt's sign-off lines (`shift_report_signatures`, on) and
  capital-letter header (`shift_report_uppercase_header`, off) are switches.
- kashi can correct a closed shift's balances (`replace_shift_balance`) and
  reprint its report (`print_shift_report`).
- `close_shift_requires_sync`: see `sync.md`.

## Logins

- **Admin** and **cashier** logins. The cashier uses a shared 4-digit PIN on a
  keypad. Branch users (and their PINs) come from kashi; kashi can set a PIN
  remotely (`set_branch_user_pin`).
- **PIN lockout:** 3 wrong PINs lock the keypad for 30 s, and each further
  lockout doubles it (60 s, 2 min, …) up to 30 minutes. The state is kept in
  the database (`pin_lockout`), so restarting the app doesn't reset it: it
  just leads to a longer wait next time. The legacy kashi-admin can clear it
  (`unlockPINLockout`).
- Credentials are in `auth_credentials` (since migration 000083). The values the
  login screen shipped with before that are still the fallback when there's no
  database or no row.
- The admin password is hashed with plain unsalted SHA-256; the shared
  cashier PIN is stored as **plain text** (`auth_credentials.cashier_pin`).
- The **manager password** is the attendance approver's password (see
  `attendance.md`); it unlocks Expenses, Loans and Conversion for a non-admin.
- Non-admin page access can be locked per page (Settings → Page access; kashi
  can push it).

## Salespersons

- While connected, kashi owns the roster. A salesperson left over from before
  the till was connected can still be deleted, but not renamed.
- `salesperson_per_invoice_required` (off by default): every sale must name a
  salesperson.
- `client_required` (on by default): every sale needs a client, with an
  "Anonymous / prefer not to say" option.

## Conversion and visitors

- The nav-drawer people counter (`visitor_counter`) records walk-ins per day.
  Turning it off only hides it; the counts stay.
- The Conversion page divides each day's sales by its visitors, using only
  days that have a visitor count. Admins pick any range; a manager (manager
  password) always gets the current month so far.

## Targets

Revenue targets come from kashi, read-only. Progress is kashi's branch-wide
figure (all registers' synced sales) plus this till's sales kashi hasn't got
yet, so a sale moves the bar at once and is never counted twice. Only targets
overlapping the current month are shown.
