import type { HostUser, IdentitySpec, PageAuth } from './types'

/**
 * โหมด browser — อ่าน "ใครล็อกอินอยู่" จากหน้าหลังบ้าน ตาม identity ของ connector
 *
 * ██ ไม่ได้ตรวจลายเซ็น (ทำไม่ได้ฝั่ง browser) — ข้อมูลจริงยังถูกหลังบ้านคุมเพราะ widget ยิงด้วย token ของแอดมินเอง
 * ██ ใช้แค่เพื่อ ติดชื่อประวัติ / allowlist / กรองเครื่องมือตามสิทธิ์ให้ตอบตรงกับที่หลังบ้านจะยอม
 */
export function getPath(obj: unknown, path: string | undefined): unknown {
  if (!path) return obj
  let cur: unknown = obj
  for (const part of path.split('.')) {
    if (cur == null || typeof cur !== 'object') return undefined
    cur = (cur as Record<string, unknown>)[part]
  }
  return cur
}

export function decodeJwtPayload(token: string): unknown {
  const parts = token.split('.')
  if (parts.length < 2) return null
  try {
    const b64 = parts[1].replace(/-/g, '+').replace(/_/g, '/')
    const pad = b64 + '==='.slice((b64.length + 3) % 4)
    const bin = atob(pad)
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0))
    return JSON.parse(new TextDecoder().decode(bytes))
  } catch {
    return null
  }
}

export function mapUser(spec: IdentitySpec, src: unknown): HostUser | null {
  const root = getPath(src, spec.root)
  const str = (p?: string) => {
    if (!p) return ''
    const v = getPath(root, p)
    return v == null || typeof v === 'object' ? '' : String(v)
  }
  const id = str(spec.id)
  const username = str(spec.username)
  if (!id || !username) return null
  const user: HostUser = { id, username, display_name: str(spec.display_name) || username }
  const lvl = spec.level ? Number(getPath(root, spec.level)) : NaN
  if (spec.level && Number.isFinite(lvl)) user.level = lvl
  if (spec.dept) user.dept = str(spec.dept)
  if (spec.permissions) {
    const list = getPath(root, spec.permissions.path)
    const out: string[] = []
    if (Array.isArray(list)) {
      for (const it of list) {
        const w = spec.permissions.where_field
        if (w && String(getPath(it, w)) !== String(spec.permissions.where_value)) continue
        const code = spec.permissions.pluck ? getPath(it, spec.permissions.pluck) : it
        if (typeof code === 'string' && code) out.push(code)
      }
    }
    user.permissions = [...new Set(out)].slice(0, 500)
  }
  return user
}

/** ลายนิ้วมือ token (sha256 hex) — ส่งแค่นี้ ไม่ส่ง token ไป backend */
export async function tokenFingerprint(token: string): Promise<string> {
  const subtle = (globalThis.crypto as Crypto | undefined)?.subtle
  if (!subtle) return ''
  const buf = await subtle.digest('SHA-256', new TextEncoder().encode(token))
  return Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, '0')).join('')
}

/** ที่อยู่ API ของหลังบ้าน (base) · snippet ทับได้ด้วย data-host-api-base */
export function hostBase(pa: PageAuth, override?: string): string {
  return (override || pa.host_api_base).replace('{origin}', location.origin).replace(/\/+$/, '')
}
