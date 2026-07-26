# Kashi POS Stabilization and Financial Correctness Plan

## Summary

Stabilize the existing app before adding advanced features. The first confirmed root cause
is that app.go overwrites the embedded database on every startup, which explains why
settings and theme changes appear not to persist. Other confirmed issues include:

- Settings/theme errors are silently discarded in the frontend.
- Currency conversion currently treats every currency as USD.
- LBP uses a hardcoded ×1000 rule.
- Checkout records only the first payment account.
- Frontend tax is not included in backend invoice totals.
- Floating-point values are converted to integer cents inconsistently.
- Invoice quantities can be truncated.
- Printing errors are reported as checkout failures after the invoice has already been
  saved.

- Return invoices and transfers have schemas/listing code but incomplete workflows.

The agreed direction is: USD cents as the accounting currency, integer money values, full
split-tender payments, USD base currency, whole-item quantities, tax added on top, durable
OS app-data storage, and stabilization before returns/exchanges.

## Implementation Changes

### 1. Establish a safe baseline

- Preserve the existing dirty worktree and document current behavior before changing it.
- Add a reproducible development database/test database setup.
- Replace silent catch {} blocks with structured frontend error handling and visible error
  states.

- Add a backend error boundary/logging convention so database and transaction failures are
  diagnosable.

- Regenerate sqlc output only from reviewed schema/query changes.

### 2. Fix database persistence and migrations

- Stop copying embeddedDB over the runtime database on every launch.
- Store the writable database in the platform’s per-user application-data directory.
- Copy the embedded database only when no database exists.
- Add a database migration/version mechanism instead of ad hoc startup SQL such as ALTER
  TABLE.

- Ensure settings has exactly one durable row and that startup creates it if missing.
- Correct the invoice-index schema/query conflict so (cashbox, year, type) is consistently
  the key.

- Add startup validation that confirms required tables, columns, indexes, and seed data
  exist.

- Add backup/export support before destructive schema migrations.

### 3. Introduce a canonical money and currency model

- Store product prices, invoice totals, discounts, taxes, loyalty values, and USD
  accounting balances as integer USD cents.

- Store LBP tender as integer LBP units.
- Keep the cashier keypad shorthand for LBP: entering 90 means 90,000 LBP; normalize it
  immediately to 90,000 for storage and calculations.

- Replace floating-point conversion with deterministic integer/rational conversion.
- Define and centralize:
    - USD cents conversion.
    - LBP exchange-rate interpretation.
    - tax calculation.
    - discount calculation.
    - half-up rounding.
    - LBP rounding to the nearest 10,000 when producing customer-facing LBP amounts.

- Make currency/rate configuration explicit rather than relying on the current
  value_in_default_currency interpretation.

- Ensure every displayed amount includes its currency and uses the same formatter.

### 4. Refactor checkout around a backend-owned calculation

Create a single checkout calculation/domain layer used by both UI validation and backend
persistence.

The backend must receive a typed checkout request containing:

- line items with product code, integer quantity, unit price/discount source;
- tax configuration;
- client;
- one or more payment tenders;
- cashbox account IDs and currencies;
- optional cashier/session metadata.

The backend must then:

- reload authoritative product prices and discounts from the database;
- validate quantities and stock;
- calculate subtotal, discount, tax, and grand total in integer cents;
- convert each tender using the stored rate;
- apply deterministic rounding;
- validate exact payment or approved change;
- create the invoice, invoice lines, tender records, stock deductions, shift balances, and
  loyalty update in one transaction;

- reject invalid or stale client-supplied totals.

Split tender must persist every payment separately. Change must be represented explicitly
rather than being silently attached to the first account.

### 5. Repair settings and theme workflows

- Replace positional settings APIs with typed request/response objects where practical.
- Make GetAppSettings return the persisted theme as well as all other settings.
- Make settings loading show errors instead of defaulting silently.
- Make save operations await UpdateSettings, refresh the stored settings, and report the
  actual backend error.

- Make ToggleTheme.vue await SetTheme, handle failure, and revert the visual theme if
  persistence fails.

- Ensure theme loading occurs after database initialization and survives application
  restart.

- Add validation for printer dimensions, tax range, printer selection, and serial-port
  values.

### 6. Simplify and strengthen the POS frontend

- Move invoice state out of component-instance exposure and into a dedicated Pinia/domain
  store.

- Remove duplicated calculation logic from home.vue, InvoiceItems.vue, InvoiceSummary.vue,
  and Calculator.vue.

- Use one shared calculation result containing:
    - line subtotal;
    - line discounts;
    - tax;
    - grand total;
    - paid amount by currency;
    - converted paid amount;
    - remaining amount;
    - change amount;
    - checkout eligibility.

- Make all payment methods show their real currency symbols and rates.
- Remove hardcoded USD/LBP assumptions from the calculator.
- Prevent checkout unless the backend-authoritative payment request is valid.
- Preserve invoice state during failed printing and expose a reprint action.

### 7. Repair invoice persistence and auditability

- Add invoice detail retrieval including lines, currencies, discounts, tax, tenders,
  client, shift, and print status.

- Correct invoice list labels and formatting; discountedTotal must not be presented as
  “Discount”.

- Add explicit invoice status/type fields where needed.
- Record tax rate and monetary inputs used at checkout so historical invoices do not
  change when settings/rates change.

- Ensure stock failure, invoice failure, balance failure, and loyalty failure roll back
  atomically.

- Keep a committed sale when printing fails, mark printing as failed, and support
  reprinting later.

- Add idempotency protection to prevent duplicate checkout submissions.

### 8. Build returns before exchanges

Implement returns as the first advanced feature:

- Search a completed sales invoice.
- Display original line items, prices, quantities, discounts, payments, and returnable
  quantities.

- Select returned quantities.
- Create a return invoice linked to the original sale.
- Restock inventory.
- Reverse the appropriate cashbox/shift balances.
- Reverse or adjust loyalty points according to a defined policy.
- Prevent returning more than the original quantity.
- Preserve a complete audit trail.

Direct exchanges can be added afterward as an atomic combination of return plus
replacement sale.

## Public Interfaces and Schema Additions

Expected interface changes:

- Typed settings DTOs instead of long positional argument lists.
- Typed checkout request/result DTOs.
- Payment tender records associated with invoices.
- Invoice detail and reprint APIs.
- Return-invoice creation and returnable-quantity APIs.
- Explicit currency/rate APIs.
- Inventory mutation APIs for validated stock adjustments.
- Database migration metadata and backup/export APIs.

The exact table names may change during implementation, but the persisted model must
support invoice lines, tenders, tax/rate snapshots, returns, print status, and audit
relationships.

## Test Plan

### Backend unit tests

- Money addition, subtraction, discount, tax, and rounding.
- USD-cent and LBP conversion.
- LBP keypad shorthand normalization.
- Rounding to nearest 10,000 LBP.
- Mixed-currency split tender.
- Exact payment, underpayment, and change.
- Whole-unit quantity validation.
- Stock validation and rollback.
- Invoice index generation for sales and returns.
- Loyalty point calculation.
- Tax and rate snapshot behavior.

### Backend integration tests

Using temporary SQLite databases:

- Settings save → restart simulation → settings load.
- Theme save → restart simulation → theme load.
- First-run database creation and seeding.
- Checkout commits all related records atomically.
- Checkout failure leaves no partial invoice, stock, balance, or loyalty changes.
- Multiple payment accounts are persisted correctly.
- Printing failure does not remove a committed invoice.
- Return updates inventory and financial balances correctly.
- Migration from the current database schema succeeds.

### Frontend tests

- Settings form loads and saves values.
- Failed settings save displays an error.
- Theme toggle persists and reverts on failure.
- Invoice calculations match backend calculation fixtures.
- Quantity and discount changes update totals correctly.
- Currency/tender display is correct.
- Checkout is disabled for underpayment and invalid input.
- Duplicate checkout submission is prevented.
- Failed printing exposes a reprint path.

### Acceptance criteria

The core release is complete when:

- Settings and theme persist across application restarts.
- No financial calculation depends on JavaScript or Go floating-point arithmetic.
- The backend is the sole authority for invoice totals.
- Mixed USD/LBP split payments reconcile exactly according to the defined rounding rules.
- Every committed invoice is fully auditable and never partially persisted.
- Inventory, shifts, cashboxes, loyalty, and invoices remain consistent after success or
  failure.

- Automated tests cover the critical money, persistence, checkout, migration, and return
  paths.

- A return can be completed safely before direct exchanges are introduced.

## Assumptions and Defaults

- Accounting/base currency: USD.
- Stored USD amounts: integer cents.
- Stored LBP amounts: integer LBP units.
- LBP keypad: 90 represents 90,000 LBP; stored value is 90,000.
- LBP customer-facing conversion rounds to the nearest 10,000 LBP.
- General conversion uses deterministic half-up rounding.
- Tax is added on top of prices.
- Product quantities are whole units.
- Multiple payment tenders are supported and individually persisted.
- Printing failure keeps the sale and enables reprinting.
- Database location is the OS application-data directory.
- Returns are implemented before direct exchanges.
