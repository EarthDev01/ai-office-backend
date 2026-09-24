import { hostBase } from './identity'
import type { HostReader } from './pageauth'
import type { PageAuth } from './types'

/** คำขอที่ backend สั่งให้ widget ยิงหลังบ้าน (SSE "fetch") */
export interface FetchOrder {
  id: string
  method: string
  path: string
  query?: Record<string, string> | null
  body?: unknown
  timeout_ms?: number
}

export interface FetchResult {
  status: number
  body: string
}

export type HostFetcher = (o: Pick<FetchOrder, 'method' | 'path' | 'query' | 'body' | 'timeout_ms'>) => Promise<FetchResult>

const MAX_BODY = 2 << 20

function readStore(source: string, key: string): string {
  try {
    const st = source === 'sessionStorage' ? globalThis.sessionStorage : globalThis.localStorage
    return st?.getItem(key) ?? ''
  } catch {
    return ''
  }
}

/**
 * โหมด browser — ยิง API เดิมของหลังบ้านด้วย token ของแอดมินที่ล็อกอินอยู่ (เหมือนที่หน้าเว็บเรียกเอง)
 *
 * ██ ยิงได้เฉพาะ path ภายใต้ host_api_base ของหลังบ้านนี้ (path ต้องขึ้นต้นด้วย / ห้าม //)
 * ██ path มาจาก backend ซึ่งสร้างจาก connector เท่านั้น — ไม่มีทางให้ข้อความของผู้ใช้กลายเป็น URL
 */
export function makeHostFetcher(pa: PageAuth, reader: HostReader, override?: string): HostFetcher {
  return async (o) => {
    const path = String(o.path || '')
    if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\') || /^\/[a-z]+:/i.test(path)) {
      return { status: 400, body: '' }
    }
    const token = reader.readToken(pa)
    if (!token) return { status: 401, body: '' }
    const url = new URL(hostBase(pa, override) + path)
    for (const [k, v] of Object.entries(o.query ?? {})) url.searchParams.set(k, String(v))
    const headers: Record<string, string> = { Accept: 'application/json', Authorization: `${pa.auth_scheme || 'Bearer'} ${token}` }
    for (const h of pa.extra_headers ?? []) {
      const v = h.source === 'token' ? token : readStore(h.source, h.key ?? '')
      if (v) headers[h.name] = v
    }
    const method = o.method === 'POST' ? 'POST' : 'GET'
    let body: string | undefined
    if (method === 'POST') {
      headers['Content-Type'] = 'application/json'
      body = JSON.stringify(o.body ?? {})
    }
    const ac = new AbortController()
    const timer = setTimeout(() => ac.abort(), Math.max(1000, o.timeout_ms ?? 8000))
    try {
      const res = await fetch(url.toString(), { method, headers, body, signal: ac.signal, credentials: 'omit' })
      const text = await res.text()
      if (text.length > MAX_BODY) return { status: 413, body: '' }
      return { status: res.status, body: text }
    } catch {
      return { status: 0, body: '' }
    } finally {
      clearTimeout(timer)
    }
  }
}
