# Data, configuration and switches

## Where things live on a till

`%APPDATA%` below is what Go's `os.UserConfigDir()` returns (on a Mac:
`~/Library/Application Support`).

| Path | What |
|---|---|
| `%APPDATA%\kashi-pos\branches.json` | The branch registry: every branch set up on this PC, which is active, which opens by default. Read before any database is opened. |
| `%APPDATA%\kashi-pos\database-<code>` | One SQLite database per branch (no file extension). |
| `%APPDATA%\kashi-pos\devices.json` | Which input device is the scanner. |
| `%APPDATA%\kashi-pos\updates\` | Downloaded releases, `state.json` and `handoff.json` for remote updates. |
| next to `GB-Cloud.exe` | The previous version's exe, kept for rollback, and an optional `.env`. |
| `%LOCALAPPDATA%\...` | WebView2 profile (see `windows-till.md`). |
| `%TEMP%\kashicloud-splash.log` | Splash window log. |

- **There is no app log.** The Go code calls `log.Printf` all over, but output
  is never redirected, and a Windows GUI build has no console, so on a till
  those lines go nowhere. Only `wails dev` and tests show them. To see what a
  till is doing, add a log file first.
- `KASHI_DB_PATH` overrides which database opens (headless/deployment use).
- Upgrading from before the registry existed adopts old `database` /
  `database-<code>` files automatically, reading each one's branch name and
  code.
- Several branches on one PC each get their own database. Switching
  (`SwitchBranch`) reopens the database in place and reloads the window; no
  restart.
- A new database starts as a copy of the seed embedded in the exe
  (`db/kashi.db`), then migrations run on it. A database is never replaced
  by the seed after that.
- SQLite runs with WAL, foreign keys on, a 5 s busy timeout and
  `_txlock=immediate` (every transaction takes the write lock at its start).

## `.env`

`loadDotEnv` reads `.env` from the working directory and from next to the exe,
without overriding variables already set. Only a small subset of dotenv is
supported: `KEY=value`, optional quotes, comments.

The `KASHI_*` settings variables (printer, receipt size, theme, tax, rounding,
shop name, socials, receipt fonts, register name…, see `env_defaults.go`)
only **seed** a new database's settings, and fill fields set in `.env` at
startup. **Branch identity (name, code, API key) is not read from `.env`**
any more; it comes from the setup screen and the registry.

Other variables:

| Variable | Effect |
|---|---|
| `KASHI_KIOSK=false` | No kiosk mode. |
| `KASHI_DB_PATH` | Open this database. |
| `KASHI_ATTENDANCE_PUSH_LISTEN`, `KASHI_SHIFT_UPLOAD_LISTEN` | Local server addresses. |
| `KASHI_ADMIN_ENABLED`, `KASHI_ADMIN_LISTEN`, `KASHI_ADMIN_TOKEN`, `KASHI_ADMIN_RELOAD_COMMAND` | Legacy kashi-admin API. |
| `KASHI_SOCIAL_PLATFORMS` | Which social handles the receipt offers. |
| `KASHI_URL`, `KASHI_USER`, `KASHI_PASSWORD` | Used by `cmd/publish-release` on the dev Mac, not by the till. |
| `KASHI_CLOUD_UPDATE_KEY` | Release-signing key override for publishing. |
| `KASHI_RECEIPT_PREVIEW_OUT`, `KASHI_RASTER_TEST_OUTPUT`, `KASHI_SHIFT_REPORT_PREVIEW_PNG` | Test output folders. |

## Time zones (`clock.go`)

Times are stored in UTC, and days and hours are read in the **branch's** time
zone (kashi's `branches.timezone`), not the PC's. The app sets Go's
`time.Local` to it, which also covers SQLite's `localtime`. What it can't fix
is a wrong PC clock or the webview's own date formatting (it uses the PC's
zone), so those show a warning instead.

## Migrations (`migrations.go`, `db/migrations`)

- Migrations are embedded and tracked **by version number only**, not by
  content. Editing a migration that already ran somewhere does nothing there.
  That drift has happened; the `ensure*Column` functions repair databases that
  recorded a version as applied but miss its columns.
- **A newer schema counts as current:** an older build runs on a database a
  newer build migrated. That's what makes rollback safe, so **migrations must
  stay additive** (add tables and columns; don't drop, rename or change
  meaning).
- Very old databases didn't record versions; they're treated as migrations
  1–16.
- Queries are generated with sqlc (`db/query` → `db/sqlc`), pinned at v1.30.0.
  See `releases-and-builds.md` for the regeneration gotchas.

## Feature switches (`feature_flags` table)

Most behaviour switches are rows in `feature_flags`. A **missing row means the
default**, and the default differs per flag, so always check before reading
one. Use `flagEnabled` for default-off flags and `flagEnabledDefaultOn` for
default-on ones.

| Flag | Default | What |
|---|---|---|
| `attendance_cashier_history` | off | Cashiers see attendance history. |
| `attendance_manager_approval` | **on** | Manager can decide attendance requests on the till. |
| `auto_append_lbp_zeros` | off | Add trailing zeros when typing LBP. |
| `auto_close_stale_shifts` | off | Close yesterday's open shift at startup. |
| `auto_cover_negative_lbp` | off | Swap negative LBP cash into USD at close. |
| `client_required` | **on** | Every sale needs a client. |
| `close_shift_requires_sync` | off | Can't close a shift with unsynced data. |
| `count_opening_cash_on_close` | **on** | The close count includes the opening float. |
| `custom_item_discounts`, `custom_item_prices`, `per_unit_item_prices` | off | Custom pricing at the till. |
| `deal_toggle`, `discount_toggle`, `discount_toggle_default_off` | off | Salesperson can turn deals/discounts off on a line. |
| `hide_payment_method_currency` | off | Hide the currency next to payment methods. |
| `invoice_keyboard_mode` | off | Invoice keyboard mode. |
| `non_admin_month_nav_enabled` | off | Non-admins can browse months in the income chart. |
| `opening_float_enabled` | **on** | The opening cash float feature. |
| `overchange_confirmation` | **on** | Confirm large change. |
| `price_change_manual_override_mode` | off | Price change mode. |
| `price_edit_restricted` | off | Price edits only for listed codes. |
| `product_search_code_only` | off | Product search by code only. |
| `product_search_enabled` | **on** | The product search button. |
| `release_stuck_keys` | **on** | Release modifier keys left held (till-only). |
| `salesperson_per_invoice_required` | off | Every sale names a salesperson. |
| `scan_by_code` | **on** | Product code in the scan field. |
| `shift_report_signatures` | **on** | Sign-off lines on the shift receipt. |
| `shift_report_uppercase_header` | off | Capitalised shift receipt header. |
| `sounds_enabled` | off | Sounds. |
| `unknown_pictures_at_checkout` | off | Photograph unknown items before payment. |
| `visitor_counter` | off | People-visits counter. |

Enum settings that can't be a flag live in `settings` columns:
`oversell_mode`, `shift_receipt_upload_mode`, `quick_access_mode`,
`receipt_theme` / `receipt_layout`, `display_prefs`.

Most switches can be pushed per branch from kashi's Branch Settings (see
`~/Code/kashi` and the per-branch settings checklist). Adding one that kashi
can push touches both repos: the flag and its setter, `GetAppSettings`, the
settings report (`branchSettingsPayload`), the `update_settings` payload and
`flagUpdates` in `commands.go`, the Settings page, and on kashi a migration,
the branch settings API and its page.
