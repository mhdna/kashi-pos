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
| `%LOCALAPPDATA%\kashi-pos\.rc\store` | Saved copies of kashi's record photos, encrypted (see below). Hidden folder; safe to delete. |
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

## Record photo cache (`photo_cache.go`, `record_photos.go`)

Expense, loan and transfer photos live in kashi (phones upload them there).
The till keeps every one it has shown so the next showing is instant, offline
too.

- **One SQLite file, no image files.** `os.UserCacheDir()/kashi-pos/.rc/store`
  (`%LOCALAPPDATA%` on Windows, never the roaming profile), the folder hidden.
  Each photo is a row, **AES-256-GCM encrypted**; the key is a secret compiled
  into the app plus a random per-file salt, and each row is bound to its own
  key, so blobs can't be swapped. Keeps people out of the photos with a file
  manager or SQLite browser; it can't stop someone who takes the exe apart.
- **Saved copy first, kashi checks in the background.** A copy older than 30 s
  (`photoRecheckAfter`) is still shown, then re-asked with its ETag (kashi
  answers 304 when unchanged); at most 4 checks at once, one per copy. "No
  photo" (404) is saved too.
- **Changes reach the screen** as the `record_photos_changed` event with the
  record's ref (expense/loan clientRef, sent transfer clientRef,
  `transfer-<id>` received); pages bump that row's photo version
  (`lib/recordPhotos.ts`). The ref naming is pinned by
  `record_photo_refs.json`, written by `TestRecordPhotoRefsGolden`.
- **New photos drop the record's copies**: on `expense_images_uploaded` /
  `loan_images_uploaded` and after *Add from this computer*.
- **Photo id lists are live.** `ListExpensePhotoIDs` / `ListLoanPhotoIDs` ask
  kashi every time (the upload dialog polls them to see a photo land) and use
  the saved list only when kashi can't be reached.
- Copies are keyed by kashi URL + branch code + request, so another branch or
  server never shows the old one's. Capped at 512 MB, least recently shown
  dropped first.
- Can't be opened: photos just come from kashi as before.

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
- Queries are generated with sqlc (`db/query` → `db/sqlc`), v1.31.1.
  See `releases-and-builds.md` for the regeneration gotchas.

## Product search shows one result per code

A code is shared by its colour and size variants (one `products` row each,
migration 27). The search still pages **rows** (`ListProductsCursor`), but the
frontend folds them into one result per code (`lib/productGroups.ts`; a product
with no code is its own result). Tapping a multi-variant result opens
`VariantPicker.vue`.

| Piece | What it does |
|---|---|
| `groupProducts` | Groups loaded rows by exact code, in first-seen order; a code's later rows join it when their page arrives. |
| `CountProductGroupsForSearch` | The "N products" label: `COUNT(DISTINCT code)` over the same matches as `CountProductsForSearch`. |
| `ListProductVariants(code)` | Every active variant of the code (any case), for the picker, since the list may hold only some yet. |
| `VariantPicker` | Colour chips with swatches and size chips; impossible pairs greyed out; a single option is picked for the cashier; identical colour+size variants are listed. |
| `lib/colorSwatch.ts` | Colour name to CSS: `A/B` names are split into slices, catalogue short forms (`Pur`, `Tra`) are read, unknown names get a dashed empty dot. kashi's colour hex values aren't synced to the till. |

The scan field's code lookup (`FindProductsForScan`) still uses its own list
picker in `home.vue`.

## Product search index (`products_search.go`, migration 135)

The sale screen's product search (`ListProductsCursor`, `CountProductsForSearch`)
no longer scans every product with `LIKE '%x%'`. `products_search` is an FTS5
**trigram** table (rowid = `products.id`; `all_text` = name + code + barcode,
`code_text` = code + barcode for the code-only switch), kept current by
triggers on `products`, so every writer (sync, CSV import, edits, an older
build) stays correct without Go changes.

- Terms of **3+ characters** use the index; **1-2 characters** fall back to the
  old LIKE queries (a trigram index can't answer them). Same rows either way.
- The queries are generated by sqlc (`ListProducts*Indexed`,
  `CountProducts*Indexed` in `db/query/product.sql`). `ListProductsPage` (the
  products page, the admin server) still uses LIKE.
- On a 13.9k-product database it takes selective searches (codes, barcodes, rare
  words) from ~5 ms to ~0.2 ms. A very common word (2,400 matches) is no faster:
  the first page is slower (~6 ms vs 1 ms) and the count a bit faster.
- Adding the migration to a big catalogue backfills the index (0.8 s for 13.9k
  products, once).

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
| `variant_pick_required` | off | A scanned barcode whose item has colour/size variants (same code) opens the variant picker. `products.variant_pick` (`always`/`never`, synced from kashi) overrides it per item. |
| `invoice_ratings` | **on** | Invoices list shows each sale's WhatsApp poll answer (😍 🙂 😕), fetched live from kashi's `/branch/invoice_ratings` for the codes on screen (`invoice_ratings.go`). kashi pushes it as `invoice_ratings_enabled`. |
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
