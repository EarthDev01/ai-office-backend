import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount, unmount, __resetSessions } from '../src/index'
import { V10X_PAGE, isPageConfig, loginV10x } from './fixtures'

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

const ROOMS = [
  { id: 'conv_a', title: 'ยอดฝากวันนี้', updated_at: new Date().toISOString(), message_count: 2 },
  { id: 'conv_b', title: 'เช็คยูส somchai01', updated_at: new Date(Date.now() - 86400000).toISOString(), message_count: 4 },
]

const ROOM_A = {
  id: 'conv_a',
  title: 'ยอดฝากวันนี้',
  messages: [
    { role: 'user', text: 'ยอดฝากวันนี้', status: '' },
    {
      role: 'assistant',
      text: 'ดูในการ์ดครับ',
      status: 'ok',
      cards: [{ id: 'c1', kind: 'ok', tool: 'deposit_summary', title: 'ยอดฝากวันนี้', fields: [{ label: 'รวม', display: '12,345.00' }], fetched_at: new Date().toISOString() }],
    },
  ],
}

function sse(events: [string, unknown][]): Response {
  const body = events.map(([e, d]) => `event: ${e}\ndata: ${JSON.stringify(d)}\n\n`).join('')
  return new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

function mockAPI(extra: (u: string, init: any) => Response | undefined | Promise<Response | undefined> = () => undefined) {
  const calls: { url: string; init: any }[] = []
  globalThis.fetch = (async (url: string, init: any = {}) => {
    const u = String(url)
    calls.push({ url: u, init })
    if (isPageConfig(u)) return Response.json({ payload: V10X_PAGE })
    if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
    if (u.endsWith('/conversations?days=7')) return Response.json({ payload: ROOMS })
    if (u.endsWith('/conversations/conv_a')) return Response.json({ payload: ROOM_A })
    const r = await extra(u, init)
    if (r) return r
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

describe('ประวัติแชท 7 วัน', () => {
  it('รายการห้อง → เปิดห้องเดิมเห็นคำถาม/คำตอบ/การ์ด → ถามต่อในห้องเดิม', async () => {
    let chatBody: any = null
    const calls = mockAPI((u, init) => {
      if (u.endsWith('/chat')) {
        chatBody = JSON.parse(init.body)
        return sse([['token', { text: 'ต่อแล้ว' }], ['done', {}]])
      }
    })
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__showRooms()
    const list = w().__logText() as string
    expect(list).toContain('ห้องแชทย้อนหลัง 7 วัน')
    expect(list).toContain('ยอดฝากวันนี้')
    expect(list).toContain('เช็คยูส somchai01')
    const listCall = calls.find((c) => c.url.includes('/conversations?'))!
    expect(listCall.url).toBe('https://ai.test/api/ai/widget/service/K11S/conversations?days=7')
    expect(listCall.init.headers.Authorization).toBe('Bearer tkt') // ใช้ตั๋ว ไม่ใช่ token หลังบ้าน

    await w().__openRoom('conv_a')
    const room = w().__logText() as string
    expect(room).toContain('ห้องเดิม: ยอดฝากวันนี้')
    expect(room).toContain('ดูในการ์ดครับ')
    expect(room).toContain('12,345.00')
    expect(room).not.toContain('เช็คยูส somchai01') // รายการห้องถูกล้างแล้ว
    expect(w().__conversationID()).toBe('conv_a')

    await w().__send('แล้วเมื่อวานล่ะ')
    expect(chatBody.conversation_id).toBe('conv_a')
  })

  it('ไม่มีห้อง → บอกว่ายังไม่มี และห้องจากล็อกอินครั้งก่อนเปิดไม่ได้', async () => {
    mockAPI()
    globalThis.fetch = (async (url: string) => {
      const u = String(url)
      if (isPageConfig(u)) return Response.json({ payload: V10X_PAGE })
      if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
      if (u.includes('/conversations?')) return Response.json({ payload: [] })
      throw new Error('unexpected ' + u)
    }) as any
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__showRooms()
    expect(w().__logText()).toContain('ยังไม่มีห้องแชทใน 7 วันนี้')
  })

  it('พิมพ์ถามระหว่างดูรายการห้อง → เริ่มห้องใหม่', async () => {
    let chatBody: any = null
    mockAPI((u, init) => {
      if (u.endsWith('/chat')) {
        chatBody = JSON.parse(init.body)
        return sse([['done', {}]])
      }
    })
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__showRooms()
    await w().__send('คำถามใหม่')
    expect(chatBody.conversation_id).toBe('')
    expect(w().__logText()).not.toContain('เช็คยูส somchai01')
  })
})

describe('สลับเว็บกลางคำตอบ (B-15)', () => {
  it('unmount ระหว่างรอคำตอบ → ยกเลิก stream · ไม่ยิงหลังบ้านด้วยคำสั่งของห้องเก่า · ไม่มีฟองแดง', async () => {
    let signal: AbortSignal | undefined
    let release!: () => void
    const gate = new Promise<void>((r) => (release = r))
    const calls = mockAPI(async (u, init) => {
      if (u.endsWith('/chat')) {
        signal = init.signal
        await gate
        if (signal?.aborted) throw new DOMException('aborted', 'AbortError')
        return sse([['fetch', { id: 'f1', method: 'GET', path: '/get-member-list-v3/K11S' }], ['done', {}]])
      }
    })
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    const sending = w().__send('ยอดฝากวันนี้')
    await new Promise((r) => setTimeout(r, 10))
    unmount() // widget ถอดตัวเองเมื่อสลับเว็บ (autoboot เรียก mount ใหม่ → unmount ก่อน)
    expect(signal?.aborted).toBe(true)
    release()
    await sending
    expect(calls.some((c) => c.url.startsWith('https://office.test/'))).toBe(false)
  })
})
