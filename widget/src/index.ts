import { createShadow, injectFonts } from './shadow'
import { buildUI, addBubble, addLoader, addSys, applyPlacement, applyAppearance, applyAccent, readSiteColors, el, renderActionButton, renderCard, renderSuggestion, roomItem, formatFetched, scroll, setOpen, type UI } from './ui'
import { PREVIEW_ALLOWED_FIELDS, type Bootstrap, type ChatAction, type ChatSuggestion, type PreviewConfig } from './types'
import { ChatSession, PageConfigError } from './session'
import { describeSource, readService, type PageAction } from './page'
import { canRun, runPageAction } from './actions'
import { runChat, runConsoleChat, listConversations, loadConversation, ChatError, DISABLED_CODES, type ConsoleTurn, type HistoryMessage } from './chat'

// data-* ที่ห้ามมาจากหน้าเว็บ — service ที่เปิดอยู่เราอ่านจาก localStorage ของ office เอง
// (data-public-key ไม่อยู่ในนี้ เพราะมันระบุแค่ว่าหน้านี้เป็นของ office ไหน)
const TENANT_ATTRS = ['serviceId', 'websiteId', 'businessId', 'tenant', 'tenantId', 'officeId', 'apiKey']

// ChatSession ต่อ API ของ AI (1 หน้า = 1 ตัว) — page-config โหลดครั้งเดียวแล้วใช้ร่วมกันทุกครั้งที่ mount ใหม่
const sessions = new Map<string, ChatSession>()

/** test เท่านั้น — ล้าง page-config/ตั๋วที่จำไว้ระหว่างเคส */
export function __resetSessions() {
  sessions.clear()
}

function sessionFor(apiBase: string, hostAPIBase = ''): ChatSession {
  const k = apiBase + '|' + hostAPIBase
  let s = sessions.get(k)
  if (!s) {
    s = new ChatSession(apiBase, hostAPIBase)
    sessions.set(k, s)
  }
  return s
}

const DEFAULT_BOOTSTRAP: Bootstrap = {
  enabled: true,
  office_id: '',
  service_id: '',
  service_label: '',
  is_hidden: false,
  avatar_url: '',
  display_name: 'ผู้ช่วยหลังบ้าน',
  greeting: 'สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ',
  theme: 'auto',
  placement: { position: 'bottom-right', offset_x: 12, offset_y: 12 },
}

export interface MountOptions {
  /** ข้าม fetch แล้วใช้ค่านี้แทน — ใช้ใน test */
  bootstrap?: Partial<Bootstrap>
  /** จำลอง data-* attribute ของ script tag — ใช้ใน test */
  dataset?: Record<string, string>
  /** ดักทุกครั้งที่จะยิง API — ใช้ใน test */
  onFetch?: (info: { url: string; body?: unknown }) => void
  apiBase?: string
  /** โหมดคอนโซล AI Office — ถามผู้ช่วยของคอนโซลด้วย token ของคอนโซล (ไม่ผูกกับหลังบ้านลูกค้า) */
  console?: ConsoleOptions
}

export interface ConsoleOptions {
  /** token ของคอนโซลที่ใช้อยู่ — อ่านสดทุกครั้งที่ส่ง (ต่ออายุได้ระหว่างเปิดหน้า) */
  getToken: () => string
  /** path ของหน้าคอนโซลที่เปิดอยู่ ให้ผู้ช่วยตอบตรงบริบท */
  getPage?: () => string
  displayName?: string
  greeting?: string
  label?: string
}

// ห้องคุยของโหมดคอนโซลเก็บในหน้าเว็บเท่านั้น — จำกัดจำนวนที่ส่งกลับไป
const CONSOLE_HISTORY = 20

interface State {
  ui: UI
  host: HTMLElement
  cfg: Bootstrap
  preview: boolean
  apiBase: string
  session: ChatSession
  conversationID: string // ห้องแชทฝั่ง server — ว่าง = ข้อความถัดไปเปิดห้องใหม่
  busy: boolean
  console: ConsoleOptions | null
  history: ConsoleTurn[]
  /** โชว์ป้ายรหัสเว็บบนหัวแชท — หน้าเว็บผู้เล่นไม่โชว์ */
  showSite: boolean
  /** ตัวแปร CSS ของหน้าเว็บที่เก็บสีของแบรนด์ (จาก page-config) */
  pageColors?: { accent: string; accent_2?: string; on_accent?: string }
  /** ยกเลิกคำตอบที่กำลังส่ง — unmount (สลับเว็บ/ออกจากระบบ) เรียกเสมอ (B-15) */
  abort: AbortController | null
  /** กำลังโชว์รายการห้องย้อนหลังแทนห้องคุย */
  listing: boolean
  /** ปุ่มใต้คำตอบที่หน้านี้ทำได้ (page-config) — server ส่งมาแค่ id */
  pageActions: PageAction[]
  /** หน้าเว็บผู้เล่น: การ์ดเป็นข้อความในฟองแชท ไม่ใช่การ์ดแบบหลังบ้าน */
  inlineCards: boolean
}

let state: State | null = null

/**
 * ถอดของเดิมออกก่อน mount ใหม่
 *
 * จำเป็นเพราะหน้า settings จะ re-mount preview ทุกครั้งที่เปลี่ยน config
 * ถ้าไม่ถอด listener จะกองทับกัน และ listener ของ instance เก่า
 * จะไปแก้ instance ใหม่ได้ (รวมถึง instance ที่เป็นโหมดปกติ)
 */
export function unmount(): void {
  window.removeEventListener('message', onPreviewMessage)
  stopSiteColors()
  state?.abort?.abort()
  state?.host.remove()
  state = null
}

export async function mount(opts: MountOptions = {}): Promise<void> {
  unmount()
  const ds = opts.dataset ?? {}

  // เว็บและตัวตนตัดสินที่ server เท่านั้น — ถ้ามีใครใส่มาก็ไม่ส่งต่อ แต่บอกให้รู้
  for (const a of TENANT_ATTRS) {
    if (ds[a]) {
      console.warn(`[ai-office] ไม่รับ data-${kebab(a)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`)
      delete ds[a]
    }
  }

  const previewSelector = ds.previewMount ?? ''
  const preview = previewSelector !== ''
  const apiBase = opts.apiBase ?? ds.apiBase ?? ''
  const session = sessionFor(apiBase, ds.hostApiBase)

  let cfg: Bootstrap
  if (opts.console) {
    cfg = {
      ...DEFAULT_BOOTSTRAP,
      display_name: opts.console.displayName ?? 'ผู้ช่วย AI Office',
      greeting: opts.console.greeting ?? 'สวัสดีครับ ถามวิธีใช้คอนโซล หรือข้อมูลในระบบได้เลย (ผมเป็นระบบอัตโนมัติ อ่านข้อมูลได้อย่างเดียว)',
      service_label: opts.console.label ?? 'คอนโซล',
      placement: { position: 'bottom-right', offset_x: 20, offset_y: 20 },
    }
  } else if (preview) {
    // โหมด preview ไม่ยิง API เลย — รอ config จากหน้า settings
    cfg = { ...DEFAULT_BOOTSTRAP, ...(opts.bootstrap ?? {}) }
  } else if (opts.bootstrap) {
    cfg = { ...DEFAULT_BOOTSTRAP, ...opts.bootstrap }
  } else {
    const fetched = await fetchBootstrap(apiBase, session, opts.onFetch)
    if (!fetched) return
    cfg = { ...DEFAULT_BOOTSTRAP, ...fetched }
  }

  // ปิดอยู่ = ไม่สร้าง DOM อะไรเลย ไม่ใช่สร้างแล้วซ่อน
  if (!preview && !cfg.enabled) return

  const mountInto = preview ? document.querySelector(previewSelector) : null
  if (preview && !mountInto) {
    console.warn(`[ai-office] ไม่พบกล่อง preview: ${previewSelector}`)
    return
  }

  const { host, root } = createShadow(mountInto)
  if (preview) host.setAttribute('data-preview', 'true')
  injectFonts(root)

  const ui = buildUI(root)
  // page-config โหลดไว้แล้วตอน bootstrap (จำไว้ต่อหน้า) — preview/คอนโซล/test ที่ส่ง bootstrap มาเองไม่มี
  const pc = !preview && !opts.console && !opts.bootstrap ? await session.pageConfig().catch(() => null) : null
  state = {
    ui, host, cfg, preview, apiBase, session, conversationID: '', busy: false,
    console: opts.console ?? null, history: [], abort: null, listing: false,
    showSite: pc?.audience !== 'player',
    pageColors: pc?.page_colors,
    pageActions: pc?.page_actions ?? [],
    inlineCards: pc?.audience === 'player',
  }
  // ประวัติฝั่ง server มีเฉพาะแชทหลังบ้านจริง — คอนโซลเก็บห้องในหน้าเว็บ · preview ไม่ยิง API
  if (preview || opts.console) ui.head.history.style.display = 'none'

  render(cfg)

  ui.launcher.addEventListener('click', () => toggle())
  ui.head.history.addEventListener('click', () => void showRooms())
  ui.send.addEventListener('click', () => send(opts))
  ui.input.addEventListener('keydown', (e) => {
    const k = e as KeyboardEvent
    // Enter = ส่ง · Shift+Enter = ขึ้นบรรทัด · กำลังพิมพ์ภาษาไทยด้วย IME อย่าส่ง
    if (k.key === 'Enter' && !k.shiftKey && !k.isComposing) {
      k.preventDefault()
      void send(opts)
    }
  })

  if (preview) {
    setOpen(ui, true)
    // ██ ผูก listener เฉพาะโหมด preview เท่านั้น
    // ██ ถ้าผูกในโหมดปกติ สคริปต์อื่นในหน้า office จะสั่งเปลี่ยนหน้าตา/ถ้อยคำของ AI ได้
    window.addEventListener('message', onPreviewMessage)
  }

  // โหมดคอนโซลมี global ของตัวเอง (__aiOfficeConsole) — ไม่ทับ __aiOffice ของ preview ในหน้าเดียวกัน
  if (!opts.console) installGlobal()
}

function render(cfg: Bootstrap) {
  if (!state) return
  state.cfg = cfg
  const theme = resolveTheme(cfg.theme)
  state.host.setAttribute('data-theme', theme)
  const colorsReady = applyColors(theme)
  applyAppearance(state.ui, cfg, state.apiBase, state.showSite)
  applyPlacement(state.ui, cfg)
  state.ui.launcher.style.display = cfg.is_hidden ? 'none' : 'grid'

  state.ui.log.textContent = ''
  if (cfg.greeting) addBubble(state.ui.log, 'ai', cfg.greeting)
  if (state.preview) addSys(state.ui.log, MESSAGES.preview)
  if (!colorsReady) waitSiteColors()
}

function onPreviewMessage(ev: MessageEvent) {
  if (ev.origin !== window.location.origin) return
  const data = ev.data as { type?: string; config?: PreviewConfig }
  if (!data || data.type !== 'ai-office:preview-config' || !data.config) return
  // ตรวจซ้ำว่า instance ที่มีชีวิตอยู่ตอนนี้เป็น preview จริง
  // ไม่พึ่งแค่การถอด listener อย่างเดียว
  if (!state || !state.preview) return

  // รับได้เฉพาะ field หน้าตา — enabled/allowlist/quota/model เป็นเรื่องความปลอดภัย
  // และค่าใช้จ่าย ต้องมาจาก server เท่านั้น
  const safe: Record<string, unknown> = {}
  for (const k of PREVIEW_ALLOWED_FIELDS) {
    if (k in data.config) safe[k] = (data.config as Record<string, unknown>)[k]
  }
  render({ ...state.cfg, ...(safe as Partial<Bootstrap>) })
}

// ไม่ต้องส่งว่าเป็น office ไหน — server ดูจากโดเมนของหน้านี้ (header Origin ที่เบราว์เซอร์ใส่ให้เอง)
// snippet จึงเหมือนกันทุกโดเมนของ officeลูกค้า · วิธีอ่าน login ของหน้านี้มาจาก page-config (ตาม kind ของ office)
async function fetchBootstrap(apiBase: string, session: ChatSession, onFetch?: MountOptions['onFetch']): Promise<Bootstrap | null> {
  const pc = await loadPageConfig(session)
  if (!pc) return null
  // หน้าเว็บที่เปิด guest: ยังไม่ล็อกอินก็ได้ credential (guest_id) — ไม่ต้องรอล็อกอิน
  const token = session.credential()
  const serviceID = readService(pc.service)

  // ยังไม่ล็อกอิน หรือยังไม่ได้เลือกเว็บ → ไม่ต้องยิง ไม่ต้องโผล่
  //
  // แต่ต้องบอกสาเหตุออกมา ไม่งั้นคนติดตั้งจะไม่มีทางรู้ว่าทำไมปุ่มไม่ขึ้น
  // (เงียบอย่างเดียวเคยทำให้เสียเวลาไล่หาสาเหตุมาแล้ว)
  if (!token || !serviceID) {
    const t = describeSource(pc.token)
    const sv = describeSource(pc.service)
    explain(
      !token && !serviceID
        ? `ไม่พบ ${t} และ ${sv} — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน`
        : !token
          ? `ไม่พบ ${t} ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว`
          : `ไม่พบ ${sv} — ยังไม่ได้เลือกเว็บในหลังบ้าน (หน้านี้ไม่ได้ผูกกับเว็บใด)`,
    )
    return null
  }

  const url = `${apiBase}/api/ai/widget/service/${encodeURIComponent(serviceID)}/bootstrap`
  onFetch?.({ url })

  try {
    // ไม่ใช้ cookie — หน้า office ส่ง Bearer token เหมือนที่ตัวมันเองเรียก API (ผู้ไม่ล็อกอิน = Guest <guest_id>)
    const res = await fetch(url, { headers: { Authorization: token } })
    const json = (await res.json().catch(() => null)) as
      | { message?: string; error?: string; payload?: Bootstrap }
      | null

    if (!res.ok) {
      explain(`เซิร์ฟเวอร์ตอบ ${res.status} ${json?.message ?? ''} — ${json?.error ?? ''}`.trim(), HINTS[json?.message ?? ''])
      return null
    }
    const payload = json?.payload ?? null
    if (!payload) return null
    if (!payload.enabled) {
      explain(`ยังไม่เปิดใช้งาน (reason: ${payload.reason})`, HINTS[payload.reason ?? ''])
      return payload
    }
    // ขอตั๋วแชทไว้ก่อนโชว์ปุ่ม — ขอไม่ผ่าน = กดแล้วก็ถามไม่ได้ จึงไม่โชว์ (ตั๋วที่ได้ใช้ต่อตอนส่งคำถามแรก)
    try {
      await session.ticket(serviceID)
    } catch (e) {
      const code = e instanceof ChatError ? e.code : ''
      explain(`ขอตั๋วแชทไม่สำเร็จ (${code || (e as Error).message})`, HINTS[code])
      return null
    }
    return payload
  } catch (e) {
    // widget พังต้องไม่ลากหน้า office พังไปด้วย
    explain(`เรียก ${url} ไม่สำเร็จ — ${(e as Error).message}`, 'หลังบ้าน ai ทำงานอยู่ไหม')
    return null
  }
}

/** page-config ของหน้านี้ · ไม่สำเร็จ = บอกสาเหตุใน console แล้วคืน null */
async function loadPageConfig(session: ChatSession) {
  try {
    return await session.pageConfig()
  } catch (e) {
    const code = e instanceof PageConfigError ? e.message : ''
    explain(`โหลดการตั้งค่าของหน้านี้ไม่สำเร็จ (${code || (e as Error).message})`, HINTS[code] ?? 'หลังบ้าน ai ทำงานอยู่ไหม')
    return null
  }
}

/** คำอธิบายสาเหตุให้คนติดตั้งอ่าน — ไม่ทำให้หน้า office พัง แค่บอกใน console */
function explain(reason: string, hint?: string) {
  console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${reason}` + (hint ? `\n           → ${hint}` : ''))
}

const HINTS: Record<string, string> = {
  ORIGIN_NOT_REGISTERED: `โดเมน ${location.origin} ยังไม่ได้ลงทะเบียน — เพิ่มใน "URL ของ domain" ที่ officeai`,
  ORIGIN_REQUIRED: 'เบราว์เซอร์ไม่ได้ส่ง Origin มา — widget ต้องถูกเรียกจากหน้าเว็บของ officeลูกค้า',
  SERVICE_NOT_ALLOWED: 'บัญชีนี้ไม่มีสิทธิ์เปิดเว็บนี้ในหลังบ้าน (รายชื่อเว็บของบัญชี)',
  KIND_NOT_SUPPORTED: 'ชนิดหลังบ้านของ domain นี้ยังไม่มี connector — ตรวจ "ชนิดหลังบ้าน" ที่คอนโซล',
  SESSION_EXPIRED: 'token หมดอายุ ให้ล็อกอินหลังบ้านใหม่',
  NOT_AUTHENTICATED: 'ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้',
  BACKOFFICE_UNAVAILABLE: 'ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว',
  office_disabled: 'office นี้ถูกปิดทั้งชุดที่คอนโซล',
  service_disabled: 'service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้',
  not_in_allowlist: 'เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล',
  wrong_office: 'token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้',
  no_service: 'ยังไม่ได้เลือกเว็บในหลังบ้าน',
}

/** สี: ใช้สีของแบรนด์จากหน้าเว็บ (ถ้าตั้งไว้และอ่านได้) ไม่งั้นใช้สีที่ตั้งในคอนโซล */
/** คืน false = ตั้ง "ใช้สีของเว็บ" แต่หน้าเว็บยังไม่มีสีให้อ่าน (ใช้สีตั้งต้นไปก่อน) */
function applyColors(theme = resolveTheme(state?.cfg.theme ?? 'auto')): boolean {
  if (!state) return true
  const cfg = state.cfg
  const site = cfg.color_source === 'site' ? readSiteColors(state.pageColors) : null
  if (site) applyAccent(state.host, [site.accent, site.accent2 || site.accent], theme, site.on)
  else if (cfg.color_source === 'site') applyAccent(state.host, undefined, theme) // อ่านสีของเว็บไม่ได้ = สีตั้งต้นของ widget
  else applyAccent(state.host, cfg.accent_colors?.length ? cfg.accent_colors : cfg.accent_color, theme)
  return cfg.color_source !== 'site' || !!site
}

// หน้าเว็บมักตั้งสีของแบรนด์หลังโหลดค่าแบรนด์จาก API (หลัง widget โผล่) — คอยอ่านซ้ำจนได้
// ระหว่างรอซ่อนปุ่มไว้ไม่ให้เห็นสีตั้งต้นแวบ (สูงสุด SITE_COLOR_HIDE_MS แล้วโชว์ด้วยสีตั้งต้น · อ่านต่อถึง SITE_COLOR_GIVEUP_MS)
const SITE_COLOR_HIDE_MS = 4000
const SITE_COLOR_GIVEUP_MS = 20000
let siteColorTimer: ReturnType<typeof setInterval> | null = null

function waitSiteColors() {
  stopSiteColors()
  const s = state
  if (!s || s.preview) return
  const started = Date.now()
  s.ui.launcher.style.visibility = 'hidden'
  siteColorTimer = setInterval(() => {
    if (state !== s) return stopSiteColors()
    const done = applyColors()
    const waited = Date.now() - started
    if (done || waited > SITE_COLOR_HIDE_MS) s.ui.launcher.style.visibility = ''
    if (done || waited > SITE_COLOR_GIVEUP_MS) stopSiteColors()
  }, 250)
}

function stopSiteColors() {
  if (siteColorTimer) clearInterval(siteColorTimer)
  siteColorTimer = null
}

function toggle(open?: boolean) {
  if (!state) return
  const next = open ?? state.ui.panel.dataset.open !== 'true'
  // หน้าเว็บอาจตั้งสีของแบรนด์หลัง widget โผล่ (โหลดค่าแบรนด์ทีหลัง) — อ่านใหม่ทุกครั้งที่เปิด
  if (next) applyColors()
  setOpen(state.ui, next)
  if (next) state.ui.input.focus()
}

/** ข้อความที่ผู้ใช้เห็นเมื่อเรียกผู้ช่วยไม่สำเร็จ และ server ไม่ได้ส่งข้อความมาเอง */
const MESSAGES = {
  preview: 'โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง',
  unavailable: 'ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง',
  empty: '(ไม่มีคำตอบ)',
  rooms: 'ห้องแชทย้อนหลัง 7 วัน — แตะเพื่อเปิดคุยต่อ',
  noRooms: 'ยังไม่มีห้องแชทใน 7 วันนี้ · ห้องจากการล็อกอินครั้งก่อนเปิดจากที่นี่ไม่ได้ (เพื่อความปลอดภัย)',
}

async function send(opts: MountOptions) {
  const s = state
  if (!s || s.busy || s.ui.input.disabled) return
  const text = s.ui.input.value.trim()
  if (!text) return
  s.ui.input.value = ''
  addBubble(s.ui.log, 'me', text)

  // preview (หน้า console) ไม่มี session ของ office — ไม่ยิงแชทจริง
  if (s.preview) {
    addBubble(s.ui.log, 'ai', MESSAGES.preview)
    return
  }
  if (s.console) {
    await sendConsole(s, s.console, text)
    return
  }

  const service = await currentService(s)
  if (!service) {
    addBubble(s.ui.log, 'ai', MESSAGES.unavailable, 'err')
    return
  }
  if (s.listing) {
    // พิมพ์ถามระหว่างดูรายการห้อง = เริ่มห้องใหม่
    s.listing = false
    s.conversationID = ''
    render(s.cfg)
    addBubble(s.ui.log, 'me', text)
  }
  opts.onFetch?.({ url: `${s.apiBase}/api/ai/widget/service/${encodeURIComponent(service)}/chat`, body: { text } })

  s.busy = true
  s.ui.send.disabled = true
  s.ui.head.history.disabled = true
  const abort = (s.abort = new AbortController())
  const loader = addLoader(s.ui.log, 'กำลังส่งคำถาม…')
  // ฟองคำตอบสร้างเมื่อมีของให้โชว์ครั้งแรก — การ์ดอยู่ในฟองเดียวกับข้อความของ AI
  let txt: HTMLElement | null = null
  let cards: HTMLElement | null = null
  let acts: HTMLElement | null = null
  const ensure = () => {
    if (txt) return
    const bubble = addBubble(s.ui.log, 'ai', '')
    txt = bubble.querySelector('.txt')
    cards = el('div', 'cards')
    acts = el('div', 'acts')
    bubble.append(cards, acts)
  }
  try {
    s.conversationID = await runChat({
      apiBase: s.apiBase,
      service,
      session: s.session,
      conversationID: s.conversationID,
      text,
      signal: abort.signal,
      on: {
        status: (t) => loader.set(t || 'กำลังทำงาน…'),
        card: (card) => {
          ensure()
          cards!.appendChild(renderCard(card, { inline: s.inlineCards }))
          scroll(s.ui.log)
        },
        action: (a) => {
          ensure()
          addAction(s, acts!, a)
          scroll(s.ui.log)
        },
        suggest: (sg) => {
          ensure()
          addSuggestion(s, acts!.parentElement!, sg, opts)
          scroll(s.ui.log)
        },
        token: (t) => {
          loader.remove()
          ensure()
          txt!.textContent = (txt!.textContent ?? '') + t
          scroll(s.ui.log)
        },
      },
    })
    if (!txt) addBubble(s.ui.log, 'ai', MESSAGES.empty)
  } catch (e) {
    if (abort.signal.aborted) return // สลับเว็บ/ออกจากระบบกลางคำตอบ — ห้องนี้ถูกถอดไปแล้ว ไม่ต้องแจ้งอะไร
    const msg = e instanceof ChatError ? e.message : MESSAGES.unavailable
    addBubble(s.ui.log, 'ai', msg, 'err')
    const code = e instanceof ChatError ? e.code : ''
    // ห้องเดิมหาย (ถูกลบตามคำขอ ฯลฯ) → ข้อความถัดไปเปิดห้องใหม่
    if (code === 'not_found') s.conversationID = ''
    if (DISABLED_CODES.has(code)) disable(s)
  } finally {
    loader.remove()
    s.abort = null
    s.busy = false
    s.ui.send.disabled = s.ui.input.disabled
    s.ui.head.history.disabled = false
  }
}

async function currentService(s: State): Promise<string> {
  const pc = await loadPageConfig(s.session)
  return pc ? readService(pc.service) : ''
}

/** รายการห้องย้อนหลัง 7 วันของตัวเอง (เฉพาะห้องจากการล็อกอินหลังบ้านครั้งนี้ — KI-A1) */
async function showRooms() {
  const s = state
  if (!s || s.busy || s.preview || s.console) return
  const service = await currentService(s)
  if (!service) return
  s.busy = true
  s.ui.head.history.disabled = true
  const loader = addLoader(s.ui.log, 'กำลังโหลดประวัติ…')
  try {
    const rooms = await listConversations(s.apiBase, service, s.session, 7)
    if (state !== s) return
    s.listing = true
    s.ui.log.textContent = ''
    addSys(s.ui.log, MESSAGES.rooms)
    s.ui.log.appendChild(roomItem('＋ เริ่มห้องใหม่', '', () => newRoom(s)))
    for (const r of rooms) {
      s.ui.log.appendChild(roomItem(r.title || '(ไม่มีหัวข้อ)', 'คุยล่าสุด ' + formatFetched(r.updated_at), () => void openRoom(s, service, r.id)))
    }
    if (!rooms.length) addSys(s.ui.log, MESSAGES.noRooms)
    scroll(s.ui.log)
  } catch (e) {
    addBubble(s.ui.log, 'ai', e instanceof ChatError ? e.message : MESSAGES.unavailable, 'err')
  } finally {
    loader.remove()
    s.busy = false
    s.ui.head.history.disabled = false
  }
}

function newRoom(s: State) {
  s.listing = false
  s.conversationID = ''
  render(s.cfg)
  s.ui.input.focus()
}

async function openRoom(s: State, service: string, id: string) {
  if (s.busy) return
  s.busy = true
  try {
    const room = await loadConversation(s.apiBase, service, s.session, id)
    if (state !== s) return
    s.listing = false
    s.conversationID = room.id
    s.ui.log.textContent = ''
    addSys(s.ui.log, `ห้องเดิม: ${room.title} · ตัวเลขในการ์ดเป็นค่า ณ เวลาที่ดึง ถามใหม่เพื่อดูค่าล่าสุด`)
    for (const m of room.messages) replay(s, m)
    s.ui.input.focus()
  } catch (e) {
    addBubble(s.ui.log, 'ai', e instanceof ChatError ? e.message : MESSAGES.unavailable, 'err')
  } finally {
    s.busy = false
  }
}

/** วาดข้อความจากประวัติด้วยตัววาดชุดเดียวกับตอนคุยสด */
function replay(s: State, m: HistoryMessage) {
  if (m.role === 'user') {
    addBubble(s.ui.log, 'me', m.text)
    return
  }
  if (!m.text && !m.cards?.length) return
  const bubble = addBubble(s.ui.log, 'ai', m.text, m.status === 'error' ? 'err' : '')
  if (m.cards?.length) {
    const cards = el('div', 'cards')
    for (const c of m.cards) cards.appendChild(renderCard(c, { inline: s.inlineCards }))
    bubble.appendChild(cards)
  }
  if (m.actions?.length) {
    const acts = el('div', 'acts')
    for (const a of m.actions) addAction(s, acts, a)
    bubble.appendChild(acts)
  }
  for (const sg of m.suggestions ?? []) addSuggestion(s, bubble, sg, {})
}

/** ปุ่มถามต่อ — กดแล้วส่ง ask เป็นคำถามใหม่ (เหมือนผู้ใช้พิมพ์เอง) */
function addSuggestion(s: State, bubble: HTMLElement, sg: ChatSuggestion, opts: MountOptions) {
  let box = bubble.querySelector(':scope > .sugs') as HTMLElement | null
  if (!box) {
    box = el('div', 'sugs')
    bubble.appendChild(box)
  }
  box.appendChild(
    renderSuggestion(sg.label, () => {
      if (s.busy) return
      s.ui.input.value = sg.ask
      void send(opts)
    }),
  )
}

/**
 * ปุ่มใต้คำตอบ — server ส่งมาแค่ id/label · วิธีสั่งหน้าเว็บหาจาก page-config ของหน้านี้
 * ไม่รู้จัก id / หน้านี้ไม่มีปุ่มจริงให้กด = ไม่วาด
 */
function addAction(s: State, box: HTMLElement, a: ChatAction) {
  const spec = s.pageActions.find((p) => p.id === a.id)
  if (!spec || !canRun(spec)) return
  box.appendChild(
    renderActionButton(spec.label || a.label, () => {
      // ปิดกล่องแชทก่อน (จอมือถือแชทเต็มจอ บัง modal) แล้วค่อยสั่งหน้าเว็บ
      toggle(false)
      if (!runPageAction(spec)) {
        toggle(true)
        addSys(s.ui.log, `เปิด ${spec.label} ไม่ได้ ลองกดเมนูที่หน้าเว็บแทน`)
        scroll(s.ui.log)
      }
    }),
  )
}

/** ผู้ช่วยของเว็บนี้ใช้ต่อไม่ได้ (ถูกปิด/ไม่มีสิทธิ์) — เก็บปุ่ม ปิดช่องพิมพ์ ข้อความที่แจ้งไปแล้วยังอ่านได้จนปิดกล่อง */
function disable(s: State) {
  s.ui.input.disabled = true
  s.ui.send.disabled = true
  s.ui.launcher.style.display = 'none'
}

/** โหมดคอนโซล: ส่งทั้งห้องคุยไปที่ผู้ช่วยของคอนโซล — UI ชุดเดียวกับแชทปกติ */
async function sendConsole(s: State, c: ConsoleOptions, text: string) {
  s.history.push({ role: 'user', text })
  s.busy = true
  s.ui.send.disabled = true
  const loader = addLoader(s.ui.log, 'กำลังส่งคำถาม…')
  let txt: HTMLElement | null = null
  let cards: HTMLElement | null = null
  let answer = ''
  const ensure = () => {
    if (txt) return
    const bubble = addBubble(s.ui.log, 'ai', '')
    txt = bubble.querySelector('.txt')
    cards = el('div', 'cards')
    bubble.appendChild(cards)
  }
  try {
    await runConsoleChat({
      apiBase: s.apiBase,
      token: c.getToken(),
      messages: s.history.slice(-CONSOLE_HISTORY),
      page: c.getPage?.() ?? '',
      on: {
        status: (t) => loader.set(t || 'กำลังทำงาน…'),
        card: (card) => {
          ensure()
          cards!.appendChild(renderCard(card))
          scroll(s.ui.log)
        },
        token: (t) => {
          loader.remove()
          ensure()
          answer += t
          txt!.textContent = answer
          scroll(s.ui.log)
        },
      },
    })
    if (answer) s.history.push({ role: 'assistant', text: answer })
    else addBubble(s.ui.log, 'ai', MESSAGES.empty)
  } catch (e) {
    s.history.pop() // คำถามที่ตอบไม่สำเร็จ ไม่ส่งซ้ำในรอบหน้า
    const msg = e instanceof ChatError ? e.message : MESSAGES.unavailable
    addBubble(s.ui.log, 'ai', msg, 'err')
  } finally {
    loader.remove()
    s.busy = false
    s.ui.send.disabled = false
  }
}

function resolveTheme(mode: string): 'light' | 'dark' {
  if (mode === 'light' || mode === 'dark') return mode
  return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ? 'dark' : 'light'
}

function kebab(s: string) {
  return s.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase())
}

/** โหมดคอนโซล: หน้าคอนโซลเรียก __aiOfficeConsole.mount({...}) เองหลังโหลด script (ส่ง token ผ่าน callback ไม่ใส่ใน DOM) */
export function installConsoleGlobal(apiBase: string) {
  const api = {
    mount: (o: ConsoleOptions) => mount({ apiBase, console: o }),
    unmount,
    open: () => toggle(true),
    close: () => toggle(false),
    // ---- test hook ----
    __logText: () => state?.ui.log.textContent ?? '',
    __send: async (text: string) => {
      if (!state) return
      state.ui.input.value = text
      await send({})
    },
  }
  Object.defineProperty(window, '__aiOfficeConsole', { value: Object.freeze(api), configurable: true })
}

function installGlobal() {
  const api = {
    open: () => toggle(true),
    close: () => toggle(false),
    // ---- test hook ----
    __hasLauncher: () => !!state && state.ui.launcher.style.display !== 'none',
    __isOpen: () => !!state && state.ui.panel.dataset.open === 'true',
    __launcherStyle: () => (state ? state.ui.launcher.style : ({} as CSSStyleDeclaration)),
    __config: () => (state ? { ...state.cfg } : {}),
    __host: () => state?.host ?? null,
    __send: async (text: string) => {
      if (!state) return
      state.ui.input.value = text
      await send({})
    },
    __logText: () => state?.ui.log.textContent ?? '',
    __conversationID: () => state?.conversationID ?? '',
    __showRooms: () => showRooms(),
    __openRoom: async (id: string) => {
      if (!state) return
      const service = await currentService(state)
      await openRoom(state, service, id)
    },
  }
  Object.defineProperty(window, '__aiOffice', { value: Object.freeze(api), configurable: true })
}

// ---- auto-boot เมื่อถูกโหลดเป็น <script> จริง ----
const self = document.currentScript as HTMLScriptElement | null
if (self) {
  const ds: Record<string, string> = {}
  for (const k in self.dataset) ds[k] = self.dataset[k] as string
  if (self.src) {
    if (!ds.apiBase) ds.apiBase = new URL(self.src, location.href).origin
  }
  // ██ หลังบ้านส่วนใหญ่เป็น SPA — ล็อกอิน/สลับเว็บแล้วเปลี่ยน storage หรือ URL โดยไม่ reload หน้า
  // ██ widget จึงต้องเฝ้า session เอง ไม่งั้นปุ่มจะไม่โผล่จนกว่าจะ refresh มือ
  // ██ storage event ไม่ยิงใน tab เดียวกัน จึง poll เบา ๆ
  //
  // sig = ลายเซ็น session: '' = ยังไม่ล็อกอิน · ไม่งั้น = service ที่เปิดอยู่
  // (หรือ '\0' ถ้าล็อกอินแล้วแต่ยังไม่เลือก service)
  const session = sessionFor(ds.apiBase ?? '', ds.hostApiBase)
  let page: Awaited<ReturnType<typeof loadPageConfig>> = null
  let retryAt = 0
  let lastSig: string | null = null
  const sync = () => {
    try {
      // ต้องรู้ก่อนว่าหลังบ้านชนิดนี้เก็บ login ไว้ที่ไหน — โหลดไม่ได้ ลองใหม่ทุก 30 วิ
      if (!page) {
        if (Date.now() < retryAt) return
        retryAt = Date.now() + 30_000
        void loadPageConfig(session).then((pc) => {
          page = pc
          if (pc) sync()
        })
        return
      }
      // Bearer/Guest แยกกัน — ล็อกอินจาก guest ต้อง mount ใหม่ด้วยตัวตนจริง
      const cred = session.credential()
      const sig = cred ? cred.slice(0, cred.indexOf(' ')) + ':' + (readService(page.service) || '\0') : ''
      if (sig === lastSig) return
      lastSig = sig
      if (sig === '') unmount() // ออกจากระบบ → เก็บปุ่มทันที ไม่ต้อง refresh
      else void mount({ dataset: ds }) // ล็อกอิน / สลับ service → (re)mount
    } catch {
      /* storage เข้าไม่ได้ — ปล่อยผ่าน */
    }
  }
  const boot = () => {
    // โหมดคอนโซล — รอหน้าคอนโซลสั่ง mount พร้อม token (ไม่เฝ้า session ของหลังบ้านลูกค้า)
    if (ds.consoleMode !== undefined) {
      installConsoleGlobal(ds.apiBase ?? '')
      return
    }
    // โหมด preview (หน้า console) ไม่มี session ของ office — mount ครั้งเดียวเลย
    // ไม่ต้องเฝ้า session (ตัวเฝ้าไว้สำหรับ widget จริงที่ฝังในหน้าหลังบ้านเท่านั้น)
    if (ds.previewMount) {
      void mount({ dataset: ds })
      return
    }
    sync()
    setInterval(sync, 800)
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot)
  else boot()
}
