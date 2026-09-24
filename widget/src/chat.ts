import { streamChat, type Api, type ChatEvent } from './api'
import type { FetchOrder, HostFetcher } from './hostfetch'
import type { SessionFailure, SessionManager } from './session'
import type { Bootstrap, Card, HistoryMessage } from './types'
import {
  addBubble,
  addLoader,
  addSys,
  applyAppearance,
  applyPlacement,
  el,
  renderCard,
  renderHistory,
  scroll,
  type UI,
} from './ui'

/** ข้อความที่ผู้ใช้เห็น (spec §8) — ใช้เมื่อ server ไม่ได้ส่งข้อความมาเอง */
export const MESSAGES: Record<string, string> = {
  login: 'กรุณาเข้าสู่ระบบหลังบ้านใหม่ แล้วลองอีกครั้ง',
  refused: 'บริการ AI ของเว็บนี้ปิดใช้งานอยู่ ติดต่อผู้ดูแล',
  unavailable: 'ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง',
  dropped: 'การเชื่อมต่อหลุดระหว่างตอบ คำตอบอาจไม่ครบ กรุณาถามใหม่อีกครั้ง',
  quota_exceeded: 'โควตาของเดือนนี้ใช้ครบแล้ว ติดต่อผู้ดูแลเพื่อเพิ่มโควตา',
  busy: 'ตอนนี้มีคนใช้ผู้ช่วยพร้อมกันเต็มแล้ว กรุณาลองใหม่อีกครั้งในอีกสักครู่',
  llm_unavailable: 'ผู้ช่วยไม่ตอบกลับในเวลาที่กำหนด กรุณาลองใหม่อีกครั้ง',
  bad_request: 'ข้อความว่างหรือยาวเกินไป',
  internal: 'เกิดข้อผิดพลาดภายในระบบผู้ช่วย กรุณาลองใหม่อีกครั้ง',
  preview: 'โหมดตัวอย่าง — ไม่ได้ส่งคำถามจริง',
}

export interface WidgetDeps {
  api: Api | null
  session: SessionManager | null
  /** โหมด browser: ยิง API เดิมของหลังบ้านตามที่ backend สั่ง (null = โหมด host) */
  fetcher?: HostFetcher | null
}

/**
 * กล่องแชท 1 ตัว · ไม่รู้ว่าหลังบ้านเป็นชนิดไหน (อ่านแค่ตั๋วจาก SessionManager)
 * ห้องแชท = conversation_id ฝั่ง server · เปลี่ยนเว็บ = ปิดห้อง + ยกเลิก stream (B-15)
 */
export class Widget {
  cfg: Bootstrap
  convId = ''
  private readOnly = false
  private stream: AbortController | null = null
  private deps: WidgetDeps

  constructor(
    readonly ui: UI,
    readonly host: HTMLElement,
    cfg: Bootstrap,
    readonly preview: boolean,
    deps: WidgetDeps,
  ) {
    this.cfg = cfg
    this.deps = deps
    ui.launcher.addEventListener('click', () => this.toggle())
    ui.send.addEventListener('click', () => void this.send())
    ui.input.addEventListener('keydown', (e) => {
      const k = e as KeyboardEvent
      // Enter = ส่ง · Shift+Enter = ขึ้นบรรทัด · กำลังพิมพ์ภาษาไทยด้วย IME อย่าส่ง
      if (k.key === 'Enter' && !k.shiftKey && !k.isComposing) {
        k.preventDefault()
        void this.send()
      }
    })
    ui.head.history.addEventListener('click', () => void this.openHistory())
    ui.hist.back.addEventListener('click', () => this.showLog())
    ui.hist.fresh.addEventListener('click', () => {
      this.closeRoom('user_closed')
      this.showLog()
      this.reset()
    })
    if (preview) ui.head.history.hidden = true
  }

  get service(): string {
    return this.deps.session?.service ?? this.cfg.service_id
  }

  get streaming(): boolean {
    return this.stream !== null
  }

  /** ทาหน้าตาใหม่โดยไม่ล้างห้อง (ใช้กับ preview) */
  paint(cfg: Bootstrap) {
    this.cfg = cfg
    this.host.setAttribute('data-theme', resolveTheme(cfg.theme))
    applyAppearance(this.ui, cfg)
    applyPlacement(this.ui, cfg)
    this.ui.launcher.style.display = cfg.is_hidden ? 'none' : 'grid'
  }

  /** เริ่มห้องใหม่: ล้างจอ + ทักทาย · sys = ข้อความระบบบรรทัดแรก (เช่น เปลี่ยนเว็บแล้ว) */
  reset(sys?: string) {
    this.abort()
    this.convId = ''
    this.readOnly = false
    this.ui.log.textContent = ''
    if (sys) addSys(this.ui.log, sys)
    if (this.cfg.greeting) addBubble(this.ui.log, 'ai', this.cfg.greeting)
    if (this.preview) addSys(this.ui.log, MESSAGES.preview)
    this.applyNotice()
  }

  private applyNotice() {
    const locked = this.cfg.notice === 'quota_exceeded'
    this.ui.input.disabled = locked
    this.ui.send.disabled = locked
    if (locked) addBubble(this.ui.log, 'ai', MESSAGES.quota_exceeded, 'err')
  }

  /**
   * เปลี่ยน service / ผู้ใช้: ปิดห้องเดิม (ด้วยตั๋วเดิม) → ยกเลิก stream → ห้องใหม่
   * ห้ามให้คำตอบของเว็บเก่าโผล่ในห้องใหม่ (B-15) — abort ก่อนเปลี่ยน deps เสมอ
   */
  switchTo(cfg: Bootstrap, deps: WidgetDeps, sys: string) {
    this.closeRoom('switch_service')
    this.abort()
    this.deps = deps
    this.paint(cfg)
    this.showLog()
    this.reset(sys)
  }

  toggle(open?: boolean) {
    const next = open ?? this.ui.panel.dataset.open !== 'true'
    this.ui.panel.dataset.open = String(next)
    if (next && !this.ui.input.disabled) this.ui.input.focus()
  }

  abort() {
    if (this.stream) {
      this.stream.abort()
      this.stream = null
    }
  }

  /** ปิดห้องที่ server (best effort · keepalive) */
  closeRoom(reason: 'switch_service' | 'user_closed' | 'logout') {
    const { api, session } = this.deps
    const ticket = session?.current()
    if (this.convId && api && ticket && !this.readOnly) api.close(this.service, ticket, this.convId, reason)
    this.convId = ''
  }

  destroy() {
    this.abort()
    this.host.remove()
  }

  async send(): Promise<void> {
    const text = this.ui.input.value.trim()
    if (!text || this.stream || this.ui.input.disabled) return
    if (this.readOnly) this.reset()
    this.ui.input.value = ''
    addBubble(this.ui.log, 'me', text)

    const { api, session } = this.deps
    if (this.preview || !api || !session) {
      addBubble(this.ui.log, 'ai', MESSAGES.preview)
      return
    }

    const ac = new AbortController()
    this.stream = ac
    this.setBusy(true)
    const loader = addLoader(this.ui.log, 'กำลังส่งคำถาม…')
    let bubble: HTMLDivElement | null = null
    let txt: HTMLElement | null = null
    let cards: HTMLElement | null = null
    let sawEvent = false
    let finished = false
    let curTicket = ''
    const service = this.service
    const fetcher = this.deps.fetcher ?? null

    const ensure = () => {
      if (bubble) return
      bubble = addBubble(this.ui.log, 'ai', '')
      txt = bubble.querySelector('.txt')
      cards = el('div', 'cards')
      bubble.appendChild(cards)
    }

    const onEvent = (ev: ChatEvent) => {
      // service เปลี่ยนระหว่างรอ → ทิ้งทุกอย่าง
      if (ac.signal.aborted || this.service !== service) return
      sawEvent = true
      switch (ev.type) {
        case 'status':
          loader.set(ev.data.text || 'กำลังทำงาน…')
          if (ev.data.conversation_id) this.convId = ev.data.conversation_id
          break
        case 'fetch': {
          // backend ขอให้ยิงหลังบ้านด้วย token ของแอดมินคนนี้ → ส่งผลกลับ (ไม่รอ ไม่บล็อกการอ่าน stream)
          const order = ev.data as FetchOrder
          loader.set('กำลังดึงข้อมูลจากหลังบ้าน…')
          if (fetcher && order?.id) {
            const tk = curTicket
            void fetcher(order).then((r) => (ac.signal.aborted ? undefined : api.relay(service, tk, order.id, r.status, r.body)))
          }
          break
        }
        case 'card':
          ensure()
          cards!.appendChild(renderCard(ev.data as Card))
          scroll(this.ui.log)
          break
        case 'token':
          loader.remove()
          ensure()
          txt!.textContent = (txt!.textContent ?? '') + (ev.data.text ?? '')
          scroll(this.ui.log)
          break
        case 'done':
          finished = true
          loader.remove()
          if (ev.data.conversation_id) this.convId = ev.data.conversation_id
          break
        case 'error':
          finished = true
          loader.remove()
          if (ev.data.code === 'not_found') this.convId = ''
          if (ev.data.code === 'quota_exceeded') {
            this.cfg = { ...this.cfg, notice: 'quota_exceeded' }
          }
          addBubble(this.ui.log, 'ai', ev.data.message || MESSAGES[ev.data.code] || MESSAGES.internal, 'err')
          break
      }
    }

    try {
      for (let attempt = 0; ; attempt++) {
        const t = await session.get(attempt > 0)
        if (ac.signal.aborted) return
        if (!t.ok) {
          this.fail(t.why)
          return
        }
        curTicket = t.ticket
        const res = await streamChat(api.chatURL(service), t.ticket, { text, conversation_id: this.convId || undefined }, onEvent, ac.signal)
        if (res.status === -1 || ac.signal.aborted) return
        // ตั๋วหมดระหว่างทาง → ต่ออายุเงียบ ๆ แล้วลองอีก 1 ครั้ง (spec §8)
        if (res.status === 401 && attempt === 0 && !sawEvent) {
          session.drop()
          continue
        }
        if (res.status === 401) this.fail('login')
        else if (res.status === 403) this.fail('refused')
        else if (res.status === 0) this.fail(sawEvent ? 'dropped' : 'unavailable')
        else if (res.status >= 500 && !sawEvent) this.fail('unavailable')
        else if (res.status !== 200 && !sawEvent) addBubble(this.ui.log, 'ai', MESSAGES[res.code.toLowerCase()] || MESSAGES.internal, 'err')
        else if (!finished) this.fail('dropped')
        return
      }
    } finally {
      loader.remove()
      if (this.stream === ac) {
        this.stream = null
        this.setBusy(false)
        if (this.cfg.notice === 'quota_exceeded') {
          this.ui.input.disabled = true
          this.ui.send.disabled = true
        }
      }
    }
  }

  private fail(why: SessionFailure | 'dropped') {
    addBubble(this.ui.log, 'ai', MESSAGES[why], 'err')
  }

  private setBusy(b: boolean) {
    this.ui.send.disabled = b
    this.ui.head.history.disabled = b
  }

  private showLog() {
    this.ui.hist.box.hidden = true
    this.ui.log.hidden = false
  }

  /** ประวัติ 7 วันของ office+service+ผู้ใช้ในตั๋วเท่านั้น (server กรองเอง) */
  async openHistory(): Promise<void> {
    const { api, session } = this.deps
    if (!api || !session || this.stream) return
    this.ui.log.hidden = true
    this.ui.hist.box.hidden = false
    this.ui.hist.list.textContent = 'กำลังโหลด…'
    const r = await this.withTicket((tk) => api.history(this.service, tk))
    if (this.ui.hist.box.hidden) return
    if (!r || !r.ok || !r.data) {
      this.ui.hist.list.textContent = MESSAGES.unavailable
      return
    }
    renderHistory(this.ui.hist.list, r.data.data ?? [], (id) => void this.openConversation(id))
  }

  async openConversation(id: string): Promise<void> {
    const { api, session } = this.deps
    if (!api || !session) return
    const service = this.service
    const r = await this.withTicket((tk) => api.conversation(service, tk, id))
    if (this.service !== service) return
    this.showLog()
    if (!r || !r.ok || !r.data) {
      addBubble(this.ui.log, 'ai', MESSAGES.unavailable, 'err')
      return
    }
    if (this.convId && this.convId !== id) this.closeRoom('user_closed')
    this.abort()
    this.ui.log.textContent = ''
    const closed = r.data.conversation?.closed
    addSys(this.ui.log, closed ? 'ห้องนี้ปิดแล้ว (อ่านอย่างเดียว) — พิมพ์คำถามใหม่เพื่อเริ่มห้องใหม่' : 'เปิดห้องเดิม — ถามต่อได้เลย')
    for (const m of r.data.messages ?? []) renderHistoryMessage(this.ui.log, m)
    this.convId = closed ? '' : id
    this.readOnly = !!closed
    this.applyNotice()
  }

  private async withTicket<T>(fn: (ticket: string) => Promise<{ ok: boolean; status: number } & T>) {
    const session = this.deps.session!
    for (let attempt = 0; attempt < 2; attempt++) {
      const t = await session.get(attempt > 0)
      if (!t.ok) return null
      const r = await fn(t.ticket)
      if (r.status === 401 && attempt === 0) {
        session.drop()
        continue
      }
      return r
    }
    return null
  }
}

function renderHistoryMessage(log: HTMLDivElement, m: HistoryMessage) {
  if (m.role === 'user') {
    addBubble(log, 'me', m.text)
    return
  }
  const b = addBubble(log, 'ai', m.text)
  if (m.cards?.length) {
    const box = el('div', 'cards')
    for (const c of m.cards) box.appendChild(renderCard(c))
    b.appendChild(box)
  }
}

export function resolveTheme(mode: string): 'light' | 'dark' {
  if (mode === 'light' || mode === 'dark') return mode
  return window.matchMedia?.('(prefers-color-scheme: dark)')?.matches ? 'dark' : 'light'
}
