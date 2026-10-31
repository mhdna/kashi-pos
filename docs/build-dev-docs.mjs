// Builds docs/dev-docs.html (a self-contained Material-style docs app) from
// the markdown named in dev-docs.config.mjs. No npm dependencies beyond @mdi/js (already a frontend
// dependency, used for the icons). Run with `make dev-docs`.
//
// Markdown subset: headings, paragraphs, lists (ordered lists render as
// numbered steps), tables, fenced code, hr, **bold**, *italic*, `code`,
// [links](...). Extensions:
//   ## 4. Title {icon=mdiName}      section (one page in the app); icon optional
//   ::: group Name                  starts a nav group
//   > [!TIP|NOTE|WARNING|DANGER|HISTORY] text   callout (continue with > lines)
//   ::: diagram name                an inline SVG from DIAGRAMS below
//   ::: image path | caption        an image relative to docs/
//   ::: timeline ... :::            lines "- 2026-06-26 | Title | text"
//   ::: details Title ... :::       collapsible block (markdown inside)
import { readFileSync, writeFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const dir = dirname(fileURLToPath(import.meta.url))
const config = (await import('./dev-docs.config.mjs')).default
const DIAGRAMS = config.diagrams
let mdi = {}
for (const from of config.mdiFrom) {
  try { mdi = createRequire(join(dir, from))('@mdi/js'); break } catch {}
}
const icon = (name, cls = '') =>
  `<svg class="ic ${cls}" viewBox="0 0 24 24" aria-hidden="true"><path d="${mdi[name] || mdi.mdiFileDocumentOutline}"/></svg>`

const esc = (s) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
const slug = (s) =>
  s.toLowerCase().replace(/`/g, '').replace(/^\d+\.\s*/, '').replace(/[^\w\s-]/g, '').trim().replace(/\s+/g, '-')

// ---------------------------------------------------------------- markdown
function inline(text) {
  const codes = []
  let t = text.replace(/`([^`]+)`/g, (_, c) => {
    codes.push(`<code>${esc(c)}</code>`)
    return `\u0000${codes.length - 1}\u0000`
  })
  t = esc(t)
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[\s(])\*([^*\s][^*]*)\*/g, '$1<em>$2</em>')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_, l, u) =>
      u.startsWith('#') ? `<a href="${u}" data-go="${u.slice(1)}">${l}</a>` : `<a href="${u}">${l}</a>`)
  return t.replace(/\u0000(\d+)\u0000/g, (_, i) => codes[i])
}

const CALLOUT = {
  TIP: ['mdiLightbulbOnOutline', 'Tip'],
  NOTE: ['mdiInformationOutline', 'Note'],
  WARNING: ['mdiAlert', 'Watch out'],
  DANGER: ['mdiAlertOctagon', 'Danger'],
  HISTORY: ['mdiHistory', 'History'],
}

function renderBlocks(src) {
  const out = []
  let i = 0
  const isListLine = (l) => /^\s*([-*]|\d+\.) /.test(l)
  while (i < src.length) {
    const line = src[i]
    if (line.startsWith('```')) {
      const buf = []
      i++
      while (i < src.length && !src[i].startsWith('```')) buf.push(src[i++])
      i++
      out.push(`<pre><code>${esc(buf.join('\n'))}</code></pre>`)
    } else if (line.startsWith('::: diagram')) {
      const name = line.replace('::: diagram', '').trim()
      out.push(`<figure class="fig">${DIAGRAMS[name] || `<p>missing diagram ${name}</p>`}</figure>`)
      i++
    } else if (line.startsWith('::: image')) {
      const [path, cap] = line.replace('::: image', '').split('|').map((s) => s.trim())
      out.push(`<figure class="fig"><img src="${path}" alt="${esc(cap || '')}"><figcaption>${inline(cap || '')}</figcaption></figure>`)
      i++
    } else if (line.startsWith('::: timeline') || line.startsWith('::: details')) {
      const kind = line.startsWith('::: timeline') ? 'timeline' : 'details'
      const title = line.replace(/^::: (timeline|details)/, '').trim()
      const buf = []
      i++
      while (i < src.length && src[i].trim() !== ':::') buf.push(src[i++])
      i++
      if (kind === 'timeline') {
        out.push('<ol class="timeline">' + buf.filter((l) => l.startsWith('- ')).map((l) => {
          const [d, t, ...rest] = l.slice(2).split('|').map((s) => s.trim())
          return `<li><span class="dot"></span><time>${esc(d)}</time><h4>${inline(t)}</h4><p>${inline(rest.join(' | '))}</p></li>`
        }).join('') + '</ol>')
      } else {
        out.push(`<details><summary>${icon('mdiChevronRight', 'chev')}${inline(title)}</summary><div>${renderBlocks(buf)}</div></details>`)
      }
    } else if (line.startsWith('> ')) {
      const buf = []
      while (i < src.length && src[i].startsWith('>')) buf.push(src[i++].replace(/^>\s?/, ''))
      const m = buf[0].match(/^\[!(\w+)\]\s*(.*)$/)
      const [ic, label] = CALLOUT[m ? m[1] : 'NOTE']
      if (m) buf[0] = m[2]
      out.push(`<aside class="callout ${(m ? m[1] : 'NOTE').toLowerCase()}">${icon(ic)}<div><b>${label}</b>${renderBlocks(buf)}</div></aside>`)
    } else if (/^---+$/.test(line.trim())) {
      i++
    } else if (/^#{3,4} /.test(line)) {
      const level = line.match(/^#+/)[0].length
      const text = line.replace(/^#+ /, '')
      out.push(`<h${level} id="${slug(text)}">${inline(text)}</h${level}>`)
      i++
    } else if (line.startsWith('|')) {
      const rows = []
      while (i < src.length && src[i].startsWith('|')) rows.push(src[i++])
      const cells = (r) => r.replace(/^\||\|$/g, '').split('|').map((c) => c.trim())
      out.push(
        `<div class="table"><table><thead><tr>${cells(rows[0]).map((c) => `<th>${inline(c)}</th>`).join('')}</tr></thead><tbody>` +
          rows.slice(2).map((r) => `<tr>${cells(r).map((c) => `<td>${inline(c)}</td>`).join('')}</tr>`).join('') +
          '</tbody></table></div>',
      )
    } else if (isListLine(line)) {
      const ordered = /^\s*\d+\./.test(line)
      const items = []
      while (i < src.length && (isListLine(src[i]) || /^\s{2,}\S/.test(src[i]))) {
        if (isListLine(src[i])) {
          const text = src[i].replace(/^\s*([-*]|\d+\.) /, '')
          if (/^\s{2,}/.test(src[i]) && items.length) items[items.length - 1] += `<div class="sub">${inline(text)}</div>`
          else items.push(inline(text))
        } else if (items.length) items[items.length - 1] += ' ' + inline(src[i].trim())
        i++
      }
      const tag = ordered ? 'ol class="steps"' : 'ul'
      out.push(`<${tag}>${items.map((x) => `<li>${x}</li>`).join('')}</${ordered ? 'ol' : 'ul'}>`)
    } else if (line.trim() === '') {
      i++
    } else {
      const buf = [src[i++]]
      while (i < src.length && src[i].trim() !== '' && !/^(#{3,4} |```|\||---+$|> |::: |\s*([-*]|\d+\.) )/.test(src[i])) buf.push(src[i++])
      out.push(`<p>${inline(buf.join(' '))}</p>`)
    }
  }
  return out.join('\n')
}

// Parse the source into sections (one page each).
let title = config.title || 'Developer docs'
const sections = []
let all = []
if (config.source.file) {
  all = readFileSync(join(dir, config.source.file), 'utf8').split('\n')
  let group = ''
  let cur = null
  let inFence = false
  for (const line of all) {
    if (line.startsWith('```')) inFence = !inFence
    if (!inFence && /^# /.test(line)) { if (!config.title) title = line.slice(2); continue }
    if (!inFence && line.startsWith('::: group ')) { group = line.slice(10).trim(); continue }
    const m = !inFence && line.match(/^## (.+?)(?:\s*\{icon=(\w+)\})?\s*$/)
    if (m) {
      cur = { title: m[1].replace(/^\d+\.\s*/, ''), icon: m[2] || 'mdiFileDocumentOutline', group, lines: [] }
      cur.id = slug(m[1])
      sections.push(cur)
    } else if (cur) cur.lines.push(line)
  }
} else {
  // One markdown file per page. Page headings are demoted (## -> ###) and
  // links to sibling .md files become in-app links.
  const idByFile = {}
  for (const p of config.source.pages) idByFile[p.file.split('/').pop()] = p.id || slug(p.title)
  for (const p of config.source.pages) {
    const raw = readFileSync(join(dir, p.file), 'utf8').split('\n')
    let fence = false
    const out = []
    for (let l of raw) {
      if (l.startsWith('```')) fence = !fence
      if (!fence) {
        if (/^# /.test(l)) continue
        l = l.replace(/^(#{2,3}) /, (_, h) => h + '# ')
        l = l.replace(/\]\(([\w-]+\.md)(#[^)]*)?\)/g, (m, f) => (idByFile[f] ? `](#${idByFile[f]})` : m))
      }
      out.push(l)
    }
    sections.push({ id: p.id || slug(p.title), title: p.title, icon: p.icon, group: p.group, lines: out })
  }
}
for (const s of sections) {
  const inj = config.inject && config.inject[s.id]
  if (inj) s.lines = [...inj.flatMap((d) => [`::: diagram ${d}`, '']), ...s.lines]
}
for (const s of sections) {
  let fence = false
  s.lead = ''
  for (const l of s.lines) {
    if (l.startsWith('```')) { fence = !fence; continue }
    if (fence || !l.trim() || /^(#|\||>|:::|\s*[-*] |\s*\d+\. |---)/.test(l)) continue
    s.lead = l.trim().replace(/`/g, '').replace(/\*\*/g, '').replace(/\[([^\]]+)\]\([^)]*\)/g, '$1')
    break
  }
  s.html = renderBlocks(s.lines)
  s.toc = [...s.html.matchAll(/<h3 id="([^"]+)">(.*?)<\/h3>/g)].map((m) => [m[1], m[2].replace(/<[^>]+>/g, '')])
  s.text = s.html.replace(/<svg[\s\S]*?<\/svg>/g, ' ').replace(/<[^>]+>/g, ' ').replace(/&[a-z]+;/g, ' ').replace(/\s+/g, ' ')
}

const groups = []
for (const s of sections) {
  let g = groups.find((x) => x.name === s.group)
  if (!g) groups.push((g = { name: s.group, items: [] }))
  g.items.push(s)
}

const navHtml = groups.map((g) =>
  `<div class="ng">${esc(g.name)}</div>` +
  g.items.map((s) => `<a href="#${s.id}" data-id="${s.id}">${icon(s.icon)}<span>${esc(s.title)}</span></a>`).join('')).join('')

const cardsHtml = groups.map((g) =>
  `<h3 class="gh">${esc(g.name)}</h3><div class="cards">` +
  g.items.map((s) => `<a class="card" href="#${s.id}" data-id="${s.id}"><span class="ci">${icon(s.icon)}</span><b>${esc(s.title)}</b><span>${esc(s.lead.slice(0, 120))}${s.lead.length > 120 ? '...' : ''}</span></a>`).join('') + '</div>').join('')

const pagesHtml = sections.map((s, n) => {
  const prev = sections[n - 1]
  const next = sections[n + 1]
  return `<section class="page" id="p-${s.id}" hidden>
<div class="main"><header class="hero"><span class="hi">${icon(s.icon)}</span><div><div class="crumb">${esc(s.group)}</div><h1>${esc(s.title)}</h1></div></header>
${s.html}
<div class="pn">${prev ? `<a href="#${prev.id}" data-id="${prev.id}">${icon('mdiChevronLeft')}<span><small>Previous</small>${esc(prev.title)}</span></a>` : '<span></span>'}${next ? `<a class="r" href="#${next.id}" data-id="${next.id}"><span><small>Next</small>${esc(next.title)}</span>${icon('mdiChevronRight')}</a>` : ''}</div></div>
<aside class="toc">${s.toc.length ? '<b>On this page</b>' + s.toc.map(([id, t]) => `<a href="#${s.id}/${id}" data-id="${s.id}" data-h="${id}">${esc(t)}</a>`).join('') : ''}</aside>
</section>`
}).join('\n')

const searchIndex = JSON.stringify(sections.map((s) => ({ id: s.id, t: s.title, g: s.group, x: s.text })))

const css = `
:root{--bar:#3f51b5;--p:#3f51b5;--pd:#303f9f;--pl:#c5cae9;--acc:#ff4081;--s:#fff;--sv:#f5f5f5;--bg:#eeeeee;--t:rgba(0,0,0,.87);--m:rgba(0,0,0,.54);--o:rgba(0,0,0,.12);--hv:rgba(0,0,0,.06);--act:#eeeeee;--link:#3f51b5;
--ok:#2e7d32;--okc:#e8f5e9;--wn:#e65100;--wnc:#fff3e0;--er:#c62828;--erc:#ffebee;--pu:#6a1b9a;--puc:#f3e5f5;--bl:#1565c0;--blc:#e3f2fd;
--e1:0 2px 2px 0 rgba(0,0,0,.14),0 3px 1px -2px rgba(0,0,0,.12),0 1px 5px 0 rgba(0,0,0,.2);--e2:0 4px 5px 0 rgba(0,0,0,.14),0 1px 10px 0 rgba(0,0,0,.12),0 2px 4px -1px rgba(0,0,0,.2);--eb:0 2px 4px -1px rgba(0,0,0,.2),0 4px 5px 0 rgba(0,0,0,.14),0 1px 10px 0 rgba(0,0,0,.12)}
:root[data-theme=dark]{--bar:#212121;--p:#9fa8da;--pd:#7986cb;--pl:#3949ab;--s:#424242;--sv:#4a4a4a;--bg:#303030;--t:#fff;--m:rgba(255,255,255,.7);--o:rgba(255,255,255,.12);--hv:rgba(255,255,255,.08);--act:rgba(255,255,255,.12);--link:#9fa8da;
--ok:#81c784;--okc:#1b3a1f;--wn:#ffb74d;--wnc:#3e2a10;--er:#ef9a9a;--erc:#4a1c1c;--pu:#ce93d8;--puc:#341d3d;--bl:#90caf9;--blc:#17304a}
*{box-sizing:border-box}html{scroll-behavior:smooth;scroll-padding-top:80px}
body{margin:0;background:var(--bg);color:var(--t);font:400 17px/30px Roboto,"Helvetica Neue",Helvetica,Arial,system-ui,sans-serif;-webkit-font-smoothing:antialiased}
.ic{width:24px;height:24px;fill:currentColor;flex:none}
header.bar{position:fixed;inset:0 0 auto 0;height:64px;background:var(--bar);color:#fff;box-shadow:var(--eb);display:flex;align-items:center;gap:8px;padding:0 16px;z-index:20}
.bar .logo{display:none}
.bar .tt{font:500 24px/1 Roboto,sans-serif;cursor:pointer;white-space:nowrap;margin-left:8px;letter-spacing:.2px}
.bar .sp{flex:1}
.ib{border:0;background:none;color:#fff;width:48px;height:48px;border-radius:50%;display:grid;place-items:center;cursor:pointer}.ib:hover{background:rgba(255,255,255,.15)}
#menu{display:grid}
.sw{position:relative;width:min(520px,40vw)}
.sw input{width:100%;height:44px;border:0;border-radius:2px;background:rgba(255,255,255,.18);color:#fff;padding:0 16px 0 52px;font:inherit;font-size:18px;outline:none}
.sw input::placeholder{color:rgba(255,255,255,.7)}
.sw input:focus{background:#fff;color:rgba(0,0,0,.87)}.sw input:focus::placeholder{color:rgba(0,0,0,.54)}.sw input:focus~.ic{color:rgba(0,0,0,.54)}
.sw .ic{position:absolute;left:14px;top:10px;color:#fff;pointer-events:none}
#res{position:absolute;top:44px;left:0;right:0;background:var(--s);color:var(--t);border-radius:2px;box-shadow:var(--e2);max-height:70vh;overflow:auto;display:none;padding:8px 0}
#res a{display:block;font-size:16px;padding:12px 16px;color:var(--t);text-decoration:none}#res a:hover{background:var(--hv)}
#res small{color:var(--m);display:block;font-size:15px;line-height:22px}#res mark{background:#fff59d;color:#000}
nav.side{position:fixed;top:64px;bottom:0;left:0;width:320px;overflow:auto;padding:8px 0 32px;background:var(--s);box-shadow:1px 0 3px rgba(0,0,0,.2);z-index:10;transition:transform .2s}
nav.side.closed{transform:translateX(-100%)}
.ng{font:500 15px/52px Roboto,sans-serif;color:var(--m);padding:0 20px;border-top:1px solid var(--o);margin-top:8px}.ng:first-child{border-top:0;margin-top:0}
nav.side a{display:flex;align-items:center;gap:28px;height:54px;padding:0 20px;color:var(--t);text-decoration:none;font:500 16.5px Roboto,sans-serif}
nav.side a:hover{background:var(--hv)}nav.side a.on{background:var(--act);color:var(--p)}nav.side .ic{width:24px;height:24px;color:var(--m)}nav.side a.on .ic{color:var(--p)}
main{margin:64px 0 0 320px;min-height:calc(100vh - 64px);transition:margin .2s}main.full{margin-left:0}
.page{display:flex;gap:24px;padding:24px;max-width:1500px;margin:0 auto}.page[hidden]{display:none}
.main{flex:1;min-width:0;max-width:1000px}
.toc{width:240px;flex:none;position:sticky;top:88px;align-self:flex-start;font-size:15px;line-height:22px}
.toc b{display:block;font:500 14px Roboto,sans-serif;color:var(--m);margin-bottom:8px}
.toc a{display:block;color:var(--m);text-decoration:none;padding:6px 0 6px 14px;border-left:2px solid var(--o)}.toc a:hover{color:var(--t)}.toc a.on{color:var(--p);border-color:var(--p)}
.hero{display:flex;align-items:center;gap:20px;margin-bottom:24px;padding:24px;background:var(--s);border-radius:2px;box-shadow:var(--e1)}
.hi{width:68px;height:68px;border-radius:50%;background:var(--p);color:#fff;display:grid;place-items:center;box-shadow:var(--e1)}:root[data-theme=dark] .hi{color:#212121}.hi .ic{width:34px;height:34px}
.crumb{font:400 14px Roboto,sans-serif;color:var(--m)}
h1{font:400 42px/50px Roboto,sans-serif;margin:0}
.main>.paper,.main>p,.main>ul,.main>ol.steps,.main>.table,.main>pre,.main>.fig,.main>details,.main>.callout{margin:16px 0}
.main>h3{font:400 30px/38px Roboto,sans-serif;margin:40px 0 8px;padding-bottom:8px;border-bottom:1px solid var(--o)}
.main>h4{font:500 19px Roboto,sans-serif;color:var(--m);margin:24px 0 8px}
p{margin:0}ul{padding-left:24px}li{margin:4px 0}.sub{color:var(--m);font-size:15px}
a{color:var(--link);text-decoration:none}a:hover{text-decoration:underline}
code{font:15px/1.5 "Roboto Mono",ui-monospace,Menlo,monospace;background:var(--hv);padding:1px 5px;border-radius:2px;color:inherit}
pre{overflow:auto;background:var(--s);border-radius:2px;box-shadow:var(--e1);padding:0}pre code{display:block;padding:16px 20px;background:none;line-height:1.6;white-space:pre}
.table{overflow:auto;background:var(--s);border-radius:2px;box-shadow:var(--e1)}
table{border-collapse:collapse;width:100%;font-size:16px;line-height:24px}
th{text-align:left;font:500 14px Roboto,sans-serif;color:var(--m);padding:0 16px;height:52px;border-bottom:1px solid var(--o)}
td{border-top:1px solid var(--o);padding:14px 16px;vertical-align:top;color:var(--t)}tbody tr:first-child td{border-top:0}tbody tr:hover{background:var(--hv)}
.callout{display:flex;gap:16px;padding:16px;border-radius:2px;background:var(--s);box-shadow:var(--e1);border-left:4px solid var(--bl)}
.callout .ic{margin-top:2px;width:28px;height:28px}.callout b{display:block;font:500 16px Roboto,sans-serif;margin-bottom:2px}.callout p{margin:0}
.callout.tip{border-color:var(--ok)}.callout.tip .ic,.callout.tip b{color:var(--ok)}
.callout.warning{border-color:var(--wn)}.callout.warning .ic,.callout.warning b{color:var(--wn)}
.callout.danger{border-color:var(--er)}.callout.danger .ic,.callout.danger b{color:var(--er)}
.callout.history{border-color:var(--pu)}.callout.history .ic,.callout.history b{color:var(--pu)}
.callout.note .ic,.callout.note b{color:var(--bl)}
ol.steps{list-style:none;counter-reset:s;padding:0;background:var(--s);border-radius:2px;box-shadow:var(--e1)}
ol.steps li{counter-increment:s;position:relative;padding:16px 16px 16px 68px;margin:0;border-top:1px solid var(--o);min-height:60px}ol.steps li:first-child{border-top:0}
ol.steps li:before{content:counter(s);position:absolute;left:16px;top:16px;width:34px;height:34px;border-radius:50%;background:var(--p);color:#fff;display:grid;place-items:center;font:500 16px Roboto,sans-serif}
:root[data-theme=dark] ol.steps li:before{color:#212121}
.fig{background:var(--s);border-radius:2px;box-shadow:var(--e1);padding:16px;text-align:center}.fig img{max-width:100%;background:#fff}figcaption{color:var(--m);font-size:14px;margin-top:8px}
.diagram{width:100%;height:auto;font-family:Roboto,sans-serif}
.box rect{fill:var(--sv);stroke:var(--o);stroke-width:1;filter:drop-shadow(0 1px 2px rgba(0,0,0,.25))}.bt{font-size:16.5px;font-weight:500;fill:var(--t);text-anchor:middle}.bs{font-size:14px;fill:var(--m);text-anchor:middle}
.box.blue rect{fill:var(--blc);stroke:var(--bl)}.box.green rect{fill:var(--okc);stroke:var(--ok)}.box.orange rect{fill:var(--wnc);stroke:var(--wn)}.box.red rect{fill:var(--erc);stroke:var(--er)}.box.purple rect{fill:var(--puc);stroke:var(--pu)}.box.plain rect{fill:var(--s)}
.ar{fill:none;stroke:var(--m);stroke-width:1.5}.ar.dash{stroke-dasharray:6 5}.ahp{fill:var(--m)}.lb{fill:var(--s);opacity:.95}.al{font-size:13.5px;fill:var(--m);text-anchor:middle}.note{font-size:15px;fill:var(--m);text-anchor:middle}
details{background:var(--s);border-radius:2px;box-shadow:var(--e1);overflow:hidden}
summary{cursor:pointer;list-style:none;display:flex;align-items:center;gap:8px;padding:0 16px;height:56px;font:500 17px Roboto,sans-serif}summary:hover{background:var(--hv)}summary::-webkit-details-marker{display:none}
details[open]>summary{border-bottom:1px solid var(--o)}details[open]>summary .chev{transform:rotate(90deg)}.chev{transition:transform .15s;color:var(--m)}details>div{padding:8px 16px 12px}details>div>p,details>div>ul{margin:8px 0}
ol.timeline{list-style:none;margin:24px 0;padding:0 0 0 32px;border-left:2px solid var(--pl)}
.timeline li{position:relative;margin:0 0 16px;padding:12px 16px;background:var(--s);border-radius:2px;box-shadow:var(--e1)}
.timeline .dot{position:absolute;left:-42px;top:16px;width:20px;height:20px;border-radius:50%;background:var(--p);border:4px solid var(--bg);box-shadow:var(--e1)}
.timeline time{font:500 14px Roboto,sans-serif;color:var(--p)}.timeline h4{margin:0 0 2px;font:500 19px Roboto,sans-serif}.timeline p{color:var(--m);font-size:16px}
.pn{display:flex;justify-content:space-between;gap:16px;margin-top:40px;border-top:1px solid var(--o);padding-top:16px}
.pn a{display:flex;align-items:center;gap:8px;padding:8px 16px;min-height:52px;border-radius:2px;color:var(--p);text-transform:uppercase;font:500 16px Roboto,sans-serif;letter-spacing:.5px}.pn a:hover{background:var(--hv);text-decoration:none}
.pn small{display:block;color:var(--m);font-size:13px;text-transform:none;letter-spacing:0;line-height:14px}.pn .r{text-align:right;margin-left:auto}
.home{max-width:1240px;margin:0 auto;padding:0 24px 80px}
.home .big{margin:0 -24px 24px;padding:48px 48px 32px;background:var(--bar);color:#fff;box-shadow:var(--e1)}
.home .big h1{font:300 54px/62px Roboto,sans-serif;margin:0 0 12px;color:#fff}.home .big p{color:rgba(255,255,255,.87);max-width:760px;font-size:19px;line-height:30px}.home .big code{background:rgba(255,255,255,.2);color:#fff}.home .big a{color:#ff80ab}.home .big .chips a{color:#fff}
.chips{display:flex;flex-wrap:wrap;gap:8px;margin-top:20px}.chips a{display:flex;align-items:center;gap:8px;padding:0 16px;height:42px;border-radius:2px;color:#fff;text-transform:uppercase;font:500 16px Roboto,sans-serif;letter-spacing:.5px;background:rgba(255,255,255,.15)}.chips a:hover{background:rgba(255,255,255,.28);text-decoration:none}
.chips .ic{width:18px;height:18px}.gh{font:500 16px Roboto,sans-serif;color:var(--m);margin:32px 0 8px;padding-bottom:8px;border-bottom:1px solid var(--o)}
.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:16px}
.card{display:flex;flex-direction:column;gap:6px;padding:16px;background:var(--s);border-radius:2px;box-shadow:var(--e1);color:var(--t);transition:box-shadow .2s}
.card:hover{box-shadow:var(--e2);text-decoration:none}.card b{font:500 19px Roboto,sans-serif}.card span:last-child{color:var(--m);font-size:15.5px;line-height:23px}
.ci{width:48px;height:48px;border-radius:50%;background:var(--p);color:#fff;display:grid;place-items:center;margin-bottom:6px}:root[data-theme=dark] .ci{color:#212121}
#fab{position:fixed;right:24px;bottom:24px;width:56px;height:56px;border-radius:50%;border:0;background:var(--acc);color:#fff;display:grid;place-items:center;cursor:pointer;box-shadow:0 3px 5px -1px rgba(0,0,0,.2),0 6px 10px 0 rgba(0,0,0,.14),0 1px 18px 0 rgba(0,0,0,.12);z-index:30;opacity:0;transform:scale(.6);pointer-events:none;transition:opacity .2s,transform .2s}
#fab.show{opacity:1;transform:none;pointer-events:auto}#fab:hover{filter:brightness(1.08)}
@media(max-width:1100px){.toc{display:none}}
@media(max-width:860px){nav.side{transform:translateX(-100%)}nav.side.open{transform:none;box-shadow:var(--e2)}main{margin-left:0}.page{padding:12px}.home{padding:0 12px 60px}.home .big{margin:0 -12px 16px;padding:32px 24px 24px}}
`

const js = `
const IDX=${searchIndex};
const $=(s,e=document)=>e.querySelector(s),$$=(s,e=document)=>[...e.querySelectorAll(s)];
const pages=$$('.page'),navs=$$('nav.side a');
function show(hash){
  const [id,h]=(hash||'').replace(/^#/,'').split('/');
  const known=IDX.some(x=>x.id===id);
  pages.forEach(p=>p.hidden=!(known&&p.id==='p-'+id));
  $('#home').hidden=known;
  navs.forEach(a=>a.classList.toggle('on',a.dataset.id===id));
  document.title=(known?IDX.find(x=>x.id===id).t+' · ':'')+${JSON.stringify(title)};
  $('nav.side').classList.remove('open');
  if(h){const el=document.querySelector('#p-'+id+' [id="'+h+'"]');el&&el.scrollIntoView()}else window.scrollTo(0,0);
}
addEventListener('hashchange',()=>show(location.hash));
document.addEventListener('click',e=>{const a=e.target.closest('a[data-go]');if(a){e.preventDefault();location.hash=a.dataset.go}});
show(location.hash);
$('#menu').onclick=()=>{const n=$('nav.side');if(innerWidth>860){n.classList.toggle('closed');$('main').classList.toggle('full')}else n.classList.toggle('open')};
$('.tt').onclick=()=>location.hash='';
const fab=$('#fab');fab.onclick=()=>scrollTo({top:0,behavior:'smooth'});addEventListener('scroll',()=>fab.classList.toggle('show',scrollY>400));
const root=document.documentElement;
const mq=matchMedia('(prefers-color-scheme:dark)');
const saved=()=>localStorage.getItem('devdocs-theme');
const apply=()=>{root.dataset.theme=saved()||(mq.matches?'dark':'light')};
apply();mq.addEventListener('change',apply);
$('#theme').onclick=()=>{const next=root.dataset.theme==='dark'?'light':'dark';localStorage.setItem('devdocs-theme',next);apply()};
// scroll-spy for the right-hand table of contents
const spy=new IntersectionObserver(es=>es.forEach(e=>{if(e.isIntersecting){$$('.toc a').forEach(a=>a.classList.toggle('on',a.dataset.h===e.target.id))}}),{rootMargin:'-90px 0px -75% 0px'});
$$('.main>h3').forEach(h=>spy.observe(h));
// search across every page
const q=$('#q'),res=$('#res');
const rx=s=>s.replace(/[.*+?^\${}()|[\\]\\\\]/g,'\\\\$&');
function search(){
  const v=q.value.trim().toLowerCase();
  if(v.length<2){res.style.display='none';return}
  const out=[];
  for(const p of IDX){
    const tl=p.t.toLowerCase().includes(v),i=p.x.toLowerCase().indexOf(v);
    if(tl||i>=0){
      let sn='';
      if(i>=0){const s=Math.max(0,i-50);sn=p.x.slice(s,i+90).replace(new RegExp(rx(v),'ig'),m=>'<mark>'+m+'</mark>')}
      out.push('<a href="#'+p.id+'"><b>'+p.t+'</b><small>'+(sn||p.g)+'</small></a>');
    }
  }
  res.innerHTML=out.join('')||'<a><small>No matches</small></a>';res.style.display='block';
}
q.addEventListener('input',search);q.addEventListener('focus',search);
document.addEventListener('click',e=>{if(!e.target.closest('.sw'))res.style.display='none';else if(e.target.closest('#res a')){res.style.display='none';q.value=''}});
addEventListener('keydown',e=>{if(e.key==='/'&&document.activeElement!==q){e.preventDefault();q.focus()}if(e.key==='Escape'){res.style.display='none';q.blur()}});
`

const intro = config.source.file
  ? renderBlocks(all.slice(0, all.findIndex((l) => l.startsWith('## '))).filter((l) => !l.startsWith('# ') && !l.startsWith('::: group')))
  : renderBlocks(config.intro.split('\n'))

const favSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="14" fill="${config.favicon.bg}"/><g transform="translate(10 10) scale(1.83)"><path fill="#fff" d="${mdi[config.favicon.icon] || ''}"/></g></svg>`
const favicon = 'data:image/svg+xml,' + encodeURIComponent(favSvg)

const html = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>${esc(title)}</title>
<link rel="icon" href="${favicon}">
<!-- Generated from docs/DEV_DOCS.md by docs/build-dev-docs.mjs. Do not edit; run make dev-docs. -->
<script>(function(){var t=null;try{t=localStorage.getItem('devdocs-theme')}catch(e){}document.documentElement.dataset.theme=t||(matchMedia('(prefers-color-scheme:dark)').matches?'dark':'light')})()</script>
<style>${css}</style></head>
<body>
<header class="bar"><button class="ib" id="menu" aria-label="Menu">${icon('mdiMenu')}</button><div class="tt">${esc(title)}</div><div class="sp"></div>
<div class="sw">${icon('mdiMagnify')}<input id="q" type="search" placeholder="Search the docs  ( / )" autocomplete="off"><div id="res"></div></div>
<button class="ib" id="theme" title="Toggle theme">${icon('mdiThemeLightDark')}</button></header>
<nav class="side">${navHtml}</nav>
<main>
<div class="home" id="home"><div class="big"><h1>${esc(title)}</h1><div>${intro}</div>
<div class="chips">${config.chips.map(([id, label, ic]) => `<a href="#${id}" data-id="${id}">${icon(ic)}${esc(label)}</a>`).join('')}</div></div>
${cardsHtml}</div>
${pagesHtml}
</main>
<button id="fab" title="Back to top">${icon('mdiChevronUp')}</button>
<script>${js}</script>
</body></html>
`
writeFileSync(join(dir, config.out), html)
console.log(`wrote ${config.out} (${sections.length} pages)`)
