// SVG helpers for the developer-docs diagrams. Shared, identical in every
// repo that has docs/build-dev-docs.mjs; per-repo diagrams live in
// dev-docs-diagrams.mjs.
export const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
export const lines = (t) => String(t).split('\n')
export function box(x, y, w, h, label, cls = '', sub = '') {
  const ls = lines(label)
  const ss = sub ? lines(sub) : []
  const total = ls.length + ss.length * 0.85
  const y0 = y + h / 2 - (total * 18) / 2 + 14
  const t =
    ls.map((l, i) => `<text class="bt" x="${x + w / 2}" y="${y0 + i * 18}">${esc(l)}</text>`).join('') +
    ss.map((l, i) => `<text class="bs" x="${x + w / 2}" y="${y0 + ls.length * 18 + i * 15 + 2}">${esc(l)}</text>`).join('')
  return `<g class="box ${cls}"><rect x="${x}" y="${y}" width="${w}" height="${h}" rx="3"/>${t}</g>`
}
export function arrow(pts, label = '', cls = '') {
  const d = pts.map((p, i) => (i ? 'L' : 'M') + p.join(' ')).join(' ')
  let lab = ''
  if (label) {
    const a = pts[Math.floor((pts.length - 1) / 2)]
    const b = pts[Math.floor((pts.length - 1) / 2) + 1]
    const mx = (a[0] + b[0]) / 2
    const my = (a[1] + b[1]) / 2
    const w = Math.max(...lines(label).map((l) => l.length)) * 7 + 14
    const h = lines(label).length * 17 + 6
    lab = `<rect class="lb" x="${mx - w / 2}" y="${my - h / 2}" width="${w}" height="${h}" rx="2"/>` +
      lines(label).map((l, i) => `<text class="al" x="${mx}" y="${my - h / 2 + 16 + i * 17 - 2}">${esc(l)}</text>`).join('')
  }
  return `<path class="ar ${cls}" d="${d}" marker-end="url(#ah)"/>${lab}`
}
export const svg = (w, h, body) =>
  `<svg class="diagram" viewBox="0 0 ${w} ${h}" role="img"><defs><marker id="ah" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0L10 5L0 10z" class="ahp"/></marker></defs>${body}</svg>`

