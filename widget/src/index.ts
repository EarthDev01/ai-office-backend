import { createShadow, injectFonts } from './shadow'
import { buildUI, addBubble, addLoader, addSys, applyPlacement, applyAppearance, el, renderCard, scroll, type UI } from './ui'
import { PREVIEW_ALLOWED_FIELDS, type Bootstrap, type PreviewConfig } from './types'
import { ChatSession } from './session'
import { runChat, runConsoleChat, ChatError, type ConsoleTurn } from './chat'

// data-* ที่ห้ามมาจากหน้าเว็บ — service ที่เปิดอยู่เราอ่านจาก localStorage ของ office เอง
// (data-public-key ไม่อยู่ในนี้ เพราะมันระบุแค่ว่าหน้านี้เป็นของ office ไหน)
const TENANT_ATTRS = ['serviceId', 'websiteId', 'businessId', 'tenant', 'tenantId', 'officeId', 'apiKey']

// office-v10x เก็บของ 2 อย่างนี้ไว้ที่ localStorage (backoffice-api-survey.md §2.1, §3)
const TOKEN_KEY = 'auth_token'   // { value: <JWT>, expiration: <unix วินาที> }
const SERVICE_KEY = 'web-service' // service ที่แอดมินเลือกอยู่ตอนนี้

/** อ่าน Bearer token ของหน้า office — รูปแบบเดียวกับ helper.getItemWithExpireV2 */
export function readOfficeToken(): string {
  try {
    const raw = localStorage.getItem(TOKEN_KEY)
    if (!raw) return ''
    const parsed = JSON.parse(raw) as { value?: string; expiration?: number }
    if (!parsed?.value) return ''
    const now = Math.floor(Date.now() / 1000)
    if (typeof parsed.expiration === 'number' && parsed.expiration <= now) return ''
    return parsed.value
  } catch {
    return ''
  }
}

/** service ที่แอดมินกำลังเปิดอยู่ — เปลี่ยนได้ตลอดโดยไม่โหลดหน้าใหม่ จึงอ่านสดทุกครั้ง */
export function readOfficeService(): string {
  try {
    return localStorage.getItem(SERVICE_KEY) ?? ''
  } catch {
    return ''
  }
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
    const fetched = await fetchBootstrap(apiBase, opts.onFetch)
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
  const session = new ChatSession(apiBase, readOfficeToken)
  state = { ui, host, cfg, preview, apiBase, session, conversationID: '', busy: false, console: opts.console ?? null, history: [] }

  render(cfg)

  ui.launcher.addEventListener('click', () => toggle())
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
    ui.panel.dataset.open = 'true'
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
  state.host.setAttribute('data-theme', resolveTheme(cfg.theme))
  applyAppearance(state.ui, cfg)
  applyPlacement(state.ui, cfg)
  state.ui.launcher.style.display = cfg.is_hidden ? 'none' : 'grid'

  state.ui.log.textContent = ''
  if (cfg.greeting) addBubble(state.ui.log, 'ai', cfg.greeting)
  if (state.preview) addSys(state.ui.log, MESSAGES.preview)
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
// snippet จึงเหมือนกันทุกโดเมนของ officeลูกค้า
async function fetchBootstrap(apiBase: string, onFetch?: MountOptions['onFetch']): Promise<Bootstrap | null> {
  const token = readOfficeToken()
  const serviceID = readOfficeService()

  // ยังไม่ล็อกอิน หรือยังไม่ได้เลือกเว็บ → ไม่ต้องยิง ไม่ต้องโผล่
  //
  // แต่ต้องบอกสาเหตุออกมา ไม่งั้นคนติดตั้งจะไม่มีทางรู้ว่าทำไมปุ่มไม่ขึ้น
  // (เงียบอย่างเดียวเคยทำให้เสียเวลาไล่หาสาเหตุมาแล้ว)
  if (!token || !serviceID) {
    explain(
      !token && !serviceID
        ? 'ไม่พบ localStorage["auth_token"] และ localStorage["web-service"] — หน้านี้ยังไม่ได้ล็อกอินหลังบ้าน'
        : !token
          ? 'ไม่พบ localStorage["auth_token"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอิน หรือ token หมดอายุแล้ว'
          : 'ไม่พบ localStorage["web-service"] — ยังไม่ได้เลือกเว็บในหลังบ้าน',
    )
    return null
  }

  const url = `${apiBase}/api/ai/widget/service/${encodeURIComponent(serviceID)}/bootstrap`
  onFetch?.({ url })

  try {
    // ไม่ใช้ cookie — หน้า office ส่ง Bearer token เหมือนที่ตัวมันเองเรียก API
    const res = await fetch(url, { headers: { Authorization: `Bearer ${token}` } })
    const json = (await res.json().catch(() => null)) as
      | { message?: string; error?: string; payload?: Bootstrap }
      | null

    if (!res.ok) {
      explain(`เซิร์ฟเวอร์ตอบ ${res.status} ${json?.message ?? ''} — ${json?.error ?? ''}`.trim(), HINTS[json?.message ?? ''])
      return null
    }
    const payload = json?.payload ?? null
    if (payload && !payload.enabled) {
      explain(`ยังไม่เปิดใช้งาน (reason: ${payload.reason})`, HINTS[payload.reason ?? ''])
    }
    return payload
  } catch (e) {
    // widget พังต้องไม่ลากหน้า office พังไปด้วย
    explain(`เรียก ${url} ไม่สำเร็จ — ${(e as Error).message}`, 'หลังบ้าน ai ทำงานอยู่ไหม')
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
  SERVICE_NOT_ALLOWED: 'บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน',
  SESSION_EXPIRED: 'token หมดอายุ ให้ล็อกอินหลังบ้านใหม่',
  NOT_AUTHENTICATED: 'ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้',
  BACKOFFICE_UNAVAILABLE: 'ตรวจสอบผู้ใช้กับ officeลูกค้า ไม่ได้ชั่วคราว',
  office_disabled: 'office นี้ถูกปิดทั้งชุดที่คอนโซล',
  service_disabled: 'service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้',
  not_in_allowlist: 'เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล',
  wrong_office: 'token เป็นของ office อื่น ไม่ตรงกับโดเมนของหน้านี้',
  no_service: 'ยังไม่ได้เลือกเว็บในหลังบ้าน',
}

function toggle(open?: boolean) {
  if (!state) return
  const next = open ?? state.ui.panel.dataset.open !== 'true'
  state.ui.panel.dataset.open = String(next)
  if (next) state.ui.input.focus()
}

/** ข้อความที่ผู้ใช้เห็นเมื่อเรียกผู้ช่วยไม่สำเร็จ และ server ไม่ได้ส่งข้อความมาเอง */
const MESSAGES = {
  preview: 'โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง',
  unavailable: 'ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง',
  empty: '(ไม่มีคำตอบ)',
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

  const service = readOfficeService()
  opts.onFetch?.({ url: `${s.apiBase}/api/ai/widget/service/${encodeURIComponent(service)}/chat`, body: { text } })

  s.busy = true
  s.ui.send.disabled = true
  const loader = addLoader(s.ui.log, 'กำลังส่งคำถาม…')
  // ฟองคำตอบสร้างเมื่อมีของให้โชว์ครั้งแรก — การ์ดอยู่ในฟองเดียวกับข้อความของ AI
  let txt: HTMLElement | null = null
  let cards: HTMLElement | null = null
  const ensure = () => {
    if (txt) return
    const bubble = addBubble(s.ui.log, 'ai', '')
    txt = bubble.querySelector('.txt')
    cards = el('div', 'cards')
    bubble.appendChild(cards)
  }
  try {
    s.conversationID = await runChat({
      apiBase: s.apiBase,
      service,
      session: s.session,
      readToken: readOfficeToken,
      conversationID: s.conversationID,
      text,
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
          txt!.textContent = (txt!.textContent ?? '') + t
          scroll(s.ui.log)
        },
      },
    })
    if (!txt) addBubble(s.ui.log, 'ai', MESSAGES.empty)
  } catch (e) {
    const msg = e instanceof ChatError ? e.message : MESSAGES.unavailable
    addBubble(s.ui.log, 'ai', msg, 'err')
  } finally {
    loader.remove()
    s.busy = false
    s.ui.send.disabled = false
  }
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
  // ██ office-v10x เป็น SPA — ล็อกอินแล้ว set localStorage โดยไม่ reload หน้า
  // ██ widget จึงต้องเฝ้า session เอง ไม่งั้นปุ่มจะไม่โผล่จนกว่าจะ refresh มือ
  // ██ storage event ไม่ยิงใน tab เดียวกัน จึง poll เบา ๆ
  //
  // sig = ลายเซ็น session: '' = ยังไม่ล็อกอิน · ไม่งั้น = service ที่เปิดอยู่
  // (หรือ '\0' ถ้าล็อกอินแล้วแต่ยังไม่เลือก service)
  let lastSig: string | null = null
  const sessionSig = () => (readOfficeToken() ? readOfficeService() || '\0' : '')
  const sync = () => {
    try {
      const sig = sessionSig()
      if (sig === lastSig) return
      lastSig = sig
      if (sig === '') unmount() // ออกจากระบบ → เก็บปุ่มทันที ไม่ต้อง refresh
      else void mount({ dataset: ds }) // ล็อกอิน / สลับ service → (re)mount
    } catch {
      /* localStorage เข้าไม่ได้ — ปล่อยผ่าน */
    }
  }
  const boot = () => {
    // โหมดคอนโซล — รอหน้าคอนโซลสั่ง mount พร้อม token (ไม่เฝ้า session ของหลังบ้านลูกค้า)
    if (ds.consoleMode !== undefined) {
      installConsoleGlobal(ds.apiBase ?? '')
      return
    }
    // โหมด preview (หน้า console) ไม่มี session ของ office — mount ครั้งเดียวเลย
    // ไม่ต้องเฝ้า session (ตัวเฝ้าไว้สำหรับ widget จริงที่ฝังในหน้า office-v10x เท่านั้น)
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
