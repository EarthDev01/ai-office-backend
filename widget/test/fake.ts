/**
 * fetch ปลอม — จับทุก request แล้วตอบตาม route ที่ตั้งไว้
 * ตอบ SSE ได้ด้วย (body.getReader แบบค่อย ๆ ปล่อยทีละ block · abort ได้)
 */
export interface Call {
  url: string
  method: string
  headers: Record<string, string>
  body: string
}

export type Handler = (c: Call, n: number) => FakeResponse | Promise<FakeResponse>

export interface FakeResponse {
  status?: number
  json?: unknown
  sse?: string[] // แต่ละตัว = 1 block "event: x\ndata: {...}"
  /** ค้าง stream ไว้หลังส่ง block ทั้งหมด (จนกว่าจะ abort) */
  hang?: boolean
  throws?: boolean
}

export function ev(type: string, data: unknown): string {
  return `event: ${type}\ndata: ${JSON.stringify(data)}`
}

export function installFetch(routes: Array<[RegExp, Handler]>, target: any = globalThis) {
  const calls: Call[] = []
  const counts = new Map<RegExp, number>()
  const orig = target.fetch
  target.fetch = async (input: any, init: any = {}) => {
    const url = String(input)
    const headers: Record<string, string> = {}
    for (const [k, v] of Object.entries(init.headers ?? {})) headers[k.toLowerCase()] = String(v)
    const call: Call = { url, method: (init.method ?? 'GET').toUpperCase(), headers, body: typeof init.body === 'string' ? init.body : '' }
    calls.push(call)
    const hit = routes.find(([re]) => re.test(url))
    if (!hit) throw new Error('no route: ' + url)
    const n = (counts.get(hit[0]) ?? 0) + 1
    counts.set(hit[0], n)
    const r = await hit[1](call, n)
    if (r.throws) throw new TypeError('Failed to fetch')
    return respond(r, init.signal as AbortSignal | undefined)
  }
  return {
    calls,
    restore: () => {
      target.fetch = orig
    },
    to: (re: RegExp) => calls.filter((c) => re.test(c.url)),
  }
}

function respond(r: FakeResponse, signal?: AbortSignal) {
  const status = r.status ?? 200
  if (r.sse) {
    const enc = new TextEncoder()
    const chunks = r.sse.map((b) => enc.encode(b + '\n\n'))
    let i = 0
    return {
      ok: status < 400,
      status,
      headers: { get: (k: string) => (k.toLowerCase() === 'content-type' ? 'text/event-stream; charset=utf-8' : null) },
      json: async () => null,
      body: {
        getReader: () => ({
          read: () =>
            new Promise((resolve, reject) => {
              const abortErr = () => {
                const e = new Error('aborted')
                e.name = 'AbortError'
                reject(e)
              }
              if (signal?.aborted) return abortErr()
              if (i < chunks.length) {
                const v = chunks[i++]
                setTimeout(() => (signal?.aborted ? abortErr() : resolve({ value: v, done: false })), 1)
                return
              }
              if (r.hang) {
                signal?.addEventListener('abort', abortErr)
                return
              }
              resolve({ value: undefined, done: true })
            }),
        }),
      },
    }
  }
  return {
    ok: status < 400,
    status,
    headers: { get: (k: string) => (k.toLowerCase() === 'content-type' ? 'application/json' : null) },
    json: async () => r.json ?? null,
    text: async () => (r.json === undefined ? '' : JSON.stringify(r.json)),
    body: null,
  }
}

export const tick = (ms = 0) => new Promise((r) => setTimeout(r, ms))

export async function until(fn: () => boolean, ms = 1500) {
  const end = Date.now() + ms
  while (Date.now() < end) {
    if (fn()) return
    await tick(5)
  }
  throw new Error('timeout waiting for condition')
}

export const PAGE_AUTH = {
  token: { source: 'localStorage', key: 'tok', format: 'json-expiration', value_field: 'value', expiration_field: 'expiration' },
  service: { source: 'localStorage', key: 'svc', encoding: 'none' },
  host_api_base: '{origin}/api',
  session_path: '/ai/session/{service}',
  auth_scheme: 'Bearer',
}

export const BOOT = {
  enabled: true,
  office_id: 'demo',
  service_id: 'K11S',
  service_label: 'เว็บ K11S',
  kind: 'sample-kind',
  is_hidden: false,
  avatar_url: '',
  display_name: 'ผู้ช่วยหลังบ้าน',
  greeting: 'สวัสดีครับ',
  theme: 'light',
  placement: { position: 'bottom-right', offset_x: 12, offset_y: 12 },
}

export function login(store: Storage, value = 'host-user-token', service = 'K11S') {
  store.setItem('tok', JSON.stringify({ value, expiration: Math.floor(Date.now() / 1000) + 600 }))
  if (service) store.setItem('svc', service)
}

/** route มาตรฐาน: page-config · host session · bootstrap · chat · history */
export function standardRoutes(over: Partial<Record<'config' | 'session' | 'bootstrap' | 'chat' | 'list' | 'conv' | 'close', Handler>> = {}) {
  let ticketN = 0
  const routes: Array<[RegExp, Handler]> = [
    [/\/page-config$/, over.config ?? (() => ({ json: { payload: { enabled: true, kind: 'sample-kind', page_auth: PAGE_AUTH } } }))],
    [
      /\/api\/ai\/session\//,
      over.session ??
        ((c) => {
          ticketN++
          const svc = c.url.split('/').pop()
          return { json: { code: 200, message: 'SUCCESS', payload: { ticket: `tk-${svc}-${ticketN}`, expires_at: Math.floor(Date.now() / 1000) + 600 } } }
        }),
    ],
    [
      /\/bootstrap$/,
      over.bootstrap ??
        ((c) => {
          const svc = c.url.split('/service/')[1].split('/')[0]
          return { json: { payload: { ...BOOT, service_id: svc, service_label: 'เว็บ ' + svc } } }
        }),
    ],
    [/\/chat$/, over.chat ?? (() => ({ sse: [ev('status', { phase: 'thinking', text: 'กำลังตรวจสอบคำถาม…', conversation_id: 'c1' }), ev('token', { text: 'สวัสดี' }), ev('done', { conversation_id: 'c1', message_id: 'm1' })] }))],
    [/\/conversations\?days=7$/, over.list ?? (() => ({ json: { payload: { data: [] } } }))],
    [/\/close$/, over.close ?? (() => ({ json: { payload: null } }))],
    [/\/conversations\/[^/?]+$/, over.conv ?? (() => ({ status: 404, json: { message: 'NOT_FOUND' } }))],
  ]
  return routes
}
