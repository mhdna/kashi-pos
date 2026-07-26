• # Revised Kashi POS Recovery and Development Plan

  The app should move forward through controlled milestones. Each milestone must pass its acceptance gate before the next
  begins. Financial features should not be built on top of unreliable persistence or duplicated calculation logic.

  ## Core invariants

  These rules apply throughout the project:

  - USD is the accounting currency.
  - USD values are stored as integer cents.
  - LBP values are stored as integer pounds.
  - Product quantities are whole numbers.
  - JavaScript and Go floating-point values are never used for persisted money.
  - The backend is authoritative for prices, discounts, taxes, totals, payments, stock, and loyalty.
  - Every financial mutation is transactional and auditable.
  - Existing runtime databases are never overwritten.
  - Printing occurs after invoice commitment; printing failure never deletes a sale.
  - Exchange rates, tax rates, prices, and discounts used by an invoice are stored as snapshots.

  ———

  ## Milestone 0: Preserve and measure the current state

  Before restructuring anything:

  - Preserve all existing uncommitted work.
  - Back up db/kashi.db.
  - Record the current schema and representative settings, products, shifts, invoices, and balances.
  - Run and record:
      - Go tests and vet;
      - frontend type-check;
      - frontend build;
      - sqlc validation;
      - Wails build.

  - Create a short defect inventory based on confirmed behavior, not assumptions.
  - Classify embedded database content as schema/seed data rather than runtime data.

  Acceptance gate:

  - The current database is recoverable.
  - Existing changes are not accidentally replaced.
  - We have a repeatable baseline showing which checks pass or fail.

  ———

  ## Milestone 1: Database persistence and migrations

  This comes first because the app currently overwrites the embedded database during startup.

  ### Runtime database

  - Resolve a platform-specific application-data directory.
  - Store the writable database there.
  - Copy the embedded seed database only when no runtime database exists.
  - Never overwrite an existing runtime database.
  - Log the resolved database path without exposing sensitive data.

  ### Migration system

  - Introduce a schema_migrations table.
  - Apply numbered migrations in a transaction and in order.
  - Remove startup ALTER TABLE, table-copy, and index-repair statements.
  - Make migrations safe for both:
      - a fresh installation;
      - the existing db/kashi.db schema.

  - Back up the database before applying an upgrade migration.
  - Abort startup with a visible error if migration fails.
  - Do not continue with a partially initialized store.

  ### Database integrity

  - Enable foreign keys.
  - Keep WAL mode if supported.
  - Add required uniqueness, check, and foreign-key constraints.
  - Correct the invoice-index key to (cashbox_id, year, type).
  - Ensure invoice-index queries use the same conflict target.

  Acceptance gate:

  - First launch creates a functional database.
  - Subsequent launches preserve changed data.
  - Existing databases migrate without losing records.
  - Failed migrations leave a recoverable database.
  - Restart-level integration tests pass.

  ———

  ## Milestone 2: Settings, theme, and application configuration

  ### Backend API

  Replace positional settings arguments with typed structures:

  AppSettings
  - branchName
  - apiKey
  - printerSize
  - receiptPrinter
  - screenPort
  - receiptWidthMm
  - receiptHeightMm
  - taxRateBasisPoints
  - theme
  - lbpPerUsd

  Use integer basis points for tax:

  - 1100 means 11%.
  - Valid range: 0–10000.

  Add:

  - GetAppSettings()
  - UpdateAppSettings(settings)
  - SetTheme(theme) only if a dedicated theme endpoint remains useful.

  ### Database behavior

  - Guarantee exactly one settings row using a fixed primary key.
  - Seed it only when missing.
  - Validate themes against supported values.
  - Validate dimensions, tax, exchange rate, and required fields.
  - Return the saved settings after an update.

  ### Frontend behavior

  - Show loading, success, and failure states.
  - Remove silent catch {} blocks.
  - Display useful backend error messages.
  - Reload returned settings after saving.
  - Await theme persistence.
  - If theme persistence fails, restore the previous visible theme.
  - Keep unsaved form edits if saving fails.

  Acceptance gate:

  - Branch name, printing settings, tax, exchange rate, and theme survive restart.
  - Invalid settings cannot be saved.
  - Database and frontend tests cover successful and failed loads/saves.

  ———

  ## Milestone 3: Canonical money and calculation engine

  Build and test the financial engine separately from checkout and Vue components.

  ### Monetary types

  Use domain-specific integer types:

  USDCents: int64
  LBPAmount: int64
  Quantity: int64
  BasisPoints: int64
  ExchangeRate: integer LBP per USD

  Every multiplication must use overflow-safe validation.

  ### Invoice calculation order

  For each line:

  1. Load the authoritative product price.
  2. Multiply unit price by whole-number quantity.
  3. Apply the line discount using integer rounding.
  4. Produce the discounted line total.

  For the invoice:

  1. Sum gross line totals into subtotal.
  2. Sum line discounts.
  3. Sum discounted line totals.
  4. Calculate tax once on the discounted invoice total.
  5. Add tax to produce the USD-cent grand total.

  Store:

  - subtotal;
  - discount total;
  - taxable total;
  - tax rate;
  - tax amount;
  - grand total.

  ### LBP conversion

  Given lbpPerUsd, for example 90,000:

  - Exact LBP equivalent:
    USD cents × lbpPerUsd ÷ 100

  - Customer-payable LBP amount:
    round the exact equivalent to the nearest 10,000 LBP.

  - Values ending in 5,000 round upward.
  - Keypad shorthand:
    entering 90 under LBP means 90,000 LBP.

  - Once entered, the normalized value is stored and displayed as 90,000.

  For $17.75 at 90,000 LBP/USD:

  - Accounting total remains 1,775 USD cents.
  - Exact equivalent is 1,597,500 LBP.
  - Payable rounded amount is 1,600,000 LBP.
  - The rounding difference is recorded explicitly; it must not alter product revenue silently.

  ### Calculation result

  The calculation service returns:

  - authoritative lines;
  - subtotal;
  - discount;
  - taxable amount;
  - tax;
  - grand total;
  - currency quote;
  - rounded LBP payable amount;
  - rounding adjustment.

  Acceptance gate:

  - Table-driven Go tests cover prices, discounts, tax, conversion, rounding, overflow, and boundary cases.
  - The same fixtures are used in frontend tests.
  - No persisted money crosses an API boundary as float64 or JavaScript decimal money.

  ———

  ## Milestone 4: Transactional checkout and split tender

  ### Checkout request

  Use a typed request containing:

  - product code and quantity only for normal sales;
  - client ID;
  - inventory ID;
  - shift ID or current authenticated shift context;
  - payment tenders;
  - an idempotency key.

  The frontend must not submit authoritative prices or totals.

  Each tender contains:

  - cashbox account ID;
  - currency code;
  - normalized integer amount.

  ### Backend checkout flow

  Inside one database transaction:

  1. Validate the idempotency key.
  2. Load the open shift.
  3. Load products, stock, settings, currencies, and client.
  4. Recalculate the complete invoice.
  5. Validate positive whole-number quantities.
  6. Validate available inventory.
  7. Validate every payment account and its currency.
  8. Convert and reconcile all tenders.
  9. Reject underpayment.
  10. Calculate change and rounding adjustment.
  11. Allocate each tender to its actual cashbox account.
  12. Generate the invoice index and code.
  13. Save invoice and line snapshots.
  14. Save every tender.
  15. Deduct stock.
  16. Update shift cashbox balances.
  17. Update loyalty points.
  18. Commit and return the complete result.

  ### Schema additions

  Add or revise:

  - invoice tax and discount totals;
  - tax-rate snapshot;
  - exchange-rate snapshot;
  - invoice status and type;
  - invoice payment/tender records;
  - rounding adjustment;
  - printing status;
  - idempotency key;
  - cashier/user reference where available.

  ### Printing

  After commit:

  - Generate the receipt from the saved invoice, not frontend data.
  - Attempt printing.
  - Return:
      - invoice code;
      - committed status;
      - print status;
      - print error when applicable.

  - Add reprint from invoice history.
  - Never tell the cashier checkout failed when only printing failed.

  Acceptance gate:

  - Split USD/LBP/card payments persist under the correct accounts.
  - Backend totals cannot be changed by tampering with frontend payloads.
  - Failures roll back invoice, stock, cashbox, and loyalty mutations.
  - Duplicate submission returns the original result instead of creating another invoice.
  - Printing failure leaves one valid committed invoice.

  ———

  ## Milestone 5: POS frontend architecture

  Move POS state into a dedicated Pinia store or composable domain layer.

  The store owns:

  - invoice lines;
  - selected client;
  - active shift;
  - calculation result;
  - payment entries;
  - checkout state;
  - hold/restore state;
  - errors.

  Components become focused views:

  - InvoiceItems edits quantities and renders calculated lines.
  - InvoiceSummary renders the calculation result.
  - Calculator captures normalized payment tenders.
  - ActionBar triggers store actions.
  - home.vue composes the workflow but performs no financial arithmetic.

  Additional behavior:

  - Disable checkout during submission.
  - Prevent duplicate clicks.
  - Preserve invoice state after backend validation failure.
  - Clear invoice state only after a committed checkout.
  - Use structured errors instead of generic snackbars.
  - Show invoice committed/printing failed as a recoverable state.
  - Use consistent currency formatting across all screens.

  Acceptance gate:

  - No financial formulas remain in Vue components.
  - UI tests cover scanning, quantity changes, split payments, checkout, errors, and state reset.
  - Backend calculation results are rendered exactly.

  ———

  ## Milestone 6: Complete existing operational screens

  After checkout is reliable:

  - Invoice list and full invoice detail.
  - Reprint workflow.
  - Inventory quantities and adjustment history.
  - Transfer creation, receiving, cancellation, and audit trail.
  - Shift opening/closing and reconciliation by payment account.
  - Client history and loyalty ledger.
  - Product and currency management.
  - Role and permission enforcement on backend methods, not only router guards.

  Each feature must include:

  - a transactional backend operation;
  - typed Wails interfaces;
  - visible frontend errors;
  - integration tests;
  - an audit record for financial or inventory changes.

  ———

  ## Milestone 7: Returns, then exchanges

  ### Returns

  - Search an original completed sale.
  - Load remaining returnable quantities.
  - Select products and quantities.
  - Validate that quantities were sold and not previously returned.
  - Create a linked return invoice.
  - Restock inventory.
  - Reverse revenue, tax, cashbox balances, and loyalty through ledger entries.
  - Refund to original tenders by default.
  - Require elevated permission to override the refund method.
  - Preserve original price, discount, tax, and exchange-rate snapshots.
  - Never modify or delete the original invoice.

  ### Exchanges

  After returns are stable:

  - Model exchange as one atomic operation:
      - return selected original items;
      - create a replacement sale;
      - settle the net difference.

  - If replacement is more expensive, collect additional tenders.
  - If replacement is cheaper, issue a refund.
  - Link the return, replacement sale, and exchange operation.

  Acceptance gate:

  - Partial and repeated returns cannot exceed sold quantities.
  - Inventory and financial ledgers remain balanced.
  - Failed exchanges produce neither a return nor replacement sale.
  - Every operation is traceable from the original invoice.

  ———

  ## Testing and quality gates

  Every milestone must pass:

  go test ./...
  go vet ./...
  frontend type-check
  frontend unit tests
  frontend production build
  sqlc generation/verification
  Wails build

  Critical financial tests should include:

  - zero, minimum, and maximum values;
  - discount and tax rounding boundaries;
  - $17.75 → 1,597,500 → 1,600,000 LBP;
  - values just below, at, and above 5,000 LBP;
  - exact payment, underpayment, overpayment, and split payment;
  - stale exchange rates;
  - insufficient stock;
  - duplicate checkout;
  - database failure at every transaction stage;
  - printing failure after commit;
  - restart persistence;
  - migration from the current database.

  ## Immediate execution order

  The first working batch should contain only:

  1. baseline and database backup;
  2. runtime database location;
  3. first-run-only seed copying;
  4. migration runner;
  5. settings-row correction;
  6. settings/theme APIs and frontend error handling;
  7. restart persistence tests.
