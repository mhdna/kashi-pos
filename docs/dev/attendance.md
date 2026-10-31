# Attendance

How punches get from the Akuvox door device into the till and on to kashi,
and the rules that decide which punches count. Code: `attendance.go`,
`attendance_push_server.go`, `attendance_probe.go`, `attendance_recorded.go`.

## Where a punch comes from

The till asks the device; the device never sends punches by itself.

| When the till pulls | What triggers it |
|---|---|
| Right after a punch | The device calls `http://<till>:8789/attendance/ping` ("Action URL" → "Relay Triggered" on the Akuvox). The till waits 0.5 s (the log entry may not exist yet), pulls, and retries once 1.5 s later if nothing new came in. |
| When the till starts | One pull, to catch punches made while it was off (the last person out after closing). |
| Every 60 s while the Attendance page is open | The page's own refresh. |
| Nothing else | There is **no background timer**. If the ping isn't set up on the device and nobody opens the Attendance page, punches wait for the next restart. |

- The ping carries no data and no trust. It only wakes a pull of the device
  the till already polls, and it's refused (403) unless it comes from the
  configured device IP.
- The ping listener address is `0.0.0.0:8789`, overridable with
  `KASHI_ATTENDANCE_PUSH_LISTEN`.
- The red/green attendance dot on the nav rail is a separate, cheap TCP
  connect to the device every second (`attendance_probe.go`). It says nothing
  about whether punches are being imported.
- Pulls are serialized (`attendanceSyncMu`), so the ping, startup and the page
  never import at the same time.

## What a pull asks for

`POST http://<device>/api/doorlog/get`, basic auth with the configured user,
15 s timeout, asking for `Status: Success` punches from **7 days before** the
date to the end of that date (`attendanceLookbackDays`).

- Until v1.1.75 (2026-10-30) it asked for that day only, so a punch made
  while the till was off was never fetched again. Anything older than 7 days
  that was missed then is still missing; nothing back-fills it.
- The code assumes the device may ignore the dates and send its whole
  history newest first. Only the **first 2000** records of the answer are
  looked at.
- Only `Success` punches are used. Failed or denied attempts are ignored.

## Which punches are imported

For each punch, in order:

1. **Device user linked to a salesperson?** The punch's device user ID must
   equal a salesperson's `attendance_user_id`. If not, it's not imported (see
   "Unlinked punches" below).
2. **Deleted or replaced by an approved request before?** The device keeps
   every punch forever and sends it again on every pull. A punch whose device
   event ID and time are in `attendance_event_audit` was taken away by an
   approved change and is skipped. Matched by event ID and time, not device IP,
   because the IP can change.
3. **Duplicate?** Two punches by the same salesperson on the same day closer
   together than the **duplicate interval** (Settings, seconds; 0 = off) are
   one punch. Only the first is kept.
4. Otherwise it's inserted into `salesperson_attendance_events` (`INSERT OR
   IGNORE`, so re-pulling is harmless) and queued for kashi.

Surprises:

- **The duplicate interval also deletes existing rows.** Every pull first runs
  `collapseAttendanceDuplicates` over the whole table, so raising the interval
  removes punches that were already imported, not just future ones.
- Every pull also deletes any live device punch that matches an audit copy
  (cleanup for an older build that re-imported removed punches).
- **IN/OUT comes from the device's `Type` field**, not from the order of the
  punches. The till never works out "first punch = in".
- Times are read as the branch's local time (see `docs/dev/data-and-config.md`,
  "Time zones").

## Unlinked punches

A punch by a device user linked to no salesperson (a visitor, or an employee
enrolled twice or under a new number) is **not attendance**, but since
v1.1.75 kashi is told about it: it's sent once with status `Skipped: unlinked`,
and `attendance_skipped_sent` (migration 000134) remembers which were sent,
since the device returns them on every pull. kashi keeps them out of worked
hours and lists them so someone can link the user. Linking the user later
doesn't turn the old skipped punches into attendance by itself.

Only sent when sync is configured.

## Fixing punches: requests and approval

A cashier can't edit a punch. They file a request (`SubmitAttendanceComplaint`):

| Action | Meaning |
|---|---|
| `add` | A punch the device never recorded (forgot to clock, reader missed). |
| `replace` | Change an existing punch's time and/or IN/OUT. |
| `delete` | Remove an existing punch. |

- The request is stored as `pending` and sent to kashi (`acr-<id>-complaint`).
- It's decided either on the till with the **manager password**
  (`ApproveAttendanceComplaint`), or in kashi, which sends a
  `review_attendance_request` command. Both run the same code
  (`reviewAttendanceComplaint`).
- Only a `pending` request can be decided; a second decision is refused.
- Approving `replace`/`delete` copies the original punch to
  `attendance_event_audit` and deletes it; `replace`/`add` insert a new punch
  with `device_id = 'manual'`, `event_id = change-request-<id>`, status
  `Approved`. The new punch goes to kashi like any punch, labelled with who
  approved it.
- `AddAttendanceWithManagerApproval` files an `add` and approves it in one
  step, so it leaves the same trail.
- Manager approval on the till can be turned off per branch
  (`attendance_manager_approval`, on by default, kashi can push it). When
  off, requests wait for kashi.
- `CreateManualAttendanceEvent` (the legacy kashi-admin tool) records a punch
  with `device_id = 'admin-manual'` and no request.

So `device_id` tells you where a punch came from: the device IP, `manual`
(approved request) or `admin-manual`.

## The manager password

There is one manager password per till, stored as
`attendance_config.approver_password_hash`. Despite where it lives, it's the
manager password for **everything**: attendance approvals, opening the
Expenses and Loans pages, the Conversion page. An admin login skips it. It
never leaves the till (kashi has no column for it).

Hashing is plain unsalted SHA-256 (`hashPassword` in `app.go`), the same as
other till passwords.

## Other settings

- `attendance_cashier_history`: lets cashiers see attendance history.
- The device IP, user, password and enabled flag live in `attendance_config`.
  The password never leaves the till; the rest is in the settings report to
  kashi.
- The "checked in" popup only shows for punches less than 2 minutes old, at
  most 5 at a time, so a catch-up import doesn't flood the screen.
