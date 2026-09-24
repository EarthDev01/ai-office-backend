import type { PageAuth } from './types'

/**
 * ตัวอ่านหน้าแบบกลาง — อ่าน "ใครล็อกอินอยู่ + เปิดเว็บไหนอยู่" ตาม page_auth ของ connector
 *
 * ██ ห้ามใส่ชื่อ localStorage ของหลังบ้านใดตรงนี้ (กฎปลั๊ก spec §5.3)
 * ██ ชื่อช่อง/รูปแบบมาจาก connectors/<kind>/host.yaml เท่านั้น
 *
 * หลังบ้านที่มีวิธีเก็บแปลกจนตั้งค่าไม่ได้ → เพิ่มไฟล์ใน hosts/<kind>.ts แล้วเรียก registerHostReader
 */
export interface HostReader {
  readToken(pa: PageAuth): string
  readService(pa: PageAuth): string
}

const custom: Record<string, HostReader> = {}

export function registerHostReader(kind: string, reader: HostReader): void {
  custom[kind] = reader
}

function storage(source: string): Storage | null {
  try {
    const g = globalThis as unknown as Record<string, Storage | undefined>
    return (source === 'sessionStorage' ? g.sessionStorage : g.localStorage) ?? null
  } catch {
    return null
  }
}

export const genericReader: HostReader = {
  readToken(pa) {
    try {
      const raw = storage(pa.token.source)?.getItem(pa.token.key)
      if (!raw || raw === 'null' || raw === 'undefined') return ''
      if (pa.token.format === 'raw') return raw.trim()
      const parsed = JSON.parse(raw) as Record<string, unknown>
      const value = parsed?.[pa.token.value_field ?? 'value']
      if (typeof value !== 'string' || !value) return ''
      const exp = parsed?.[pa.token.expiration_field ?? 'expiration']
      // เวลาหมดอายุอาจเป็นวินาทีหรือมิลลิวินาที
      if (typeof exp === 'number') {
        const expSec = exp > 1e12 ? exp / 1000 : exp
        if (expSec <= Date.now() / 1000) return ''
      }
      return value
    } catch {
      return ''
    }
  },
  readService(pa) {
    try {
      let v = ''
      if (pa.service.source === 'query') {
        v = new URLSearchParams(location.search).get(pa.service.key) ?? ''
      } else {
        v = storage(pa.service.source)?.getItem(pa.service.key) ?? ''
      }
      v = v.trim()
      if (!v || v === 'null') return ''
      if (pa.service.encoding === 'base64') {
        try {
          v = atob(v)
        } catch {
          return ''
        }
      }
      // service id ต้องเป็นรูปแบบปกติ — กันค่าประหลาดไปต่อ URL
      return /^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$/.test(v) ? v : ''
    } catch {
      return ''
    }
  },
}

export function readerFor(kind: string | undefined): HostReader {
  return (kind && custom[kind]) || genericReader
}

/** URL ของ host สำหรับขอตั๋ว: {base}{session_path} · base ทับได้จาก snippet */
export function hostSessionURL(pa: PageAuth, service: string, override?: string): string {
  const base = (override || pa.host_api_base).replace('{origin}', location.origin).replace(/\/+$/, '')
  return base + pa.session_path.replace('{service}', encodeURIComponent(service))
}
