import type { Bootstrap, Card } from './types'

export interface UI {
  launcher: HTMLButtonElement
  panel: HTMLDivElement
  head: { avatar: HTMLDivElement; name: HTMLDivElement; site: HTMLDivElement; history: HTMLButtonElement }
  log: HTMLDivElement
  input: HTMLTextAreaElement
  send: HTMLButtonElement
}

export const MAX_QUESTION = 2000

export function buildUI(root: ShadowRoot): UI {
  const launcher = el('button', 'launcher') as HTMLButtonElement
  launcher.type = 'button'
  launcher.setAttribute('aria-label', 'เปิดผู้ช่วยหลังบ้าน')
  launcher.setAttribute('aria-expanded', 'false')
  launcher.dataset.open = 'false'

  const panel = el('div', 'panel') as HTMLDivElement
  panel.dataset.open = 'false'
  panel.setAttribute('role', 'dialog')
  panel.setAttribute('aria-label', 'ผู้ช่วยหลังบ้าน')

  const head = el('div', 'head')
  const avatar = el('div', 'avatar') as HTMLDivElement
  const nameWrap = el('div', 'namewrap')
  const name = el('div', 'name') as HTMLDivElement
  nameWrap.appendChild(name)
  const site = el('div', 'site') as HTMLDivElement
  site.title = 'เว็บที่กำลังคุยอยู่'
  // ปุ่มประวัติ — index.ts ซ่อนในโหมดคอนโซล/preview (ห้องอยู่ที่หน้าเว็บ ไม่มีประวัติฝั่ง server)
  const history = el('button', 'x hist') as HTMLButtonElement
  history.type = 'button'
  history.textContent = 'ประวัติ'
  history.title = 'ห้องแชทย้อนหลัง 7 วัน'
  history.setAttribute('aria-label', 'ห้องแชทย้อนหลัง 7 วัน')
  const close = el('button', 'x') as HTMLButtonElement
  close.type = 'button'
  close.textContent = '×'
  close.setAttribute('aria-label', 'ปิด')
  head.append(avatar, nameWrap, site, history, close)

  const log = el('div', 'log') as HTMLDivElement
  log.setAttribute('aria-live', 'polite')

  const foot = el('div', 'foot')
  const input = el('textarea') as HTMLTextAreaElement
  input.rows = 1
  input.maxLength = MAX_QUESTION
  input.placeholder = 'พิมพ์คำถาม…'
  input.setAttribute('aria-label', 'คำถาม')
  const send = el('button', 'send') as HTMLButtonElement
  send.type = 'button'
  send.textContent = '↑'
  send.setAttribute('aria-label', 'ส่ง')
  foot.append(input, send)

  panel.append(head, log, foot)
  root.append(launcher, panel)

  const ui = { launcher, panel, head: { avatar, name, site, history }, log, input, send }
  close.addEventListener('click', () => setOpen(ui, false))
  return ui
}

/** เปิด/ปิดแผงแชท — ปุ่มลอยเปลี่ยนเป็น × ตามไปด้วย */
export function setOpen(ui: UI, open: boolean) {
  ui.panel.dataset.open = String(open)
  ui.launcher.dataset.open = String(open)
  ui.launcher.setAttribute('aria-expanded', String(open))
  ui.launcher.setAttribute('aria-label', open ? 'ปิดผู้ช่วยหลังบ้าน' : 'เปิดผู้ช่วยหลังบ้าน')
}

const SVG_NS = 'http://www.w3.org/2000/svg'

/** ไอคอนเส้น (stroke = currentColor → ใช้สีตัวอักษรบนสีหลักอัตโนมัติ) · path คงที่ของเราเอง */
function icon(cls: string, paths: { d: string; fill?: boolean }[]): SVGSVGElement {
  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('viewBox', '0 0 24 24')
  svg.setAttribute('aria-hidden', 'true')
  svg.setAttribute('class', cls)
  for (const p of paths) {
    const path = document.createElementNS(SVG_NS, 'path')
    path.setAttribute('d', p.d)
    if (p.fill) path.setAttribute('class', 'fill')
    svg.appendChild(path)
  }
  return svg
}

// ฟองแชท + ประกาย (ผู้ช่วยอัตโนมัติ)
const CHAT_SPARK = [
  { d: 'M3.5 6.25A3.25 3.25 0 0 1 6.75 3h10.5a3.25 3.25 0 0 1 3.25 3.25v7.5A3.25 3.25 0 0 1 17.25 17H11.5l-4.1 3.4c-.7.58-1.9.1-1.9-.83V17A3.25 3.25 0 0 1 3.5 13.75z' },
  { d: 'M12 5.6l1.05 2.6 2.6 1.05-2.6 1.05L12 12.9l-1.05-2.6-2.6-1.05 2.6-1.05z', fill: true },
]
const CLOSE = [{ d: 'M7 7l10 10M17 7L7 17' }]

function launcherFace(ui: UI, avatarURL: string) {
  ui.launcher.textContent = ''
  const face = el('span', 'face')
  if (avatarURL) face.appendChild(img(avatarURL))
  else face.appendChild(icon('ico', CHAT_SPARK))
  ui.launcher.append(face, icon('ico close', CLOSE))
}

/** ข้อความทุกชิ้นลงด้วย textContent — ไม่มี innerHTML กับข้อความที่ไม่ได้มาจากเรา */
export function addBubble(log: HTMLDivElement, who: 'me' | 'ai', text: string, extra = ''): HTMLDivElement {
  const row = el('div', 'row ' + who)
  const b = el('div', 'bubble' + (extra ? ' ' + extra : '')) as HTMLDivElement
  const t = el('div', 'txt')
  t.textContent = text
  b.appendChild(t)
  row.appendChild(b)
  log.appendChild(row)
  scroll(log)
  return b
}

/** ข้อความระบบกลางห้อง (เปลี่ยนเว็บ · ห้องปิด · อ่านอย่างเดียว) */
export function addSys(log: HTMLDivElement, text: string): HTMLDivElement {
  const n = el('div', 'sys') as HTMLDivElement
  n.textContent = text
  log.appendChild(n)
  scroll(log)
  return n
}

/** แถบ "กำลังดึงข้อมูล…" — แทนที่ข้อความได้ตาม status ที่ server ส่ง */
export function addLoader(log: HTMLDivElement, text: string): { set(t: string): void; remove(): void } {
  const row = el('div', 'row ai')
  const b = el('div', 'bubble load')
  const t = el('span')
  t.textContent = text
  const dots = el('span', 'dots')
  dots.append(el('span'), el('span'), el('span'))
  b.append(t, dots)
  row.appendChild(b)
  log.appendChild(row)
  scroll(log)
  return {
    set: (s) => (t.textContent = s),
    remove: () => row.remove(),
  }
}

/** รายการห้องย้อนหลัง — ใช้หน้าตาการ์ดเดิม (หัวข้อ + เวลา) กดทั้งใบได้ */
export function roomItem(title: string, when: string, onOpen: () => void): HTMLButtonElement {
  const b = el('button', 'datacard room') as HTMLButtonElement
  b.type = 'button'
  const t = el('div', 'dc-title')
  t.textContent = title
  b.appendChild(t)
  if (when) {
    const src = el('div', 'src')
    src.textContent = when
    b.appendChild(src)
  }
  b.addEventListener('click', onOpen)
  return b
}

/**
 * การ์ดข้อมูล — ค่าจากระบบตรง ๆ ไม่ผ่านโมเดล
 * บอกเวลาที่ดึงเสมอ และมีลิงก์ไปหน้าจริงถ้า connector ให้มา
 */
export function renderCard(card: Card): HTMLDivElement {
  const box = el('div', 'datacard k-' + safeClass(card.kind)) as HTMLDivElement
  if (card.title) {
    const t = el('div', 'dc-title')
    t.textContent = card.title
    box.appendChild(t)
  }
  for (const f of card.fields ?? []) {
    const r = el('div', 'dc-row')
    const l = el('span')
    l.textContent = f.label
    const v = el('b')
    v.textContent = f.display
    r.append(l, v)
    box.appendChild(r)
  }
  if (card.table && card.table.rows?.length) {
    const wrap = el('div', 'dc-tablewrap')
    const tbl = el('table', 'dc-table')
    const thead = el('thead')
    const hr = el('tr')
    for (const c of card.table.columns ?? []) {
      const th = el('th')
      th.textContent = c.label
      hr.appendChild(th)
    }
    thead.appendChild(hr)
    const tbody = el('tbody')
    for (const row of card.table.rows) {
      const tr = el('tr')
      for (const cell of row) {
        const td = el('td')
        td.textContent = cell?.display ?? ''
        tr.appendChild(td)
      }
      tbody.appendChild(tr)
    }
    tbl.append(thead, tbody)
    wrap.appendChild(tbl)
    box.appendChild(wrap)
  }
  if (card.note) {
    const n = el('div', 'dc-note')
    n.textContent = card.note
    box.appendChild(n)
  }
  const src = el('div', 'src')
  const at = el('span')
  at.textContent = card.kind === 'reference' ? 'จากคู่มือของระบบ' : 'ข้อมูล ณ ' + formatFetched(card.fetched_at) + (card.cached ? ' · ค่าที่ดึงไว้ไม่เกิน 1 นาที' : '')
  src.appendChild(at)
  const href = safeHref(card.link?.path)
  if (card.link && href) {
    const a = el('a') as HTMLAnchorElement
    a.href = href
    a.textContent = (card.link.label || 'เปิดหน้าจริง') + ' →'
    a.target = '_self'
    a.rel = 'noopener'
    src.appendChild(a)
  }
  box.appendChild(src)
  return box
}

/** เวลาไทยเสมอ (หลังบ้านทุกตัวตัดวันตาม Asia/Bangkok) · วันนี้โชว์แค่เวลา */
export function formatFetched(iso: string): string {
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '-'
  const opt = { timeZone: 'Asia/Bangkok' } as const
  try {
    const time = d.toLocaleTimeString('th-TH', { ...opt, hour: '2-digit', minute: '2-digit', hour12: false })
    const day = d.toLocaleDateString('en-CA', opt)
    const today = new Date().toLocaleDateString('en-CA', opt)
    if (day === today) return time
    const [, mm, dd] = day.split('-')
    return `${dd}/${mm} ${time}`
  } catch {
    return d.toISOString().slice(0, 16).replace('T', ' ')
  }
}

/** ลิงก์ในการ์ดต้องเป็น path ในหลังบ้านเดียวกัน — กัน javascript: / โดเมนอื่น */
export function safeHref(path: string | undefined): string {
  if (!path) return ''
  const p = path.trim()
  if (p.startsWith('#')) return p
  if (!p.startsWith('/') || p.startsWith('//') || p.includes('\\')) return ''
  return p
}

/** วางปุ่มลอยและ panel ตาม placement ที่ server ส่งมา */
export function applyPlacement(ui: UI, b: Bootstrap) {
  const { position, offset_x, offset_y } = b.placement
  const left = position === 'bottom-left'

  for (const node of [ui.launcher, ui.panel]) {
    node.style.left = ''
    node.style.right = ''
  }

  ui.launcher.style.bottom = px(offset_y)
  ui.panel.style.bottom = px(offset_y + 68)

  if (left) {
    ui.launcher.style.left = px(offset_x)
    ui.panel.style.left = px(offset_x)
  } else {
    ui.launcher.style.right = px(offset_x)
    ui.panel.style.right = px(offset_x)
  }
}

export function applyAppearance(ui: UI, b: Bootstrap) {
  ui.head.name.textContent = b.display_name || 'ผู้ช่วยหลังบ้าน'
  // ป้ายชื่อ service ค้างบนหัวตลอด — แอดมินต้องรู้ตลอดว่ากำลังคุยเรื่อง service ไหน
  ui.head.site.textContent = b.service_label || b.service_id || ''

  ui.head.avatar.textContent = ''
  if (b.avatar_url) ui.head.avatar.appendChild(img(b.avatar_url))
  else ui.head.avatar.appendChild(icon('ico', CHAT_SPARK))
  launcherFace(ui, b.avatar_url)
}

const HEX = /^#[0-9a-f]{6}$/i

/**
 * สีหลักจากคอนโซล — ทับ --accent ของธีมทั้งสว่าง/มืด · สีพื้นอ่อนและสีตัวอักษรบนสีหลักคำนวณให้เอง
 * (สีไม่ถูกรูปแบบ = ใช้สีตั้งต้น — ค่านี้เข้า style จึงรับเฉพาะ #rrggbb)
 */
export function applyAccent(host: HTMLElement, color: string | undefined, theme: 'light' | 'dark') {
  const props = ['--accent', '--accent-soft', '--on-accent']
  if (!color || !HEX.test(color)) {
    for (const p of props) host.style.removeProperty(p)
    return
  }
  const rgb = [1, 3, 5].map((i) => parseInt(color.slice(i, i + 2), 16))
  // พื้นอ่อน = ผสมสีหลักกับพื้นของธีม (สว่าง: ขาว · มืด: สีพื้นแชท)
  const base = theme === 'dark' ? [0x15, 0x21, 0x23] : [0xff, 0xff, 0xff]
  const mix = (w: number) => '#' + rgb.map((c, i) => Math.round(c * w + base[i] * (1 - w)).toString(16).padStart(2, '0')).join('')
  // ความสว่างตาม WCAG — สีหลักสว่างใช้ตัวอักษรเข้ม ไม่งั้นใช้ขาว
  const lum = rgb
    .map((c) => c / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    .reduce((a, c, i) => a + c * [0.2126, 0.7152, 0.0722][i], 0)
  host.style.setProperty('--accent', color)
  host.style.setProperty('--accent-soft', mix(theme === 'dark' ? 0.25 : 0.16))
  host.style.setProperty('--on-accent', lum > 0.45 ? '#13282B' : '#ffffff')
}

export function scroll(log: HTMLElement) {
  log.scrollTop = log.scrollHeight
}

function img(src: string): HTMLImageElement {
  const i = document.createElement('img')
  i.src = src
  i.alt = ''
  i.referrerPolicy = 'no-referrer'
  return i
}

function safeClass(s: string) {
  return String(s || '').replace(/[^a-z_]/g, '')
}

function px(n: number) {
  return `${n | 0}px`
}

export function el(tag: string, cls?: string): HTMLElement {
  const e = document.createElement(tag)
  if (cls) e.className = cls
  return e
}
