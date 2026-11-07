# kashi-pos developer docs

For developers working on the till app. Not a user guide: this is how the
code behaves, including the parts that aren't obvious from the screens.

| File | About |
|---|---|
| [attendance.md](attendance.md) | Akuvox punches: when they're pulled, which are imported, requests and approval. |
| [sync.md](sync.md) | kashi ↔ till: who owns what, the outbox, pulls, commands, the settings report. |
| [money-and-selling.md](money-and-selling.md) | Cents, currencies, where prices come from, deals, discounts, stock, returns. |
| [shifts-and-logins.md](shifts-and-logins.md) | Shifts and closing, the opening float, logins and the PIN lockout, salespersons, targets. |
| [ui-components.md](ui-components.md) | The own component kit that replaced Vuetify: theme vars, layers, quirks. |
| [windows-till.md](windows-till.md) | Kiosk, cash machine mode, autostart, devices and the scanner, stuck keys, local ports, the compact build. |
| [data-and-config.md](data-and-config.md) | Files on the till, `.env`, time zones, migrations, every feature switch and its default. |
| [performance.md](performance.md) | Older tills: the rules for UI and queries, WebView flags, animations, startup, the search index, the draft saver, background loops, how to measure. |
| [releases-and-builds.md](releases-and-builds.md) | `make bump` / `make publish`, how tills update and roll back, build and test gotchas. |

Bug history is in [`../FIXED_BUGS.md`](../FIXED_BUGS.md).

## Things that surprise people

1. **There is no log on a till.** `log.Printf` output goes nowhere in the
   Windows build. Add a log file before debugging a till by its logs.
2. **Checkout doesn't take stock off.** Stock is kashi's, from synced sales.
3. **A product has no price of its own.** Prices come only from kashi's price
   lists; `products.price` is frozen. Not in an active list = can't be sold.
4. **All money is integer USD cents**, whatever was paid with. Expenses and
   loans are the exception (their own currency, x100).
5. **Attendance isn't polled in the background.** Punches come in when the
   device pings the till, when the till starts, or while the Attendance page
   is open. Each pull looks back 7 days.
6. **Raising the attendance duplicate interval deletes punches already
   imported**, not just future ones.
7. **IN/OUT comes from the device**, not from punch order.
8. **The "manager password" is the attendance approver's password**, and it
   also unlocks Expenses, Loans and Conversion.
9. **The PIN lockout survives restarts** and doubles each time (30 s up to
   30 min).
10. **A missing feature flag means its default, and defaults differ** (some
    on, most off). Read default-on flags with `flagEnabledDefaultOn`.
11. **Migrations are tracked by number only and must stay additive.** An older
    build runs on a newer database; that's what makes rollback safe.
12. **A bad update rolls itself back:** if the new version doesn't confirm it
    started, the helper restores the old exe.
13. **Kiosk mode closes Explorer.** Close App brings it back; a crash needs
    Task Manager → Run new task → `explorer`.
14. **Cash machine mode closes every other program every 30 seconds** while
    kiosk mode is on, and its one-time setup removes apps for good.
15. **The scanner is recognised by typing speed** (6+ keys under 50 ms apart,
    ending in Enter/Tab), not by its USB ID.
16. **A stuck Windows key after AnyDesk is released automatically** after 3
    seconds (till-only switch, on by default).
17. **While connected, the till can't remove products or clients**; kashi
    deactivates them. Nothing anything refers to is ever hard-deleted.
18. **Price lists, payment methods and currencies arrive whole**, so something
    missing from kashi's list disappears from the till.
19. **Sync stops at the first network error and waits for the connection**,
    instead of backing off each row.
20. **Some settings never leave the till:** API key, theme, register number,
    attendance device password, manager password, stuck-keys switch.
21. **Times are read in the branch's time zone, not the PC's.**
22. **Stale shifts can be auto-closed at startup** (if switched on), and empty
    ones are deleted rather than closed.
23. **The cashier PIN is stored as plain text**; the admin password as
    unsalted SHA-256.
24. **The first Windows build after the Go module cache is cleared takes a
    long time** (SQLite-in-Go is big). Go is pinned to 1.25.5 for Wails.
25. **`TestStartupPreservesRuntimeDatabase` kills the Go test run** (known,
    since at least v1.1.75).
26. **Animations are off by default, but spinners always spin.** The Animations
    switch only governs the nav rail's red status dot and one menu fade;
    `styles/settings.scss` kills every other transition regardless. v1.1.82
    switched every till off once.
27. **A layered `!important` beats an unlayered one.** The UI kit's CSS is in
    `@layer`s, so a page's `!important` can't override a kit `!important`.
28. **The empty-cart overlay is blurred on purpose**, even on old tills
    (`backdrop-filter`, CPU-drawn because GPU rasterisation is off in
    `webview_windows.go`). Don't add other blurs or heavy shadows.
29. **The "Ready to scan" panel and the idle-built calculator are on the
    `ready-state` branch**, not on `master`.
30. **The cart draft is saved ~0.4 s late, on purpose** (`lib/draftSaver.ts`);
    clearing it is immediate. Never write it straight to `localStorage`.
31. **Product search has an FTS5 index for terms of 3+ characters** and plain
    `LIKE` below that; both must return the same rows. It is slower than `LIKE`
    for the first page of a very common word.
32. **`libphonenumber` lives only in the `ClientDialog` chunk.** Importing it from
    the sale screen's startup path puts ~140 kB back into `home`.
33. **sqlc is v1.31.1 now**, and it generates the FTS queries too. Some
    `EXISTS`/`COUNT` results are `bool`, not `int64`.
34. **`GB_CLOUD_WEBVIEW_ARGS` replaces the WebView2 flags for one till**, to
    compare GPU rasterisation on and off without a rebuild.
35. **`cp` may be aliased to `cp -i` in a shell**, and hangs a script waiting for
    "y". Use `/bin/cp` when copying a database to benchmark on.
36. **Product search groups variants by code** in the frontend; the count label
    counts codes (`CountProductGroupsForSearch`) and the picker asks
    `ListProductVariants` for the whole set (see data-and-config.md).

These docs were written on 2026-10-31 from the code at v1.1.77; items 26-35 and
[performance.md](performance.md) were added with v1.1.82. When you
change a behaviour described here, update the page in the same commit.
