import { Api } from './api'
import { Widget } from './chat'
import { readerFor, type HostReader } from './pageauth'
import { makeHostFetcher, type HostFetcher } from './hostfetch'
import { SessionManager, type TicketResult } from './session'
import { createShadow, injectFonts } from './shadow'
import { PREVIEW_ALLOWED_FIELDS, type Bootstrap, type PageAuth, type PreviewConfig } from './types'
import { buildUI } from './ui'

// ตัวอ่านเฉพาะ kind (ถ้ามี) — ไฟล์ใน hosts/*.ts เรียก registerHostReader เอง
// ส่วนใหญ่ไม่ต้องมี: host.yaml บอกชื่อช่อง/รูปแบบได้ครบ
import.meta.glob('./hosts/*.ts', { eager: true })

export { registerHostReader } from './pageauth'

// data-* ที่ห้ามมาจากหน้าเว็บ — service ที่เปิดอยู่อ่านจากหน้าหลังบ้านตาม page_auth เท่านั้น
// (data-public-key ไม่อยู่ในนี้ เพราะมันระบุแค่ว่าหน้านี้เป็นของ office ไหน)
const TENANT_ATTRS = ['serviceId', 'websiteId', 'businessId', 'tenant', 'tenantId', 'officeId', 'apiKey', 'secret', 'secretKey', 'ticket']

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
  /** ข้าม page-config/session/bootstrap แล้วใช้ค่านี้แทน — ใช้ใน test */
  bootstrap?: Partial<Bootstrap>
  /** ตั๋วที่มีอยู่แล้ว (คู่กับ bootstrap) — ใช้ใน test แชท */
  session?: { service: string; ticket: string; expires_at?: number }
  /** จำลอง data-* attribute ของ script tag */
  dataset?: Record<string, string>
  apiBase?: string
}

let widget: Widget | null = null
let controller: Controller | null = null

/**
 * ถอดของเดิมออกก่อน mount ใหม่
 *
 * หน้า settings re-mount preview ทุกครั้งที่เปลี่ยน config — ถ้าไม่ถอด listener จะกองทับกัน
 * และ listener ของ instance เก่าจะไปแก้ instance ใหม่ได้ (รวมถึง instance โหมดปกติ)
 */
export function unmount(): void {
  window.removeEventListener('message', onPreviewMessage)
  widget?.destroy()
  widget = null
}

/** หยุดทุกอย่าง (รวมตัวเฝ้า session) — ใช้ใน test */
export function shutdown(): void {
  controller?.stop()
  controller = null
  unmount()
}

function sanitize(ds: Record<string, string>): Record<string, string> {
  const out = { ...ds }
  // เว็บและตัวตนตัดสินที่ host + server เท่านั้น — ถ้ามีใครใส่มาก็ไม่ส่งต่อ แต่บอกให้รู้
  for (const a of TENANT_ATTRS) {
    if (out[a]) {
      console.warn(`[ai-office] ไม่รับ data-${kebab(a)} — เว็บและสิทธิ์ตัดสินที่เซิร์ฟเวอร์เท่านั้น`)
      delete out[a]
    }
  }
  return out
}

export async function mount(opts: MountOptions = {}): Promise<void> {
  shutdown()
  const ds = sanitize(opts.dataset ?? {})
  const apiBase = opts.apiBase ?? ds.apiBase ?? ''
  const publicKey = ds.publicKey ?? ''

  if (ds.previewMount) {
    // โหมด preview ไม่ยิง API เลย — รอ config จากหน้า settings
    const cfg = { ...DEFAULT_BOOTSTRAP, ...(opts.bootstrap ?? {}) }
    const box = document.querySelector(ds.previewMount)
    if (!box) {
      console.warn(`[ai-office] ไม่พบกล่อง preview: ${ds.previewMount}`)
      return
    }
    create(cfg, { api: null, session: null }, box)
    widget!.ui.panel.dataset.open = 'true'
    // ██ ผูก listener เฉพาะโหมด preview เท่านั้น
    // ██ ถ้าผูกในโหมดปกติ สคริปต์อื่นในหน้า office จะสั่งเปลี่ยนหน้าตา/ถ้อยคำของ AI ได้
    window.addEventListener('message', onPreviewMessage)
    return
  }

  if (opts.bootstrap) {
    const cfg = { ...DEFAULT_BOOTSTRAP, ...opts.bootstrap }
    if (!cfg.enabled) return
    const api = new Api(apiBase, publicKey)
    let session: SessionManager | null = null
    if (opts.session) {
      session = new SessionManager(api, opts.session.service, null, null)
      session.seed(opts.session.ticket, opts.session.expires_at ?? Math.floor(Date.now() / 1000) + 600)
    }
    create(cfg, { api, session }, null)
    return
  }

  // ทางจริงแบบครั้งเดียว (ไม่เฝ้า session) — auto-boot ใช้ boot() ที่เฝ้าด้วย
  controller = new Controller(ds, apiBase, publicKey)
  await controller.once()
}

/** auto-boot: เฝ้า session ของหน้าหลังบ้าน (SPA ล็อกอิน/สลับเว็บโดยไม่ reload) */
export function boot(dataset: Record<string, string>, pollMs = 800): void {
  shutdown()
  const ds = sanitize(dataset)
  if (ds.previewMount) {
    void mount({ dataset: ds })
    return
  }
  controller = new Controller(ds, ds.apiBase ?? '', ds.publicKey ?? '')
  controller.start(pollMs)
}

function create(cfg: Bootstrap, deps: ConstructorParameters<typeof Widget>[4], mountInto: Element | null) {
  const { host, root } = createShadow(mountInto)
  if (mountInto) host.setAttribute('data-preview', 'true')
  injectFonts(root)
  const ui = buildUI(root)
  widget = new Widget(ui, host, cfg, !!mountInto, deps)
  widget.paint(cfg)
  widget.reset()
  installGlobal()
}

const RETRY_MS = 30_000

/**
 * ขั้นตอนเปิดผู้ช่วย (D-87):
 *   page-config (kind + วิธีอ่านล็อกอิน) → อ่าน token/service ของหน้า → host /ai/session → ตั๋ว → bootstrap(ตั๋ว)
 * ขั้นไหนไม่ผ่าน = ไม่โชว์ปุ่ม + บอกสาเหตุใน console (หน้าหลังบ้านต้องไม่พังตาม)
 */
class Controller {
  private api: Api
  private pa: PageAuth | null = null
  private kind = ''
  private mode: 'host' | 'browser' = 'host'
  private fetcher: HostFetcher | null = null
  private reader: HostReader | null = null
  private timer: ReturnType<typeof setInterval> | null = null
  private lastSig: string | null = null
  private failedAt = 0
  private configFailedAt = 0
  private configDead = false
  private queue: Promise<void> = Promise.resolve()
  private service = ''
  private token = ''
  private stopped = false

  constructor(
    private ds: Record<string, string>,
    apiBase: string,
    private publicKey: string,
  ) {
    this.api = new Api(apiBase, publicKey)
  }

  stop() {
    this.stopped = true
    if (this.timer) clearInterval(this.timer)
    this.timer = null
  }

  start(pollMs: number) {
    void this.tick()
    this.timer = setInterval(() => void this.tick(), pollMs)
  }

  async once() {
    await this.tick()
    await this.queue
  }

  private async loadConfig(): Promise<boolean> {
    if (this.pa) return true
    if (this.configDead) return false
    if (!this.publicKey) {
      explain('ไม่พบ public key ใน URL ของ script', 'snippet ต้องเป็น .../widget/v1/<public_key>/ai-office.js')
      this.configDead = true
      return false
    }
    if (this.configFailedAt && Date.now() - this.configFailedAt < RETRY_MS) return false
    const r = await this.api.pageConfig()
    if (this.stopped) return false
    if (r.status === 0 || r.status >= 500) {
      // backend ล่ม → หลังบ้านใช้งานปกติ · ลองใหม่ทีหลังเงียบ ๆ
      this.configFailedAt = Date.now()
      explain(`เรียก ${this.api.pageConfigURL()} ไม่สำเร็จ (${r.status || r.code})`, 'ai-office-backend ทำงานอยู่ไหม — จะลองใหม่ทุก 30 วินาที')
      return false
    }
    if (!r.ok || !r.data) {
      this.configDead = true
      explain(`page-config ตอบ ${r.status} ${r.code} ${r.error}`.trim(), HINTS[r.code])
      return false
    }
    if (!r.data.enabled || !r.data.page_auth) {
      this.configDead = true
      explain(`ยังไม่เปิดใช้งาน (reason: ${r.data.reason ?? '-'})`, HINTS[r.data.reason ?? ''])
      return false
    }
    this.pa = r.data.page_auth
    this.kind = r.data.kind ?? ''
    this.reader = readerFor(this.kind)
    this.mode = r.data.mode === 'browser' ? 'browser' : 'host'
    // โหมด browser: widget ยิง API เดิมของหลังบ้านให้ AI ด้วย token ของแอดมินเอง (หลังบ้านไม่ต้องแก้)
    this.fetcher = this.mode === 'browser' ? makeHostFetcher(this.pa, this.reader, this.ds.hostApiBase) : null
    return true
  }

  private async tick() {
    if (this.stopped) return
    try {
      if (!(await this.loadConfig()) || this.stopped) return
      const token = this.reader!.readToken(this.pa!)
      const service = token ? this.reader!.readService(this.pa!) : ''
      // sig = ลายเซ็น session: token เปลี่ยน (ล็อกอินใหม่) หรือ service เปลี่ยน = ต้องทำใหม่
      const sig = token ? `${service || '\0'}|${fingerprint(token)}` : ''
      const retry = this.failedAt > 0 && Date.now() - this.failedAt > RETRY_MS
      if (sig === this.lastSig && !retry) return
      this.lastSig = sig
      this.failedAt = 0
      this.queue = this.queue.then(() => this.transition(token, service)).catch(() => {})
    } catch {
      /* storage เข้าไม่ได้ ฯลฯ — ปล่อยผ่าน ห้ามลากหน้าหลังบ้านพัง */
    }
  }

  private async transition(token: string, service: string) {
    if (this.stopped) return
    const pa = this.pa!
    if (!token) {
      if (widget) explain('ออกจากระบบแล้ว — เก็บผู้ช่วย')
      else explain(`ไม่พบ ${pa.token.source}["${pa.token.key}"] ที่ยังไม่หมดอายุ — ยังไม่ได้ล็อกอินหลังบ้าน`)
      widget?.closeRoom('logout')
      unmount()
      this.service = ''
      this.token = ''
      return
    }
    if (!service) {
      explain(`ไม่พบ service ที่เปิดอยู่ (${pa.service.source}["${pa.service.key}"]) — ยังไม่ได้เลือกเว็บ`)
      widget?.closeRoom('switch_service')
      unmount()
      this.service = ''
      return
    }

    const session = new SessionManager(this.api, service, pa, this.reader, this.ds.hostApiBase,
      this.fetcher ? { fetcher: this.fetcher } : undefined)
    const t: TicketResult = await session.get()
    if (this.stopped) return
    if (!t.ok) {
      if (t.why === 'unavailable') this.failedAt = Date.now()
      explain(`ขอตั๋วจากหลังบ้านไม่ผ่าน (${t.code}${t.reason ? ' · ' + t.reason : ''})`, HINTS[t.reason] ?? HINTS[t.code])
      widget?.closeRoom('switch_service')
      unmount()
      this.service = ''
      return
    }
    const b = await this.api.bootstrap(service, t.ticket)
    if (this.stopped) return
    if (!b.ok || !b.data || !b.data.enabled) {
      if (b.status === 0 || b.status >= 500) this.failedAt = Date.now()
      explain(
        b.data && !b.data.enabled ? `ยังไม่เปิดใช้งาน (reason: ${b.data.reason})` : `bootstrap ตอบ ${b.status} ${b.code}`,
        HINTS[b.data?.reason ?? ''] ?? HINTS[b.code],
      )
      widget?.closeRoom('switch_service')
      unmount()
      this.service = ''
      return
    }
    const cfg = { ...DEFAULT_BOOTSTRAP, ...b.data }
    const deps = { api: this.api, session, fetcher: this.fetcher }
    const from = this.service
    const relogin = this.token !== '' && this.token !== token
    this.service = service
    this.token = token

    if (widget && !widget.preview) {
      const label = cfg.service_label || service
      const sys =
        from && from !== service
          ? `เปลี่ยนจากเว็บ ${from} ไป ${label} — ห้องแชทเดิมถูกปิด ประวัติของ ${from} ยังเปิดดูได้เมื่อกลับไปเว็บนั้น`
          : relogin
            ? 'เข้าสู่ระบบใหม่ — เริ่มห้องใหม่'
            : ''
      widget.switchTo(cfg, deps, sys)
      return
    }
    create(cfg, deps, null)
  }
}

/** ลายนิ้วมือของ token ไว้ดูว่าเปลี่ยนไหม — ไม่เก็บ/ไม่ส่ง token ไปไหน */
function fingerprint(s: string): string {
  let h = 2166136261
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i)
    h = Math.imul(h, 16777619)
  }
  return (h >>> 0).toString(36)
}

function onPreviewMessage(ev: MessageEvent) {
  if (ev.origin !== window.location.origin) return
  const data = ev.data as { type?: string; config?: PreviewConfig }
  if (!data || data.type !== 'ai-office:preview-config' || !data.config) return
  // ตรวจซ้ำว่า instance ที่มีชีวิตอยู่ตอนนี้เป็น preview จริง ไม่พึ่งแค่การถอด listener อย่างเดียว
  if (!widget || !widget.preview) return

  // รับได้เฉพาะ field หน้าตา — enabled/allowlist/quota/model เป็นเรื่องความปลอดภัยและค่าใช้จ่าย ต้องมาจาก server
  const safe: Record<string, unknown> = {}
  for (const k of PREVIEW_ALLOWED_FIELDS) {
    if (k in data.config) safe[k] = (data.config as Record<string, unknown>)[k]
  }
  const cfg = { ...widget.cfg, ...(safe as Partial<Bootstrap>) }
  widget.paint(cfg)
  widget.reset()
}

/** คำอธิบายสาเหตุให้คนติดตั้งอ่าน — ไม่ทำให้หน้า office พัง แค่บอกใน console */
function explain(reason: string, hint?: string) {
  console.info(`[ai-office] ไม่แสดงผู้ช่วย: ${reason}` + (hint ? `\n           → ${hint}` : ''))
}

const HINTS: Record<string, string> = {
  ORIGIN_NOT_ALLOWED: 'เพิ่มโดเมนของหน้านี้ลงใน "โดเมนที่อนุญาต" ของ office ที่คอนโซล',
  NOT_FOUND: 'public key ใน snippet ไม่ตรงกับ office ไหนเลย — ถูกลบหรือเปลี่ยน key ไปแล้วหรือเปล่า',
  SECRET_INVALID: 'secret_key ที่หลังบ้านตั้งไว้ไม่ตรง/ถูกยกเลิก — ออก secret ใหม่ที่คอนโซลแล้วใส่ที่หลังบ้าน',
  AI_UNAVAILABLE: 'หลังบ้านติดต่อ ai-office-backend ไม่ได้ — ตรวจ AI_BACKEND_URL ของหลังบ้าน',
  AI_DISABLED: 'หลังบ้านยังไม่ได้ตั้งค่า AI (AI_BACKEND_URL / secret)',
  TICKET_INVALID: 'ตั๋วไม่ตรงกับเว็บนี้ — ลองโหลดหน้าใหม่',
  NO_TOKEN: 'ยังไม่ได้ล็อกอินหลังบ้าน',
  office_disabled: 'office นี้ถูกปิดทั้งชุดที่คอนโซล',
  service_disabled: 'service นี้ยังไม่ได้เปิดที่คอนโซล',
  service_not_in_office: 'service นี้ยังไม่ได้เพิ่มใน office ที่คอนโซล',
  not_in_allowlist: 'เพิ่มผู้ใช้นี้ลง allowlist ของ service ที่คอนโซล หรือเปิด "ทุกคนในเว็บนี้"',
  kind_not_set: 'office นี้ยังไม่ได้เลือกชนิดหลังบ้าน (kind) ที่คอนโซล',
  kind_mismatch: 'ชนิดหลังบ้าน (kind) ของ office ไม่ตรงกับหลังบ้านที่ติดตั้ง',
  no_secret: 'service นี้ยังไม่มี secret_key — ออกที่คอนโซลแล้วใส่ที่หลังบ้าน',
  expired: 'token ของหลังบ้านหมดอายุ — ล็อกอินใหม่',
  no_user: 'บัญชีนี้ใช้ผู้ช่วยไม่ได้ (บัญชีทดลอง/ไม่มีตัวตน)',
  no_service: 'หลังบ้านไม่รู้จัก service นี้',
  service_not_allowed: 'บัญชีนี้ไม่มีสิทธิ์เข้า service นี้ในหลังบ้าน',
}

function kebab(s: string) {
  return s.replace(/[A-Z]/g, (m) => '-' + m.toLowerCase())
}

function installGlobal() {
  const api = {
    open: () => widget?.toggle(true),
    close: () => widget?.toggle(false),
    // ---- test hook (อ่านอย่างเดียว) ----
    __hasLauncher: () => !!widget && widget.ui.launcher.style.display !== 'none',
    __isOpen: () => !!widget && widget.ui.panel.dataset.open === 'true',
    __launcherStyle: () => (widget ? widget.ui.launcher.style : ({} as CSSStyleDeclaration)),
    __config: () => (widget ? { ...widget.cfg } : {}),
    __host: () => widget?.host ?? null,
    __state: () => (widget ? { service: widget.service, conversation_id: widget.convId, streaming: widget.streaming } : null),
    __ui: () => widget?.ui ?? null,
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
  if (i <= 0) return ''
  const key = decodeURIComponent(parts[i - 1])
  // snippet แบบไม่มี key (.../widget/v1/ai-office.js) = ให้ backend หา office จากโดเมนของหน้า
  return key === 'v1' ? 'auto' : key
}

const self = typeof document !== 'undefined' ? (document.currentScript as HTMLScriptElement | null) : null
if (self) {
  const ds: Record<string, string> = {}
  for (const k in self.dataset) ds[k] = self.dataset[k] as string
  if (self.src) {
    if (!ds.apiBase) ds.apiBase = new URL(self.src, location.href).origin
    if (!ds.publicKey) ds.publicKey = keyFromScriptURL(self.src)
  }
  const start = () => boot(ds)
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start)
  else start()
}
