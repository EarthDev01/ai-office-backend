import { hostFetch } from './hostfetch'

/** สิ่งที่ backend บอกว่าหลังบ้านชนิดนี้ยิง API ที่ไหน และอ่านสิทธิ์จากเส้นไหน */
export interface PageConfig {
  kind: string
  mode: string
  host_api_base: string
  identity?: {
    /** path ของเส้นหลังบ้านที่คืนรายการสิทธิ์ (template {service}) */
    permissions_request?: string
    /** อ่านรหัสสิทธิ์จาก response: array ที่ path → เอา field pluck ของ item ที่ where_field == where_value */
    permissions?: { path: string; pluck?: string; where_field?: string; where_value?: unknown }
  }
}

// ตั๋วที่เหลืออายุน้อยกว่านี้ขอใหม่ก่อนเริ่มแชท — คำตอบ 1 ข้อ (รวมรอ relay) ต้องใช้ตั๋วใบเดิมจนจบ
const RENEW_BEFORE_MS = 3 * 60 * 1000

/**
 * ตั๋วแชทของหน้านี้ — ขอด้วย token ของหน้า office แล้วใช้ตั๋วแทนในทุกคำขอแชท
 *
 * สิทธิ์ของแอดมินอ่านจากเส้นของหลังบ้านเอง (JWT ของ office-v10x ไม่มีสิทธิ์ติดมา)
 * ส่งไปแค่รหัสสิทธิ์ ไม่ส่ง token ซ้ำ
 */
export class ChatSession {
  private cfg: PageConfig | null = null
  private cur: { service: string; ticket: string; expiresAt: number } | null = null

  constructor(
    private apiBase: string,
    private readToken: () => string,
  ) {}

  /** ที่อยู่ API หลังบ้าน — backend ตัดสินจากโดเมนของหน้านี้ (ค่าที่ตั้งในคอนโซล หรือ {origin}/api) */
  hostApiBase(): string {
    return this.cfg?.host_api_base || ''
  }

  async pageConfig(): Promise<PageConfig> {
    if (this.cfg) return this.cfg
    const res = await fetch(`${this.apiBase}/api/ai/widget/page-config`)
    const json = (await res.json().catch(() => null)) as { payload?: PageConfig; message?: string } | null
    if (!res.ok || !json?.payload) throw new Error(`โหลดการตั้งค่าไม่สำเร็จ (${json?.message ?? res.status})`)
    this.cfg = json.payload
    return this.cfg
  }

  /** ตั๋วที่ยังเหลืออายุพอสำหรับคำตอบ 1 ข้อ */
  async ticket(service: string): Promise<string> {
    if (this.cur && this.cur.service === service && this.cur.expiresAt - Date.now() > RENEW_BEFORE_MS) {
      return this.cur.ticket
    }
    const cfg = await this.pageConfig()
    const permissions = await this.readPermissions(cfg, service)
    const res = await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(service)}/browser-session`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${this.readToken()}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ permissions }),
    })
    const json = (await res.json().catch(() => null)) as
      | { payload?: { ticket: string; expires_in: number }; message?: string; error?: string }
      | null
    if (!res.ok || !json?.payload?.ticket) {
      throw new Error(json?.error || `ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${res.status})`)
    }
    this.cur = { service, ticket: json.payload.ticket, expiresAt: Date.now() + json.payload.expires_in * 1000 }
    return this.cur.ticket
  }

  /** ตั๋วใช้ไม่ได้แล้ว (401) — ทิ้งไปให้ขอใหม่รอบหน้า */
  invalidate() {
    this.cur = null
  }

  private async readPermissions(cfg: PageConfig, service: string): Promise<string[]> {
    const req = cfg.identity?.permissions_request
    const spec = cfg.identity?.permissions
    if (!req || !spec?.path) return []
    const res = await hostFetch(
      this.hostApiBase(),
      { id: 'perm', method: 'GET', path: req.split('{service}').join(encodeURIComponent(service)) },
      this.readToken(),
    )
    if (res.status !== 200) {
      console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${res.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`)
      return []
    }
    try {
      const list = dig(JSON.parse(res.body), spec.path)
      if (!Array.isArray(list)) return []
      const out: string[] = []
      for (const item of list) {
        if (spec.where_field && dig(item, spec.where_field) !== spec.where_value) continue
        const code = spec.pluck ? dig(item, spec.pluck) : item
        if (typeof code === 'string' && code) out.push(code)
      }
      return out
    } catch {
      return []
    }
  }
}

function dig(v: unknown, path: string): unknown {
  let cur = v
  for (const k of path.split('.')) {
    if (!cur || typeof cur !== 'object') return undefined
    cur = (cur as Record<string, unknown>)[k]
  }
  return cur
}
