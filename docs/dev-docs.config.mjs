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
  chips: [
    ['sync-with-kashi', 'Sync', 'mdiSync'],
    ['money-and-selling', 'Money', 'mdiCashRegister'],
    ['windows-till', 'Windows till', 'mdiMonitor'],
    ['releases-and-builds', 'Releases', 'mdiRocketLaunch'],
  ],
  diagrams,
}
