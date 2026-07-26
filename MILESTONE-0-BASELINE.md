# Milestone 0 baseline

Captured before the persistence/migration changes on 2026-07-24.

## Repository state

The worktree was already dirty. Existing tracked and untracked changes were preserved; no reset or checkout was performed.

## Database observations

- `db/kashi.db` is a SQLite schema/seed database with no user rows in `roles` or `settings`.
- Its schema already contains `settings.tax_rate` and the corrected `invoice_indexes` primary key `(cashbox_id, year, type)`.
- `PRAGMA user_version` is `0`; there was no application-managed migration metadata table.
- `app.go` copied the embedded database over the runtime database on every startup.
- `app.go` also ran unchecked `ALTER TABLE` and invoice-index table-replacement statements at startup.

## Confirmed baseline defects

- Runtime settings and theme changes could be lost on restart because the database was overwritten.
- Startup SQL errors were ignored, so schema failures were not diagnosable.
- Frontend contains multiple empty `catch {}` handlers.
- Currency, checkout, tax, quantity, and printing behavior remain scheduled stabilization work after the migration gate.

## Reproduction/check commands

```sh
git status --short
sqlite3 db/kashi.db "PRAGMA user_version; PRAGMA table_info(settings); SELECT sql FROM sqlite_master WHERE name='invoice_indexes';"
go test ./...
go vet ./...
cd frontend && pnpm run type-check && pnpm run build
```

