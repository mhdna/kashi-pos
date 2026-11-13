import { box, arrow, svg } from './dev-docs-svg.mjs'

export default {
  architecture: svg(900, 400, [
    box(20, 20, 560, 280, 'The till (Wails: Go + Vue)', 'orange'),
    box(50, 70, 150, 70, 'Vue UI', 'plain', 'sale screen, pages'),
    box(225, 70, 150, 70, 'Go backend', 'plain', 'invoices, shifts, sync'),
    box(400, 70, 150, 70, 'SQLite', 'plain', 'local, additive migrations'),
    arrow([[200, 105], [225, 105]]), arrow([[375, 105], [400, 105]]),
    box(50, 180, 240, 90, 'local servers', 'plain', ':8788 shift receipt photo\n:8789 attendance ping'),
    box(315, 180, 235, 90, 'devices', 'plain', 'receipt printer, scanner,\ncash drawer, Akuvox door'),
    box(640, 70, 240, 110, 'central', 'blue', 'kashi server\nGo + Postgres + web app'),
    arrow([[580, 125], [640, 125]], 'sync\nWebSocket'),
    `<text class="note" x="450" y="350">A till works alone when no central URL is set (standalone mode).</text>`,
    `<text class="note" x="450" y="376">central owns products, prices and clients; the till owns invoices, shifts and cash.</text>`,
  ].join('')),

  syncOverview: svg(900, 330, [
    box(20, 110, 250, 110, 'Till', 'orange', 'invoices, exchanges, shifts,\nexpenses, loans, attendance'),
    box(630, 110, 250, 110, 'central', 'blue', 'products, variants, prices,\nclients, salespersons, currencies'),
    arrow([[270, 140], [630, 140]], 'push: outbox, kicked on every write'),
    arrow([[630, 195], [270, 195]], 'pull: updated_at cursor, every 15 s'),
    arrow([[750, 110], [750, 40], [150, 40], [150, 110]], 'WebSocket nudges: pull now, command waiting, update available', 'dash'),
    arrow([[750, 220], [750, 290], [150, 290], [150, 220]], 'commands: update_settings, print, sync_now ...', 'dash'),
  ].join('')),

  outbox: svg(900, 220, [
    box(10, 40, 190, 80, 'local write', 'orange', 'invoice, shift, expense'),
    box(235, 40, 190, 80, 'sync_outbox row', 'green', 'same transaction\nas the write'),
    box(460, 40, 190, 80, 'push loop', 'blue', 'kick after a write\n+ 5 s backstop'),
    box(685, 40, 200, 80, 'central', 'plain'),
    arrow([[200, 80], [235, 80]]), arrow([[425, 80], [460, 80]]), arrow([[650, 80], [685, 80]]),
    arrow([[555, 120], [555, 175], [300, 175], [300, 120]], 'no connection: stop the pass, wait 5 s, 10 s ... 1 min', 'dash'),
    `<text class="note" x="450" y="212">A real error from central (5xx, 408, 429) backs off only that row; failed rows retry hourly.</text>`,
  ].join('')),

  linkGrace: svg(900, 250, [
    box(10, 30, 270, 90, 'one request fails', 'plain', 'tracker: down at once\nprobe + push back-off start'),
    box(315, 30, 270, 90, 'up to 30 s', 'green', 'light keeps the last good state\n(up, or connecting)'),
    box(620, 30, 270, 90, 'still silent at 30 s', 'orange', 'No central, or No internet\nif the probe failed too'),
    arrow([[280, 75], [315, 75]]), arrow([[585, 75], [620, 75]]),
    arrow([[450, 120], [450, 180], [145, 180], [145, 120]], 'any answer inside the grace: nothing ever showed', 'dash'),
    `<text class="note" x="450" y="232">The outage counts from whichever went silent first; a timer refetches when the grace ends.</text>`,
  ].join('')),

  pricing: svg(900, 220, [
    box(10, 40, 200, 80, 'kashi', 'blue', 'decides price lists,\ndiscounts, deals'),
    box(245, 40, 190, 80, 'price list on till', 'plain', 'read-only mirror,\narrives whole'),
    box(470, 40, 200, 80, 'priceInvoiceTotals', 'green', 'the one pricing path'),
    box(705, 20, 180, 50, 'Checkout', 'plain', 'salesInvoiceTx'),
    box(705, 90, 180, 50, 'Live quote', 'plain', 'InvoiceQuote'),
    arrow([[210, 80], [245, 80]]), arrow([[435, 80], [470, 80]]),
    arrow([[670, 65], [705, 45]]), arrow([[670, 95], [705, 115]]),
    `<text class="note" x="450" y="180">Not in an active price list means no price, and checkout refuses a zero price.</text>`,
    `<text class="note" x="450" y="204">All money is integer USD cents, whatever the customer paid with.</text>`,
  ].join('')),

  shift: svg(900, 200, [
    box(10, 40, 160, 80, 'Open shift', 'green', 'opening float'),
    box(200, 40, 160, 80, 'Sales', 'plain', 'a sale needs\nan open shift'),
    box(390, 40, 160, 80, 'Close', 'orange', 'count the drawer'),
    box(580, 40, 150, 80, 'Receipt photo', 'blue', 'QR code,\nphone uploads'),
    box(760, 40, 130, 80, 'Pushed', 'plain', 'to central'),
    arrow([[170, 80], [200, 80]]), arrow([[360, 80], [390, 80]]), arrow([[550, 80], [580, 80]]), arrow([[730, 80], [760, 80]]),
    `<text class="note" x="450" y="165">Expenses and loans do not need an open shift.</text>`,
  ].join('')),

  attendance: svg(900, 220, [
    box(10, 50, 160, 80, 'Akuvox punch', 'plain', 'door device'),
    box(200, 50, 170, 80, 'ping :8789', 'orange', 'only from the\nconfigured device IP'),
    box(400, 50, 170, 80, 'wait 0.5 s', 'gray', 'then pull\n(retry once at 1.5 s)'),
    box(600, 50, 150, 80, 'doorlog pull', 'blue', 'last 7 days,\nfirst 2000 records'),
    box(775, 50, 115, 80, 'import', 'green', 'linked users'),
    arrow([[170, 90], [200, 90]]), arrow([[370, 90], [400, 90]]), arrow([[570, 90], [600, 90]]), arrow([[750, 90], [775, 90]]),
    `<text class="note" x="450" y="180">No background timer: also pulls at startup and every 60 s while the Attendance page is open.</text>`,
  ].join('')),

  update: svg(900, 330, [
    box(10, 20, 150, 70, 'make bump', 'gray', 'commit + tag'),
    box(190, 20, 160, 70, 'make publish', 'gray', 'build, sign, upload'),
    box(380, 20, 150, 70, 'kashi', 'blue', 'chooses which\ntills get it'),
    box(560, 20, 160, 70, 'till downloads', 'plain', 'size, SHA-256,\nsignature checked'),
    box(750, 20, 140, 70, 'safe moment', 'plain', 'no open shift,\ncountdown'),
    arrow([[160, 55], [190, 55]]), arrow([[350, 55], [380, 55]]), arrow([[530, 55], [560, 55]]), arrow([[720, 55], [750, 55]]),
    box(300, 150, 300, 70, 'helper swaps the exe', 'orange', 'old exe kept beside the new one'),
    arrow([[820, 90], [820, 120], [450, 120], [450, 150]]),
    box(100, 260, 280, 50, 'new version confirms: done', 'green'),
    box(520, 260, 300, 50, 'no confirmation: old exe restored', 'red'),
    arrow([[380, 220], [240, 260]]), arrow([[520, 220], [670, 260]]),
  ].join('')),
}
