# Building, releasing and remote updates

## Day-to-day commands

| Command | Does |
|---|---|
| `make dev` | `wails dev`: windowed, hot reload, no kiosk, no updates. |
| `make test` | Go tests, then frontend type-check and build. |
| `make lint` | Type-check, `go vet`, golangci-lint. |
| `make generate` | sqlc. |
| `make bump` | The next release: commit `chore: release vX` and tag it. |
| `make publish NOTES="…"` | Build the Windows exe at the latest tag, sign it, upload it to kashi, ask which branches get it. |
| `make publish-all` | Same, and every active till of that flavor is set to it. |
| `make publish-all-force` | Same, and tills install it right away, even mid-shift, after a one-minute countdown. |
| `make publish-compact…` | The same for the compact flavor. |
| `make receipt-previews` | Regenerate the receipt look images kashi's settings pages show; commit them in kashi. |

## Releasing

1. Finish the work and commit it. `make bump` refuses if anything is
   uncommitted, if nothing was committed since the last release, or if the
   code doesn't build. It writes the version into `wails.json` and the
   installer (Windows wants 4 parts: `1.1.77` → `1.1.77.0`), commits only those
   two files, and tags `v1.1.77`.
2. `make publish` builds **whatever is on disk** at the latest tag's version,
   so publish right after bumping, from a clean tree.
3. Choose which tills get it on kashi's App Updates page (or use
   `publish-all`).

`publish` needs `KASHI_URL` and `KASHI_USER`; the password is asked once and
kept in the macOS Keychain. The release-signing key is in
`~/.config/gb-cloud/` (back it up); its public half is compiled in from
`internal/updatekey`. Builds without it never install updates.

## How a till updates (`updater.go`)

- kashi tells each till which release to run. The till downloads it in the
  background (resuming partial downloads), checks size, SHA-256 and the
  release-key signature, and waits for a safe moment: the cashier taps
  "Install now", or it offers itself with a cancellable countdown once no
  shift is open (after close shift, or overnight). A **forced** update is
  offered right away with a countdown that can't be cancelled.
- Installing hands off to a helper (a temporary copy of the old exe run with
  `--apply-update`): it waits for the app to exit, swaps the exe, starts the
  new one and waits for it to call `ConfirmAppStarted`. **If the new version
  doesn't confirm, the helper puts the old exe back and starts it.** A bad
  build can't leave a shop without a till.
- The previous exe is kept next to the current one. "Roll back" in Settings
  swaps them (and again to go forward).
- **The database is never touched by an update.** Rollback only works because
  an older build accepts a newer schema, so migrations must stay additive (see
  `data-and-config.md`).
- A till only gets updates of its own flavor (standard or compact).
- A kashi without the update route (404) just means no updates; the till stays
  quiet.

## Build gotchas

- **Go toolchain is pinned to go1.25.5** in the Makefile (`GOTOOLCHAIN`):
  Wails' package loader breaks under Go 1.27. Plain `go` commands outside
  `make` may use a newer Go; that's fine for tests but not for `wails build`.
- **The first Windows build after the Go module cache is cleared is slow.** It
  downloads every module again, and `modernc.org/sqlite` + `modernc.org/libc`
  (SQLite translated to Go) are large to download and to compile. On
  2026-10-31 the cache had been wiped and one 18 MB module took ~12 minutes at
  ~24 KB/s. Later builds use the cache.
- `wails build` runs `go list -export` over the whole project first, to
  generate the frontend bindings, before the Windows compile. That's a second
  full compile for the Mac. The bindings (`frontend/wailsjs`) are committed.
- **After adding a Go method the frontend calls:** run `wails generate module`,
  then `git checkout frontend/wailsjs/runtime` (it rewrites the runtime too),
  and commit `frontend/wailsjs/go`.
- **sqlc is pinned at v1.30.0.** A local v1.31.1 rewrites every generated
  file's header, and also turns some `EXISTS`/`COUNT` results into `bool`
  (`ClientHasInvoice`, `ShiftCloseSyncedForRef`, `ListInvoices*.IsSale`),
  which breaks the build. Revert those after `make generate`.
- **Icons:** the app doesn't use the MDI webfont. Every `mdi-*` name must be
  imported from `@mdi/js` and added to `iconPaths` in
  `frontend/src/lib/icons.ts`, or it renders as text. `pnpm build` runs
  `scripts/check-icons.mjs` and fails on unregistered names.
- `frontend/src/pages/settings.vue` (and some Go files) aren't Prettier/gofmt
  clean as committed. Run formatters on what you changed only, or the diff
  fills with unrelated reformatting.

## Tests

- `go test ./...` from the repo root.
- **Known failure:** `TestStartupPreservesRuntimeDatabase` calls Wails'
  `EventsEmit` without an app context, which kills the whole test run (seen
  on v1.1.75–v1.1.77). Run the other tests by name with `-run` until it's
  fixed.
- Windows-only code (`*_windows.go`) isn't compiled on the Mac. Check it with
  `GOOS=windows GOARCH=amd64 go build -o /dev/null .` and
  `GOOS=windows go vet .`.
- Tests that need a database open a fresh SQLite file in `t.TempDir()` and run
  `applyMigrations` (see `sounds_test.go` for the pattern).
- Bug fixes get an entry in `docs/FIXED_BUGS.md` in the same commit
  (Symptom / Cause / Fix).
