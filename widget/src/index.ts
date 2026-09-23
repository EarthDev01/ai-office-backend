import { createShadow, injectFonts } from './shadow'
import { buildUI, addBubble, addNote, applyPlacement, applyAppearance, type UI } from './ui'
import { PREVIEW_ALLOWED_FIELDS, type Bootstrap, type PreviewConfig } from './types'

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
}

interface State {
  ui: UI
  host: HTMLElement
  cfg: Bootstrap
  preview: boolean
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
  const publicKey = ds.publicKey ?? ''

  let cfg: Bootstrap
  if (preview) {
    // โหมด preview ไม่ยิง API เลย — รอ config จากหน้า settings
    cfg = { ...DEFAULT_BOOTSTRAP, ...(opts.bootstrap ?? {}) }
  } else if (opts.bootstrap) {
    cfg = { ...DEFAULT_BOOTSTRAP, ...opts.bootstrap }
  } else {
    const fetched = await fetchBootstrap(apiBase, publicKey, opts.onFetch)
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
  state = { ui, host, cfg, preview }

  render(cfg)

  ui.launcher.addEventListener('click', () => toggle())
  ui.send.addEventListener('click', () => send(opts))
  ui.input.addEventListener('keydown', (e) => {
    if ((e as KeyboardEvent).key === 'Enter') send(opts)
  })

  if (preview) {
    ui.panel.dataset.open = 'true'
    // ██ ผูก listener เฉพาะโหมด preview เท่านั้น
    // ██ ถ้าผูกในโหมดปกติ สคริปต์อื่นในหน้า office จะสั่งเปลี่ยนหน้าตา/ถ้อยคำของ AI ได้
    window.addEventListener('message', onPreviewMessage)
  }

  installGlobal()
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
  addNote(state.ui.log, 'ยังต่อกับข้อมูลจริงไม่ได้ — รอบนี้ทดสอบการติดตั้งและการตั้งค่าเท่านั้น')
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

async function fetchBootstrap(
  apiBase: string,
  publicKey: string,
  onFetch?: MountOptions['onFetch'],
): Promise<Bootstrap | null> {
  if (!publicKey) {
    explain('ไม่พบ public key ใน URL ของ script', 'snippet ต้องเป็น .../widget/v1/<public_key>/ai-office.js')
    return null
  }
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

  const url =
    `${apiBase}/api/ai/office/${encodeURIComponent(publicKey)}` +
    `/service/${encodeURIComponent(serviceID)}/bootstrap`
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
    explain(`เรียก ${url} ไม่สำเร็จ — ${(e as Error).message}`, 'backend ทำงานอยู่ไหม และโดเมนนี้อยู่ใน allowed_origins หรือยัง')
    return null
  }
}

/** คำอธิบายสาเหตุให้คนติดตั้งอ่าน — ไม่ทำให้หน้า office พัง แค่บอกใน console */
function explain(reason: string, hint?: string) {
  console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${reason}` + (hint ? `\n           → ${hint}` : ''))
}

const HINTS: Record<string, string> = {
  ORIGIN_NOT_ALLOWED: 'เพิ่มโดเมนของหน้านี้ลงใน "โดเมนที่อนุญาต" ของ office ที่คอนโซล',
  SERVICE_NOT_ALLOWED: 'บัญชีนี้ไม่มี service นี้ใน Role.ListService ของหลังบ้าน',
  NOT_FOUND: 'public key ใน snippet ไม่ตรงกับ office ไหนเลย — ถูกลบหรือ rotate key ไปแล้วหรือเปล่า',
  SESSION_EXPIRED: 'token หมดอายุ ให้ล็อกอินหลังบ้านใหม่',
  NOT_AUTHENTICATED: 'ไม่ได้ส่ง token ไป หรือ token ใช้ไม่ได้',
  BACKOFFICE_UNAVAILABLE: 'ตรวจสอบผู้ใช้กับ office-api ไม่ได้ — ตรวจ backoffice_api_url ของ office',
  office_disabled: 'office นี้ถูกปิดทั้งชุดที่คอนโซล',
  service_disabled: 'service นี้ยังไม่ได้เปิด หรือยังไม่มีใน office นี้',
  not_in_allowlist: 'เพิ่ม username ของบัญชีนี้ลง allowlist ของ service ที่คอนโซล',
  wrong_office: 'token เป็นของ office อื่น ไม่ตรงกับ key ใน snippet',
  no_service: 'ยังไม่ได้เลือกเว็บในหลังบ้าน',
}

function toggle(open?: boolean) {
  if (!state) return
  const next = open ?? state.ui.panel.dataset.open !== 'true'
  state.ui.panel.dataset.open = String(next)
  if (next) state.ui.input.focus()
}

function send(opts: MountOptions) {
  if (!state) return
  const text = state.ui.input.value.trim()
  if (!text) return
  state.ui.input.value = ''
  addBubble(state.ui.log, 'me', text)

  // ██ รอบนี้ยังไม่ต่อ LLM — /api/ai/chat จะมาใน Phase 3
  opts.onFetch?.({ url: '(chat ยังไม่เปิดใช้ในรอบนี้)', body: { text } })
  addBubble(
    state.ui.log,
    'ai',
    'ตอนนี้ผมยังตอบคำถามไม่ได้ครับ — รอบนี้ติดตั้งและตั้งค่าได้แล้ว ส่วนการตอบจากข้อมูลจริงจะมาในรอบถัดไป',
  )
}

function resolveTheme(mode: string): 'light' | 'dark' {
  if (mode === 'light' || mode === 'dark') return mode
  return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ? 'dark' : 'light'
}

function kebab(s: string) {
  return s.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase())
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
  }
  Object.defineProperty(window, '__aiOffice', { value: Object.freeze(api), configurable: true })
}

// ---- auto-boot เมื่อถูกโหลดเป็น <script> จริง ----
/**
 * แกะ public key จาก URL ของ script ตัวเอง
 * .../widget/v1/<public_key>/ai-office.js
 */
export function keyFromScriptURL(src: string): string {
  const parts = new URL(src, location.href).pathname.split('/').filter(Boolean)
  const i = parts.lastIndexOf('ai-office.js')
  return i > 0 ? decodeURIComponent(parts[i - 1]) : ''
}

const self = document.currentScript as HTMLScriptElement | null
if (self) {
  const ds: Record<string, string> = {}
  for (const k in self.dataset) ds[k] = self.dataset[k] as string
  if (self.src) {
    if (!ds.apiBase) ds.apiBase = new URL(self.src, location.href).origin
    if (!ds.publicKey) ds.publicKey = keyFromScriptURL(self.src)
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
    sync()
    setInterval(sync, 800)
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot)
  else boot()
}
