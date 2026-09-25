/**
 * ยิง API ของหลังบ้านตามคำสั่ง fetch จาก backend — ใช้ token ของแอดมินที่ล็อกอินอยู่ในหน้านี้
 *
 * backend ไม่เคยได้ token นี้ไปยิงเอง · คำสั่งที่รับมาจึงต้องถูกกรองที่นี่อีกชั้น:
 *   - path ต้องขึ้นต้น / · ห้าม //, \, scheme: · ต่อท้าย host_api_base เท่านั้น (ห้ามหลุดออกนอก base)
 *   - method ได้แค่ GET/POST · ไม่ส่ง cookie · ผลใหญ่เกิน 2 MB ไม่ส่งกลับ
 */

export const HOST_MAX_BODY = 2 * 1024 * 1024

export interface FetchCommand {
  id: string
  method: string
  path: string
  query?: Record<string, string>
  body?: unknown
  timeout_ms?: number
}

export interface HostResult {
  status: number // 0 = network error / ถูกปฏิเสธฝั่ง widget
  body: string
}

/** สร้าง URL เต็ม — คืน null ถ้าคำสั่งไม่ผ่านกติกา */
export function buildHostURL(base: string, path: string, query?: Record<string, string>): string | null {
  if (!base || typeof path !== 'string') return null
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\') || /^[a-z][a-z0-9+.-]*:/i.test(path) || path.includes('://')) {
    return null
  }
  let baseURL: URL
  try {
    baseURL = new URL(base)
  } catch {
    return null
  }
  const basePath = baseURL.pathname.replace(/\/+$/, '')
  let url: URL
  try {
    url = new URL(baseURL.origin + basePath + path)
  } catch {
    return null
  }
  // /../ ถูก normalize แล้ว — ต้องยังอยู่ใต้ base เสมอ
  if (url.origin !== baseURL.origin || !(url.pathname === basePath || url.pathname.startsWith(basePath + '/'))) return null
  if (query) {
    for (const [k, v] of Object.entries(query)) url.searchParams.set(k, String(v))
  }
  return url.toString()
}

export async function hostFetch(base: string, cmd: FetchCommand, token: string): Promise<HostResult> {
  if (!token) return { status: 401, body: '' }
  const method = String(cmd.method || '').toUpperCase()
  if (method !== 'GET' && method !== 'POST') return { status: 0, body: '' }
  const url = buildHostURL(base, cmd.path, cmd.query)
  if (!url) return { status: 0, body: '' }

  const ctrl = new AbortController()
  const timer = setTimeout(() => ctrl.abort(), Math.max(1000, cmd.timeout_ms ?? 20000))
  try {
    const init: RequestInit = {
      method,
      credentials: 'omit',
      signal: ctrl.signal,
      headers: { Authorization: `Bearer ${token}`, Accept: 'application/json' } as Record<string, string>,
    }
    if (method === 'POST' && cmd.body !== undefined && cmd.body !== null) {
      ;(init.headers as Record<string, string>)['Content-Type'] = 'application/json'
      init.body = JSON.stringify(cmd.body)
    }
    const res = await fetch(url, init)
    const text = await res.text()
    if (text.length > HOST_MAX_BODY) return { status: 0, body: '' }
    return { status: res.status, body: text }
  } catch {
    return { status: 0, body: '' }
  } finally {
    clearTimeout(timer)
  }
}
