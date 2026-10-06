import type { Bootstrap, Card } from './types'

export interface UI {
  launcher: HTMLButtonElement
  panel: HTMLDivElement
  head: { avatar: HTMLDivElement; name: HTMLDivElement; tag: HTMLDivElement; site: HTMLDivElement }
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
  const tag = el('div', 'tag') as HTMLDivElement
  nameWrap.append(name, tag)
  const site = el('div', 'site') as HTMLDivElement
  site.title = 'เว็บที่กำลังคุยอยู่'
  const close = el('button', 'x') as HTMLButtonElement
  close.type = 'button'
  close.textContent = '×'
  close.setAttribute('aria-label', 'ปิด')
  head.append(avatar, nameWrap, site, close)

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

  const ui = { launcher, panel, head: { avatar, name, tag, site }, log, input, send }
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

/**
 * ปุ่มเปิดแชท = ฟองแชท 3D ลอย (รูปจากคลัง) ย้อมสีด้วยสีไล่ของ widget — แสง/เงาของรูปยังอยู่ จุดขาวยังขาว
 * (ชั้นสีใช้ mix-blend-mode: color ตัดตามรูปด้วย mask) · ไม่มีรูป = ไอคอนในตัวบนวงกลมสีหลักแบบเดิม
 */
function launcherFace(ui: UI, iconURL: string) {
  ui.launcher.textContent = ''
  const face = el('span', 'face')
  if (iconURL) {
    ui.launcher.dataset.icon = 'true'
    const wrap = el('span', 'licon')
    wrap.style.setProperty('--licon', `url("${iconURL}")`)
    wrap.append(img(iconURL), el('span', 'tint'))
    face.appendChild(wrap)
  } else {
    delete ui.launcher.dataset.icon
    face.appendChild(icon('ico', CHAT_SPARK))
  }
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

/**
 * หน้าตาตามที่ตั้งในคอนโซล · assetBase = ที่อยู่คลังรูปของ ai-office-backend (สำหรับค่า asset:<ไฟล์>)
 * showSite = โชว์ป้ายรหัสเว็บบนหัว (หลังบ้านต้องรู้ว่าคุยเรื่องเว็บไหน · หน้าเว็บผู้เล่นไม่ต้อง)
 */
export function applyAppearance(ui: UI, b: Bootstrap, assetBase = '', showSite = true) {
  ui.head.name.textContent = b.display_name || 'ผู้ช่วยหลังบ้าน'
  ui.head.tag.textContent = b.tagline ?? ''
  // ป้ายชื่อ service ค้างบนหัวตลอด — แอดมินต้องรู้ตลอดว่ากำลังคุยเรื่อง service ไหน
  ui.head.site.textContent = showSite ? b.service_label || b.service_id || '' : ''
  ui.head.site.style.display = ui.head.site.textContent ? '' : 'none'

  const avatar = lookURL(b.avatar_url, assetBase)
  ui.head.avatar.textContent = ''
  if (avatar) ui.head.avatar.appendChild(img(avatar))
  else ui.head.avatar.appendChild(icon('ico', CHAT_SPARK))
  // ปุ่มเปิดแชทเป็นฟองแชทเสมอ (รูปผู้ช่วยอยู่บนหัวแชท)
  launcherFace(ui, lookURL(b.launcher_icon, assetBase))

  // หัวแชทแบบไล่สี + รูปผู้ช่วยใหญ่ — เมื่อตั้งรูป/คำโปรย/พื้นหลังไว้ (ไม่ตั้งอะไร = หน้าตาเดิม)
  if (avatar || b.tagline || b.background) ui.panel.dataset.rich = 'true'
  else delete ui.panel.dataset.rich
  applyBackground(ui.log, b.background ?? '', assetBase)
  // พื้นหลังเป็นรูป (รูปในคลังเป็นโทนเข้ม) → ช่องพิมพ์และฟองของ AI เป็นกระจกเข้มให้กลืนกับรูป
  if (ui.log.dataset.bg === 'image') ui.panel.dataset.bgimg = 'true'
  else delete ui.panel.dataset.bgimg
}

// ค่าที่ลงไปเป็น CSS url(...) — ห้ามมีเครื่องหมายคำพูด วงเล็บ ช่องว่าง (server ตรวจแล้ว ตรวจซ้ำกันไว้)
const SAFE_URL = /^(https:\/\/|http:\/\/localhost(:\d+)?\/)[A-Za-z0-9._~:/?#[\]@!$&*+,;=%-]+$/
const PATTERNS = new Set(['dots', 'grid', 'diagonal', 'glow'])

/** asset:<ไฟล์> → URL ในคลังรูป · URL ตรง ๆ ต้องปลอดภัย · อย่างอื่น = '' */
export function lookURL(v: string | undefined, assetBase: string): string {
  if (!v) return ''
  if (v.startsWith('asset:')) {
    const name = v.slice(6)
    return /^[a-z0-9_-]+\/[A-Za-z0-9._-]+$/.test(name) ? `${assetBase}/widget/v1/assets/${name}` : ''
  }
  return SAFE_URL.test(v) ? v : ''
}

function applyBackground(log: HTMLElement, v: string, assetBase: string) {
  log.style.removeProperty('--log-img')
  delete log.dataset.bg
  if (v.startsWith('pattern:')) {
    const p = v.slice(8)
    if (PATTERNS.has(p)) log.dataset.bg = p
    return
  }
  const url = lookURL(v, assetBase)
  if (!url) return
  log.dataset.bg = 'image'
  log.style.setProperty('--log-img', `url("${url}")`)
}

const HEX = /^#[0-9a-f]{6}$/i

/**
 * สีหลักจากคอนโซล — ทับ --accent ของธีมทั้งสว่าง/มืด · สีพื้นอ่อนและสีตัวอักษรบนสีหลักคำนวณให้เอง
 * (สีไม่ถูกรูปแบบ = ใช้สีตั้งต้น — ค่านี้เข้า style จึงรับเฉพาะ #rrggbb)
 */
export function applyAccent(host: HTMLElement, colors: string | string[] | undefined, theme: 'light' | 'dark', onAccent = '') {
  const props = ['--accent', '--accent-stops', '--accent-soft', '--on-accent']
  const stops = (Array.isArray(colors) ? colors : colors ? [colors] : []).filter((c) => HEX.test(c)).slice(0, 4)
  if (!stops.length) {
    for (const p of props) host.style.removeProperty(p)
    return
  }
  const color = stops[0]
  if (stops.length === 1) stops.push(color)
  // ตัวอักษรบนสีไล่ต้องอ่านออกทุกช่วง — คิดความสว่างจากสีเฉลี่ยของทุกสี
  const rgbs = stops.map((c) => [1, 3, 5].map((i) => parseInt(c.slice(i, i + 2), 16)))
  const rgb = rgbs[0]
  const mid = [0, 1, 2].map((k) => rgbs.reduce((a, c) => a + c[k], 0) / rgbs.length)
  // พื้นอ่อน = ผสมสีหลักกับพื้นของธีม (สว่าง: ขาว · มืด: สีพื้นแชท)
  const base = theme === 'dark' ? [0x15, 0x21, 0x23] : [0xff, 0xff, 0xff]
  const mix = (w: number) => '#' + rgb.map((c, i) => Math.round(c * w + base[i] * (1 - w)).toString(16).padStart(2, '0')).join('')
  // ความสว่างตาม WCAG — สีหลักสว่างใช้ตัวอักษรเข้ม ไม่งั้นใช้ขาว
  const lum = mid
    .map((c) => c / 255)
    .map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    .reduce((a, c, i) => a + c * [0.2126, 0.7152, 0.0722][i], 0)
  host.style.setProperty('--accent', color)
  host.style.setProperty('--accent-stops', stops.join(', '))
  host.style.setProperty('--accent-soft', mix(theme === 'dark' ? 0.25 : 0.16))
  host.style.setProperty('--on-accent', HEX.test(onAccent) ? onAccent : lum > 0.45 ? '#13282B' : '#ffffff')
}

/**
 * สีของแบรนด์จากหน้าเว็บ (ตัวแปร CSS บน :root ที่ธีมตั้งจากค่าของแบรนด์) → #rrggbb
 * ค่าที่อ่านได้ผ่านการแปลงเป็นสีจริงก่อน (ให้เบราว์เซอร์ตีความ) — ค่าแปลก ๆ จะไม่หลุดเข้า style
 */
export function readSiteColors(vars: { accent: string; accent_2?: string; on_accent?: string } | undefined) {
  if (!vars) return null
  const root = getComputedStyle(document.documentElement)
  const probe = document.createElement('span')
  probe.style.display = 'none'
  document.body.appendChild(probe)
  const toHex = (name?: string): string => {
    if (!name || !/^--[A-Za-z0-9_-]+$/.test(name)) return ''
    const raw = root.getPropertyValue(name).trim()
    if (!raw) return ''
    probe.style.color = ''
    probe.style.color = raw
    if (!probe.style.color) return ''
    const m = /rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?/.exec(getComputedStyle(probe).color)
    if (!m || (m[4] !== undefined && Number(m[4]) === 0)) return ''
    return '#' + [m[1], m[2], m[3]].map((n) => Number(n).toString(16).padStart(2, '0')).join('')
  }
  const out = { accent: toHex(vars.accent), accent2: toHex(vars.accent_2), on: toHex(vars.on_accent) }
  probe.remove()
  return out.accent ? out : null
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
