import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount, unmount, __resetSessions } from '../src/index'
import { V10X_PAGE, isPageConfig, loginV10x } from './fixtures'

// spec §8 — สิ่งที่แอดมินเห็นเมื่อผู้ช่วยใช้ไม่ได้ · ห้ามโชว์ข้อความเทคนิคของ server

const base = {
  enabled: true,
  office_id: 'demo',
  service_id: 'K11S',
  service_label: 'เว็บ K11S',
  is_hidden: false,
  avatar_url: '',
  display_name: 'ผู้ช่วยหลังบ้าน',
  greeting: 'สวัสดีครับ',
  theme: 'light' as const,
  placement: { position: 'bottom-right' as const, offset_x: 12, offset_y: 12 },
}

const w = () => (window as any).__aiOffice
const realFetch = globalThis.fetch

function sse(events: [string, unknown][]): Response {
  const body = events.map(([e, d]) => `event: ${e}\ndata: ${JSON.stringify(d)}\n\n`).join('')
  return new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

const fail = (status: number, message: string, error = 'ข้อความเทคนิคของ server') =>
  Response.json({ code: status, message, error }, { status })

/** fetch จำลอง: page-config = v10x · ตั๋วออกเป็น tkt1, tkt2, … · /chat ให้ handler ตอบ */
function mockAPI(chat: (ticket: string) => Response, session?: () => Response) {
  let n = 0
  const calls: string[] = []
  globalThis.fetch = (async (url: string, init: any = {}) => {
    const u = String(url)
    calls.push(u)
    if (isPageConfig(u)) return Response.json({ payload: V10X_PAGE })
    if (u.endsWith('/bootstrap')) return Response.json({ payload: base })
    if (u.endsWith('/browser-session')) return session ? session() : Response.json({ payload: { ticket: `tkt${++n}`, expires_in: 1800 } })
    if (u.endsWith('/chat')) return chat(String(init.headers.Authorization).replace('Bearer ', ''))
    throw new Error('unexpected ' + u)
  }) as any
  return calls
}

beforeEach(() => {
  document.body.innerHTML = ''
  localStorage.clear()
  delete (window as any).__aiOffice
  __resetSessions()
  loginV10x('K11S')
})
afterEach(() => {
  unmount()
  globalThis.fetch = realFetch
})

describe('ปุ่มผู้ช่วย', () => {
  it('bootstrap ผ่านแต่ขอตั๋วไม่ผ่าน → ไม่โชว์ปุ่ม', async () => {
    const calls = mockAPI(() => sse([]), () => fail(503, 'BACKOFFICE_UNAVAILABLE'))
    await mount({ dataset: { apiBase: 'https://ai.test' } })
    expect(calls.some((u) => u.endsWith('/browser-session'))).toBe(true)
    expect(w()).toBeUndefined()
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('ขอตั๋วผ่าน → โชว์ปุ่ม และใช้ตั๋วใบนั้นตอนถามข้อแรก (ไม่ขอซ้ำ)', async () => {
    const calls = mockAPI((t) => sse([['token', { text: `ตอบด้วย ${t}` }], ['done', {}]]))
    await mount({ dataset: { apiBase: 'https://ai.test' } })
    expect(w().__hasLauncher()).toBe(true)
    await w().__send('สวัสดี')
    expect(calls.filter((u) => u.endsWith('/browser-session'))).toHaveLength(1)
    expect(w().__logText()).toContain('ตอบด้วย tkt1')
  })
})

describe('ข้อความเมื่อถามไม่สำเร็จ', () => {
  it('ตั๋วหมดอายุ → ขอใหม่เงียบ ๆ แล้วถามต่อได้ ไม่มีฟองแดง', async () => {
    mockAPI((t) => (t === 'tkt1' ? fail(401, 'TICKET_INVALID') : sse([['token', { text: 'ตอบแล้วครับ' }], ['done', {}]])))
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('ยอดฝากวันนี้')
    const log = w().__logText() as string
    expect(log).toContain('ตอบแล้วครับ')
    expect(log).not.toContain('หมดอายุ')
  })

  it('ขอใหม่แล้วตั๋วยังใช้ไม่ได้ → บอกให้รีเฟรช ไม่วนขอซ้ำ', async () => {
    const calls = mockAPI(() => fail(401, 'TICKET_INVALID'))
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('ยอดฝากวันนี้')
    expect(calls.filter((u) => u.endsWith('/chat'))).toHaveLength(2)
    expect(w().__logText()).toContain('กรุณารีเฟรชหน้า')
  })

  it.each([
    ['LLM_NOT_CONFIGURED', 503, 'ผู้ดูแลระบบยังตั้งค่าไม่เสร็จ'],
    ['BACKOFFICE_UNAVAILABLE', 503, 'ตรวจสอบสิทธิ์กับหลังบ้านไม่ได้ชั่วคราว'],
    ['SOMETHING_NEW', 500, 'ผู้ช่วยไม่พร้อมใช้งานชั่วคราว'],
  ])('%s → ข้อความให้คนอ่าน ไม่ใช่ข้อความเทคนิค', async (code, status, want) => {
    mockAPI(() => fail(status, code))
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('สวัสดี')
    const log = w().__logText() as string
    expect(log).toContain(want)
    expect(log).not.toContain('ข้อความเทคนิคของ server')
    expect(w().__hasLauncher()).toBe(true)
  })

  it('เว็บถูกปิดกลางแชท → แจ้ง + เก็บปุ่ม + ปิดช่องพิมพ์', async () => {
    mockAPI(() => fail(403, 'SERVICE_DISABLED'))
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('สวัสดี')
    expect(w().__logText()).toContain('ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่')
    expect(w().__hasLauncher()).toBe(false)
  })

  it('ห้องเดิมหาย (not_found) → ข้อความถัดไปเปิดห้องใหม่', async () => {
    let sent: any[] = []
    const realMock = () => {
      globalThis.fetch = (async (url: string, init: any = {}) => {
        const u = String(url)
        if (isPageConfig(u)) return Response.json({ payload: V10X_PAGE })
        if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
        if (u.endsWith('/chat')) {
          const body = JSON.parse(init.body)
          sent.push(body)
          if (sent.length === 1) return sse([['status', { phase: 'thinking', conversation_id: 'conv_old' }], ['token', { text: 'ok' }], ['done', {}]])
          if (sent.length === 2) return sse([['error', { code: 'not_found', message: 'ไม่พบห้องแชทนี้ เริ่มห้องใหม่ได้เลย' }]])
          return sse([['status', { phase: 'thinking', conversation_id: 'conv_new' }], ['done', {}]])
        }
        throw new Error('unexpected ' + u)
      }) as any
    }
    realMock()
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('ข้อ 1')
    await w().__send('ข้อ 2')
    await w().__send('ข้อ 3')
    expect(sent.map((b) => b.conversation_id)).toEqual(['', 'conv_old', ''])
    expect(w().__conversationID()).toBe('conv_new')
  })
})
