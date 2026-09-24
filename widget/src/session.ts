import type { Api, Result } from './api'
import type { HostFetcher } from './hostfetch'
import { decodeJwtPayload, identityProblem, mapUser, tokenFingerprint } from './identity'
import { hostSessionURL, type HostReader } from './pageauth'
import type { PageAuth } from './types'

/** ทำไมขอตั๋วไม่ได้ — ใช้เลือกข้อความที่ผู้ใช้เห็น (spec §8) */
export type SessionFailure = 'login' | 'refused' | 'unavailable'

export type TicketResult =
  | { ok: true; ticket: string }
  | { ok: false; why: SessionFailure; code: string; reason: string }

const RENEW_BEFORE_SEC = 60

const now = () => Math.floor(Date.now() / 1000)

/**
 * ตั๋วของ 1 service — ขอจาก host ด้วย token ผู้ใช้ของ host เอง แล้วต่ออายุเงียบ ๆ ก่อนหมด
 * ตั๋วผูก service ตัวเดียว: เปลี่ยน service = สร้าง SessionManager ใหม่
 */
export class SessionManager {
  private cur: { ticket: string; exp: number } | null = null
  private inflight: Promise<TicketResult> | null = null

  constructor(
    private api: Api,
    readonly service: string,
    private pa: PageAuth | null,
    private reader: HostReader | null,
    private hostBase?: string,
    /** โหมด browser: widget ขอตั๋วเองจาก backend (ไม่มี host ออกให้) */
    private browser?: { fetcher: HostFetcher },
  ) {}

  /** ใช้ใน test / preview: ใส่ตั๋วที่มีอยู่แล้ว */
  seed(ticket: string, exp: number) {
    this.cur = { ticket, exp }
  }

  /** ตั๋วล่าสุดที่ถืออยู่ (ใช้ปิดห้องแบบ best effort แม้ใกล้หมดอายุ) */
  current(): string {
    return this.cur?.ticket ?? ''
  }

  expiresAt(): number {
    return this.cur?.exp ?? 0
  }

  drop() {
    this.cur = null
  }

  async get(force = false): Promise<TicketResult> {
    if (!force && this.cur && this.cur.exp - now() > RENEW_BEFORE_SEC) return { ok: true, ticket: this.cur.ticket }
    if (!this.inflight) {
      this.inflight = this.renew().finally(() => {
        this.inflight = null
      })
    }
    return this.inflight
  }

  /** โหมด browser: อ่านตัวตน → ลายนิ้วมือ token → ขอตั๋วจาก backend */
  private async browserSession(token: string) {
    const spec = this.pa!.identity
    if (!spec) return null
    let src: unknown = null
    if (spec.source === 'jwt') {
      src = decodeJwtPayload(token)
    } else {
      const res = await this.browser!.fetcher({ method: spec.method || 'GET', path: (spec.path || '').replace('{service}', encodeURIComponent(this.service)) })
      if (res.status === 401 || res.status === 403) return null
      try {
        src = JSON.parse(res.body)
      } catch {
        src = null
      }
    }
    const user = src ? mapUser(spec, src) : null
    const fp = await tokenFingerprint(token)
    if (!user || !fp) return null
    return this.api.browserSession(this.service, user, fp)
  }

  private async renew(): Promise<TicketResult> {
    if (!this.pa || !this.reader) return { ok: false, why: 'login', code: 'NO_PAGE_AUTH', reason: '' }
    const token = this.reader.readToken(this.pa)
    if (!token) return { ok: false, why: 'login', code: 'NO_TOKEN', reason: '' }
    const r = this.browser ? await this.browserSession(token) : await this.api.hostSession(hostSessionURL(this.pa, this.service, this.hostBase), this.pa.auth_scheme || 'Bearer', token)
    if (!r) return { ok: false, why: 'login', code: 'NO_IDENTITY', reason: identityProblem }
    if (r.ok && r.data?.ticket) {
      const exp = typeof r.data.expires_at === 'number' && r.data.expires_at > 0 ? r.data.expires_at : now() + 300
      this.cur = { ticket: r.data.ticket, exp }
      return { ok: true, ticket: r.data.ticket }
    }
    this.cur = null
    const reason = String((r.data as { reason?: string } | null)?.reason ?? '')
    return { ok: false, why: classify(r, reason), code: r.code, reason }
  }
}

export function classify(r: Result<unknown>, reason: string): SessionFailure {
  if (r.status === 0 || r.status >= 500) return 'unavailable'
  if (r.status === 429) return 'unavailable'
  // middleware ตรวจผู้ใช้เดิมของ host ตอบเองก่อนถึง handler: HTTP 200 ไม่มี payload
  // (เช่น {"error":"unauthorized"} · {"code":401,"msg":…} · {"code":4011,…}) = token ของ host ใช้ไม่ได้แล้ว
  if (r.status === 401 || r.bodyCode === 401 || r.status === 200) return 'login'
  if (reason === 'expired' || reason === 'no_user') return 'login'
  return 'refused'
}
