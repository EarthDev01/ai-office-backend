import { apiError, ChatError } from './chat'
import { hostFetch } from './hostfetch'
import { decodeJWT, dig, guestID, pluck, readToken, type PageConfig } from './page'

export type { PageConfig } from './page'

// ตั๋วที่เหลืออายุน้อยกว่านี้ขอใหม่ก่อนเริ่มแชท — คำตอบ 1 ข้อ (รวมรอ relay) ต้องใช้ตั๋วใบเดิมจนจบ
const RENEW_BEFORE_MS = 3 * 60 * 1000

/**
 * ตั๋วแชทของหน้านี้ — ขอด้วย token ของหน้า office แล้วใช้ตั๋วแทนในทุกคำขอแชท
 *
 * สิทธิ์ของแอดมินอ่านตามที่ connector บอก (เส้นสิทธิ์ของหลังบ้าน หรือ JWT อีกใบในหน้า)
 * ส่งไปแค่รหัสสิทธิ์ ไม่ส่ง token ซ้ำ
 */
export class ChatSession {
  private cfg: PageConfig | null = null
  private loading: Promise<PageConfig> | null = null
  private cur: { service: string; cred: string; ticket: string; expiresAt: number } | null = null

  /** hostAPIBase = ค่าจาก data-host-api-base ของ snippet (ทับค่าจาก backend) */
  constructor(
    private apiBase: string,
    private hostAPIBase = '',
  ) {}

  /** ที่อยู่ API หลังบ้าน — snippet ระบุเอง หรือ backend ตัดสินจากโดเมนของหน้านี้ (ค่าที่ตั้งในคอนโซล / {origin}/api) */
  hostApiBase(): string {
    return this.hostAPIBase || this.cfg?.host_api_base || ''
  }

  /** token ของแอดมินในหน้านี้ ตามที่ page-config บอก ('' = ยังไม่รู้/ยังไม่ล็อกอิน) */
  readToken(): string {
    return readToken(this.cfg?.token)
  }

  /**
   * ตัวตนที่ใช้ขอตั๋ว: token ของหน้า (Bearer) · ยังไม่ล็อกอินบนหน้าเว็บที่เปิด guest = guest_id ของเบราว์เซอร์ (Guest)
   * '' = ยังไม่รู้/ยังไม่ล็อกอินและหน้านี้ไม่เปิด guest
   */
  credential(): string {
    const token = this.readToken()
    if (token) return `Bearer ${token}`
    return this.cfg?.guest ? `Guest ${guestID()}` : ''
  }

  isGuest(): boolean {
    return this.credential().startsWith('Guest ')
  }

  authScheme(): string {
    return this.cfg?.auth_scheme || 'Bearer'
  }

  /** โหลดครั้งเดียวต่อหน้า · พลาดแล้วลองใหม่ได้ในรอบถัดไป */
  async pageConfig(): Promise<PageConfig> {
    if (this.cfg) return this.cfg
    this.loading ??= (async () => {
      const res = await fetch(`${this.apiBase}/api/ai/widget/page-config`)
      const json = (await res.json().catch(() => null)) as { payload?: PageConfig; message?: string } | null
      if (!res.ok || !json?.payload) throw new PageConfigError(json?.message ?? String(res.status))
      this.cfg = json.payload
      return this.cfg
    })().finally(() => {
      this.loading = null
    })
    return this.loading
  }

  /** ตั๋วที่ยังเหลืออายุพอสำหรับคำตอบ 1 ข้อ */
  async ticket(service: string): Promise<string> {
    const cfg = await this.pageConfig()
    // ตั๋วผูกกับตัวตนที่ใช้ขอ — ล็อกอินจาก guest แล้วต้องขอใหม่ ห้ามใช้ตั๋ว guest ต่อ
    const cred = this.credential()
    if (this.cur && this.cur.service === service && this.cur.cred === cred && this.cur.expiresAt - Date.now() > RENEW_BEFORE_MS) {
      return this.cur.ticket
    }
    const permissions = cred.startsWith('Guest ') ? [] : await this.readPermissions(cfg, service)
    const res = await fetch(`${this.apiBase}/api/ai/widget/service/${encodeURIComponent(service)}/browser-session`, {
      method: 'POST',
      headers: { Authorization: cred, 'Content-Type': 'application/json' },
      body: JSON.stringify({ permissions }),
    })
    if (!res.ok) throw await apiError(res, `ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${res.status})`)
    const json = (await res.json().catch(() => null)) as { payload?: { ticket: string; expires_in: number } } | null
    if (!json?.payload?.ticket) throw new ChatError(`ขอสิทธิ์ใช้งานผู้ช่วยไม่สำเร็จ (${res.status})`)
    this.cur = { service, cred, ticket: json.payload.ticket, expiresAt: Date.now() + json.payload.expires_in * 1000 }
    return this.cur.ticket
  }

  /** ตั๋วใช้ไม่ได้แล้ว (401) — ทิ้งไปให้ขอใหม่รอบหน้า */
  invalidate() {
    this.cur = null
  }

  private async readPermissions(cfg: PageConfig, service: string): Promise<string[]> {
    const id = cfg.identity
    const spec = id?.permissions
    if (!spec?.path) return []

    // สิทธิ์ติดมากับ JWT อีกใบในหน้า — ไม่ต้องยิงหลังบ้าน
    if (id?.permissions_token) {
      const tok = readToken(id.permissions_token)
      return tok ? pluck(dig(decodeJWT(tok), id.root), spec) : []
    }

    const req = id?.permissions_request
    if (!req) return []
    const res = await hostFetch(
      this.hostApiBase(),
      { id: 'perm', method: 'GET', path: req.split('{service}').join(encodeURIComponent(service)) },
      this.readToken(),
      this.authScheme(),
    )
    if (res.status !== 200) {
      console.info(`[ai-office] อ่านสิทธิ์จากหลังบ้านไม่สำเร็จ (HTTP ${res.status}) — ใช้ต่อได้แต่ข้อมูลที่ต้องมีสิทธิ์จะถูกกั้น`)
      return []
    }
    try {
      return pluck(JSON.parse(res.body), spec)
    } catch {
      return []
    }
  }
}

/** โหลด page-config ไม่สำเร็จ — message = รหัสจาก server (เช่น ORIGIN_NOT_REGISTERED) หรือ HTTP status */
export class PageConfigError extends Error {}
