import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { buildHostURL, hostFetch } from '../src/hostfetch'
import { mount, unmount } from '../src/index'

const realFetch = globalThis.fetch
afterEach(() => {
  unmount()
  globalThis.fetch = realFetch
})

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

describe('hostfetch: กติกา path', () => {
  const b = 'https://office.example.com/api'
  it('path ปกติต่อท้าย base', () => {
    expect(buildHostURL(b, '/get-member-list-v3/K11S')).toBe('https://office.example.com/api/get-member-list-v3/K11S')
  })
  it.each(['x', '//evil.com/a', '/a\\b', 'https://evil.com', '/../admin', '/a/../../x', 'javascript:alert(1)'])(
    '%s ถูกปฏิเสธ',
    (path) => {
      expect(buildHostURL(b, path)).toBeNull()
    },
  )
  it('query ถูกใส่เป็น search params', () => {
    expect(buildHostURL(b, '/m', { search: 'a b', page: '1' })).toBe('https://office.example.com/api/m?search=a+b&page=1')
  })
})

describe('hostfetch: การยิง', () => {
  it('ไม่มี token → 401 ไม่ยิง', async () => {
    let called = false
    globalThis.fetch = (async () => ((called = true), new Response('{}'))) as any
    expect(await hostFetch('https://o.test/api', { id: '1', method: 'GET', path: '/x' }, '')).toEqual({ status: 401, body: '' })
    expect(called).toBe(false)
  })
  it('method อื่นนอกจาก GET/POST ไม่ยิง', async () => {
    let called = false
    globalThis.fetch = (async () => ((called = true), new Response('{}'))) as any
    expect((await hostFetch('https://o.test/api', { id: '1', method: 'DELETE', path: '/x' }, 't')).status).toBe(0)
    expect(called).toBe(false)
  })
  it('ส่ง Bearer และไม่ส่ง cookie', async () => {
    let init: any
    globalThis.fetch = (async (_u: string, i: any) => ((init = i), new Response('{"code":0}', { status: 200 }))) as any
    const r = await hostFetch('https://o.test/api', { id: '1', method: 'GET', path: '/x' }, 'tok')
    expect(r).toEqual({ status: 200, body: '{"code":0}' })
    expect(init.credentials).toBe('omit')
    expect(init.headers.Authorization).toBe('Bearer tok')
  })
  it('network error → status 0', async () => {
    globalThis.fetch = (async () => {
      throw new Error('offline')
    }) as any
    expect((await hostFetch('https://o.test/api', { id: '1', method: 'GET', path: '/x' }, 't')).status).toBe(0)
  })
})

function sse(events: [string, unknown][]): Response {
  const body = events.map(([e, d]) => `event: ${e}\ndata: ${JSON.stringify(d)}\n\n`).join('')
  return new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

describe('แชท: ตั๋ว → SSE → fetch/relay → การ์ด', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    localStorage.clear()
    localStorage.setItem('auth_token', JSON.stringify({ value: 'office-jwt', expiration: Math.floor(Date.now() / 1000) + 600 }))
    localStorage.setItem('web-service', 'K11S')
  })

  it('ครบทั้งเส้น', async () => {
    const calls: { url: string; init: any }[] = []
    globalThis.fetch = (async (url: string, init: any = {}) => {
      calls.push({ url: String(url), init })
      const u = String(url)
      if (u.endsWith('/api/ai/widget/page-config')) {
        return Response.json({
          payload: {
            kind: 'office-v10x', mode: 'browser', host_api_base: 'https://office.test/api',
            identity: {
              permissions_request: '/GetPrefixOfficeMainSuperCom/{service}',
              permissions: { path: 'permission', pluck: 'code', where_field: 'action.is_view', where_value: true },
            },
          },
        })
      }
      if (u === 'https://office.test/api/GetPrefixOfficeMainSuperCom/K11S') {
        return Response.json({ code: 0, permission: [
          { code: 'MEMBER', action: { is_view: true } },
          { code: 'BANK', action: { is_view: false } },
        ] })
      }
      if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
      if (u.endsWith('/chat')) {
        return sse([
          ['status', { phase: 'thinking', text: 'กำลังคิด…', conversation_id: 'conv_1' }],
          ['fetch', { id: 'f1', method: 'GET', path: '/get-member-list-v3/K11S', query: { search: 'somchai01' }, timeout_ms: 20000 }],
          ['card', { id: 'c1', kind: 'ok', tool: 'member_lookup', title: 'ข้อมูลสมาชิก', fields: [{ label: 'ชื่อ', display: '<b>สมชาย</b>' }],
            table: { columns: [{ label: 'ธนาคาร' }], rows: [[{ display: 'KBANK' }]] }, link: { label: 'เปิดหน้าสมาชิก', path: '/member/somchai01' },
            fetched_at: new Date().toISOString() }],
          ['token', { text: 'ดูได้ในการ์ดครับ' }],
          ['done', {}],
        ])
      }
      if (u.startsWith('https://office.test/api/get-member-list-v3/K11S')) return new Response('{"code":0,"data":[]}', { status: 200 })
      if (u.includes('/chat/relay/')) return Response.json({ code: 200 })
      throw new Error('unexpected ' + u)
    }) as any

    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    const w = window as any
    // shadow root เป็น closed — ส่งข้อความผ่าน test hook ที่กรอก input แล้วกดส่งให้
    await w.__aiOffice.__send('เช็คยูส somchai01')

    const session = calls.find((c) => c.url.endsWith('/browser-session'))!
    expect(session.init.headers.Authorization).toBe('Bearer office-jwt')
    expect(JSON.parse(session.init.body)).toEqual({ permissions: ['MEMBER'] }) // is_view=false ถูกตัด

    const chat = calls.find((c) => c.url.endsWith('/chat'))!
    expect(chat.init.headers.Authorization).toBe('Bearer tkt') // แชทใช้ตั๋ว ไม่ใช้ token ของหน้า

    const host = calls.find((c) => c.url.startsWith('https://office.test/api/get-member-list-v3'))!
    expect(host.url).toBe('https://office.test/api/get-member-list-v3/K11S?search=somchai01')
    expect(host.init.headers.Authorization).toBe('Bearer office-jwt')

    const relay = calls.find((c) => c.url.includes('/chat/relay/f1'))!
    expect(relay.init.headers.Authorization).toBe('Bearer tkt')
    expect(JSON.parse(relay.init.body)).toEqual({ status: 200, body: '{"code":0,"data":[]}' })

    const texts = w.__aiOffice.__logText() as string
    expect(texts).toContain('ข้อมูลสมาชิก')
    expect(texts).toContain('<b>สมชาย</b>') // เป็นข้อความ ไม่ใช่ HTML
    expect(texts).toContain('ดูได้ในการ์ดครับ')
    expect(texts).toContain('KBANK')
    expect(texts).toContain('เปิดหน้าสมาชิก')
    expect(w.__aiOffice.__conversationID()).toBe('conv_1')
  })

  it('server ส่ง error → ฟองแดงพร้อมข้อความจาก server และไม่มีฟอง "ไม่มีคำตอบ"', async () => {
    globalThis.fetch = (async (url: string) => {
      const u = String(url)
      if (u.endsWith('/page-config')) return Response.json({ payload: { kind: 'office-v10x', mode: 'browser', host_api_base: 'https://office.test/api' } })
      if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
      if (u.endsWith('/chat')) {
        return sse([
          ['status', { phase: 'thinking', text: 'กำลังคิด…', conversation_id: 'conv_2' }],
          ['error', { code: 'busy', message: 'ผู้ช่วยกำลังตอบคนอื่นอยู่หลายคน' }],
        ])
      }
      throw new Error('unexpected ' + u)
    }) as any
    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    const w = window as any
    await w.__aiOffice.__send('สวัสดี')
    const texts = w.__aiOffice.__logText() as string
    expect(texts).toContain('ผู้ช่วยกำลังตอบคนอื่นอยู่หลายคน')
    expect(texts).not.toContain('(ไม่มีคำตอบ)')
    expect(texts).not.toContain('กำลังคิด…') // loader ถูกเก็บแล้ว
  })
})
