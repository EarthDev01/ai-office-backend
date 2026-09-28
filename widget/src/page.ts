/**
 * อ่าน "ใครล็อกอินอยู่ + เปิดเว็บไหนอยู่" จากหน้าหลังบ้าน ตาม page-config ที่ backend ส่งมา
 *
 * ██ โค้ดนี้ไม่รู้จักหลังบ้านชนิดใด — ชื่อช่อง storage / รูปแบบ token / วิธีหา service
 * ██ มาจาก connectors/<kind>/host.yaml (page_auth) ผ่าน GET /api/ai/widget/page-config เท่านั้น
 */

export interface TokenSource {
  source: string // localStorage | sessionStorage
  key: string
  format?: string // raw | json-expiration
  value_field?: string
  expiration_field?: string
}

export interface ServiceSource {
  source: string // localStorage | sessionStorage | query
  key: string
  encoding?: string // none | base64
}

export interface PluckSpec {
  path: string
  pluck?: string
  where_field?: string
  where_value?: unknown
}

/** สิ่งที่ backend บอกว่าหลังบ้านชนิดนี้เก็บ login ไว้ที่ไหน ยิง API ที่ไหน และอ่านสิทธิ์จากไหน */
export interface PageConfig {
  kind: string
  mode: string
  host_api_base: string
  token: TokenSource
  service: ServiceSource
  auth_scheme?: string
  identity?: {
    root?: string
    /** path ของเส้นหลังบ้านที่คืนรายการสิทธิ์ (template {service}) */
    permissions_request?: string
    /** JWT อีกใบในหน้าที่มีสิทธิ์ติดมา — decode เองแล้วอ่าน permissions.path สัมพัทธ์กับ root */
    permissions_token?: TokenSource
    /** อ่านรหัสสิทธิ์: array ที่ path → เอา field pluck ของ item ที่ where_field == where_value */
    permissions?: PluckSpec
  }
}

// ค่าที่หน้าหลังบ้านเขียนทิ้งไว้ตอนออกจากระบบ (setItem("x", null) = สตริง "null")
const EMPTY = new Set(['', 'null', 'undefined'])

function storage(source: string): Storage | null {
  try {
    if (source === 'localStorage') return localStorage
    if (source === 'sessionStorage') return sessionStorage
  } catch {
    /* storage เข้าไม่ได้ (เช่นปิด cookie) */
  }
  return null
}

/** อ่าน token ตาม TokenSource — ไม่มี / หมดอายุ / พัง = '' (ไม่ throw) */
export function readToken(src: TokenSource | undefined): string {
  if (!src) return ''
  try {
    const raw = storage(src.source)?.getItem(src.key) ?? ''
    if (EMPTY.has(raw.trim())) return ''
    if (src.format !== 'json-expiration') return raw.trim()

    const parsed = JSON.parse(raw) as Record<string, unknown>
    const value = parsed?.[src.value_field || 'value']
    if (typeof value !== 'string' || EMPTY.has(value)) return ''
    const exp = parsed[src.expiration_field || 'expiration']
    // หน่วยวินาที
    if (typeof exp === 'number' && exp <= Math.floor(Date.now() / 1000)) return ''
    return value
  } catch {
    return ''
  }
}

/** service ที่แอดมินเปิดอยู่ — เปลี่ยนได้ตลอดโดยไม่โหลดหน้าใหม่ จึงอ่านสดทุกครั้ง */
export function readService(src: ServiceSource | undefined): string {
  if (!src) return ''
  try {
    let raw = ''
    if (src.source === 'query') {
      raw = new URLSearchParams(location.search).get(src.key) ?? ''
      // SPA แบบ hash router (#/page?service=…)
      if (!raw) {
        const q = location.hash.indexOf('?')
        if (q >= 0) raw = new URLSearchParams(location.hash.slice(q + 1)).get(src.key) ?? ''
      }
    } else {
      raw = storage(src.source)?.getItem(src.key) ?? ''
    }
    raw = raw.trim()
    if (EMPTY.has(raw)) return ''
    if (src.encoding === 'base64') {
      const v = atob(raw).trim()
      return EMPTY.has(v) ? '' : v
    }
    return raw
  } catch {
    return '' // base64 พัง ฯลฯ = ถือว่ายังไม่ได้เลือกเว็บ
  }
}

/** ชื่อที่คนติดตั้งอ่านรู้เรื่อง เช่น localStorage["headertoken"] / ?service= */
export function describeSource(src: TokenSource | ServiceSource | undefined): string {
  if (!src) return '(ไม่ทราบ)'
  return src.source === 'query' ? `?${src.key}=` : `${src.source}["${src.key}"]`
}

/** payload ของ JWT (ไม่ตรวจลายเซ็น) — อ่านไม่ได้ = null */
export function decodeJWT(token: string): unknown {
  const part = token.split('.')[1]
  if (!part) return null
  try {
    const b64 = part.replace(/-/g, '+').replace(/_/g, '/')
    const bin = atob(b64 + '='.repeat((4 - (b64.length % 4)) % 4))
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
    return JSON.parse(new TextDecoder().decode(bytes))
  } catch {
    return null
  }
}

/** ดึงรายการรหัสจาก object ตาม PluckSpec */
export function pluck(v: unknown, spec: PluckSpec): string[] {
  const list = dig(v, spec.path)
  if (!Array.isArray(list)) return []
  const out: string[] = []
  for (const item of list) {
    if (spec.where_field && dig(item, spec.where_field) !== spec.where_value) continue
    const code = spec.pluck ? dig(item, spec.pluck) : item
    if (typeof code === 'string' && code) out.push(code)
  }
  return out
}

export function dig(v: unknown, path: string | undefined): unknown {
  let cur = v
  if (!path) return cur
  for (const k of path.split('.')) {
    if (!cur || typeof cur !== 'object') return undefined
    cur = (cur as Record<string, unknown>)[k]
  }
  return cur
}
