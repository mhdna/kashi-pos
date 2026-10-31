// Config for docs/build-dev-docs.mjs in this repo. Run with `make dev-docs`.
// The pages are the markdown files in docs/dev/ (they stay the source of truth,
// readable on their own); diagrams are injected at the top of a page here.
import diagrams from './dev-docs-diagrams.mjs'

export default {
  title: 'kashi-pos developer docs',
  out: 'dev-docs.html',
  mdiFrom: ['../frontend/package.json'],
  favicon: { icon: 'mdiCashRegister', bg: '#e65100' },
  intro:
    'For developers working on the till app. Not a user guide: this is how the code behaves, including the parts that are not obvious from the screens.\n\nWhen you change a behaviour described here, update its page in the same commit, then run `make dev-docs`.',
  source: {
    pages: [
      { file: 'dev/README.md', id: 'overview', title: 'Overview', icon: 'mdiCompass', group: 'Start here' },
      { file: 'dev/money-and-selling.md', title: 'Money and selling', icon: 'mdiCashRegister', group: 'Selling' },
      { file: 'dev/shifts-and-logins.md', title: 'Shifts and logins', icon: 'mdiAccountClock', group: 'Selling' },
      { file: 'dev/sync.md', title: 'Sync with kashi', icon: 'mdiSync', group: 'Sync and devices' },
      { file: 'dev/attendance.md', title: 'Attendance', icon: 'mdiFingerprint', group: 'Sync and devices' },
      { file: 'dev/windows-till.md', title: 'The Windows till', icon: 'mdiMonitor', group: 'Sync and devices' },
      { file: 'dev/data-and-config.md', title: 'Data and config', icon: 'mdiDatabaseCog', group: 'Platform' },
      { file: 'dev/ui-components.md', title: 'UI components', icon: 'mdiPalette', group: 'Platform' },
      { file: 'dev/performance.md', title: 'Performance', icon: 'mdiSpeedometer', group: 'Platform' },
      { file: 'dev/releases-and-builds.md', title: 'Releases and builds', icon: 'mdiRocketLaunch', group: 'Shipping' },
    ],
  },
  inject: {
    overview: ['architecture'],
    'money-and-selling': ['pricing'],
    'shifts-and-logins': ['shift'],
    'sync-with-kashi': ['syncOverview', 'outbox'],
    attendance: ['attendance'],
    'releases-and-builds': ['update'],
  },
  glance: {
    overview: [
      '**There is no log on a till** (Windows build): add a log file before debugging by logs.',
      '**Checkout does not take stock off**; stock is kashi\'s, from synced sales.',
      '**A product has no price of its own**: prices come from kashi\'s price lists only.',
      '**All money is integer USD cents**, whatever was paid with (expenses/loans: own currency x100).',
      '**Migrations are additive**; an older build must run on a newer database (that is what makes rollback safe).',
    ],
    'money-and-selling': [
      'Integer **USD cents** everywhere; rates are `units_per_usd_micros`. Never compare currency codes to literals.',
      'Not in an active price list = **no price**; checkout refuses a zero price.',
      '**`priceInvoiceTotals`** is the one pricing path (checkout and live quote), so shown = charged.',
      'Returned lines have **negative** quantity and total; sum as stored.',
      '`On Account` and `On Coupon` are found by exact name: **do not rename**.',
    ],
    'shifts-and-logins': [
      'A **sale needs an open shift**; expenses and loans do not.',
      '**PIN lockout survives restarts** and doubles: 30 s up to 30 min.',
      'The "manager password" is the **attendance approver\'s** and also unlocks Expenses, Loans, Conversion.',
      'Stale shifts auto-close only if switched on; **empty ones are deleted**, not closed.',
    ],
    'sync-with-kashi': [
      'kashi owns catalogue and prices; the till owns invoices, shifts, expenses, loans, attendance.',
      'Match on **`uid`**, never integer ids.',
      'Every synced write queues an outbox row **in the same transaction**.',
      'A **network failure stops the whole pass** and waits; a 5xx/408/429 backs off just that row.',
      '**Never delete what history points at**; deactivate. Price lists, payment methods, currencies arrive **whole**.',
    ],
    attendance: [
      '**No background timer**: pulls on the device ping, at start, and every 60 s while the page is open.',
      'Each pull looks back **7 days**; only the first 2000 records are read.',
      '**Raising the duplicate interval deletes** punches already imported.',
      'IN/OUT comes from the **device**, not punch order.',
    ],
    'windows-till': [
      '**Kiosk mode closes Explorer**; Close App brings it back.',
      '**Cash machine mode closes other programs every 30 s**; its setup removes apps for good.',
      'The scanner is recognised by **typing speed** (6+ keys under 50 ms), not USB id.',
      'Local ports: **8788** shift photo, **8789** attendance ping.',
    ],
    'data-and-config': [
      'A **missing feature flag means its default**, and defaults differ; read default-on ones with `flagEnabledDefaultOn`.',
      'Migrations are tracked **by number** and stay additive.',
      'Times are read in the **branch\'s time zone**, not the PC\'s.',
      'Some settings **never leave the till**: API key, theme, register number, device and manager passwords.',
    ],
    'ui-components': [
      'The own component kit **replaced Vuetify**.',
      'Kit CSS is in `@layer`s: a layered `!important` beats an unlayered one.',
    ],
    performance: [
      'Old tills are the target: read **the rules** before adding UI.',
      'Animations are off by default but **spinners always spin**.',
      'The empty-cart **blur is deliberate**; do not add other blurs or heavy shadows.',
      'The cart draft saves **~0.4 s late** on purpose (`draftSaver.ts`).',
      'FTS5 search for 3+ characters, `LIKE` below; both must return the same rows.',
    ],
    'releases-and-builds': [
      '`make bump` (commit + tag), then `make publish` right after, from a clean tree.',
      'A **bad update rolls itself back**: the helper restores the old exe if the new one does not confirm.',
      'An update **never touches the database**.',
      'Go is **pinned to 1.25.5**; first Windows build after a cache wipe is very slow.',
      'Known: `TestStartupPreservesRuntimeDatabase` kills the Go test run.',
    ],
  },
  chips: [
    ['sync-with-kashi', 'Sync', 'mdiSync'],
    ['money-and-selling', 'Money', 'mdiCashRegister'],
    ['windows-till', 'Windows till', 'mdiMonitor'],
    ['releases-and-builds', 'Releases', 'mdiRocketLaunch'],
  ],
  diagrams,
}
