import { describe, it, expect, afterEach } from 'vitest'
import { installConsoleGlobal, unmount } from '../src/index'

const realFetch = globalThis.fetch
afterEach(() => {
  unmount()
  globalThis.fetch = realFetch
})

function sse(events: [string, unknown][]): Response {
  const body = events.map(([e, d]) => `event: ${e}\ndata: ${JSON.stringify(d)}\n\n`).join('')
  return new Response(body, { status: 200, headers: { 'Content-Type': 'text/event-stream' } })
}

type ConsoleAPI = {
  mount: (o: { getToken: () => string; getPage?: () => string }) => Promise<void>
  __send: (t: string) => Promise<void>
  __logText: () => string
}

describe('โหมดคอนโซล', () => {
  it('ถามผู้ช่วยคอนโซลด้วย token ของคอนโซล ส่งห้องคุยทั้งชุด และไม่ทับ __aiOffice', async () => {
    const calls: { url: string; init: any }[] = []
    globalThis.fetch = (async (url: string, init: any) => {
      calls.push({ url, init })
      return sse([
        ['status', { text: 'กำลังคิด…' }],
        ['token', { text: 'มี 2 ' }],
        ['token', { text: 'domain ครับ' }],
        ['done', {}],
      ])
    }) as any

    const before = (window as any).__aiOffice
    installConsoleGlobal('https://ai.example.com')
    const api = (window as any).__aiOfficeConsole as ConsoleAPI
    let token = 'tok-1'
    await api.mount({ getToken: () => token, getPage: () => '/offices' })
    expect((window as any).__aiOffice).toBe(before)

    await api.__send('มี domain กี่ตัว')
    expect(calls).toHaveLength(1)
    expect(calls[0].url).toBe('https://ai.example.com/api/ai/admin/assistant')
    expect(calls[0].init.headers.Authorization).toBe('Bearer tok-1')
    expect(JSON.parse(calls[0].init.body)).toEqual({ messages: [{ role: 'user', text: 'มี domain กี่ตัว' }], page: '/offices' })
    expect(api.__logText()).toContain('มี 2 domain ครับ')

    // ข้อความถัดไปส่งประวัติไปด้วย และอ่าน token ล่าสุดทุกครั้ง
    token = 'tok-2'
    await api.__send('แล้ว service ล่ะ')
    const body = JSON.parse(calls[1].init.body)
    expect(calls[1].init.headers.Authorization).toBe('Bearer tok-2')
    expect(body.messages.map((m: any) => m.role)).toEqual(['user', 'assistant', 'user'])
    expect(body.messages[1].text).toBe('มี 2 domain ครับ')
  })

  it('server ตอบ error — แสดงข้อความ และไม่เก็บคำถามที่ล้มไว้ในประวัติ', async () => {
    const bodies: any[] = []
    let n = 0
    globalThis.fetch = (async (_u: string, init: any) => {
      bodies.push(JSON.parse(init.body))
      n++
      if (n === 1) return new Response(JSON.stringify({ error: 'ผู้ช่วยในคอนโซลปิดอยู่' }), { status: 503 })
      return sse([['token', { text: 'ok' }], ['done', {}]])
    }) as any
    installConsoleGlobal('')
    const api = (window as any).__aiOfficeConsole as ConsoleAPI
    await api.mount({ getToken: () => 't' })
    await api.__send('ข้อแรก')
    expect(api.__logText()).toContain('ผู้ช่วยในคอนโซลปิดอยู่')
    await api.__send('ข้อสอง')
    expect(bodies[1].messages).toEqual([{ role: 'user', text: 'ข้อสอง' }])
  })
})
