import type { Bootstrap, Card, HistoryItem, HistoryMessage, HostUser, PageConfig } from './types'

/** ผลของการเรียก API · ไม่ throw (widget พังต้องไม่ลากหน้า office พังตาม) */
export interface Result<T> {
  ok: boolean
  status: number
  code: string
  /** code ตัวเลขใน body (host บางตัวตอบ HTTP 200 แต่ body code 401) */
  bodyCode?: number
  error: string
  data: T | null
}

async function call<T>(url: string, init: RequestInit): Promise<Result<T>> {
  try {
    const res = await fetch(url, init)
    const json = (await res.json().catch(() => null)) as
      | { message?: string; error?: string; payload?: T; code?: number | string; msg?: string }
      | null
    const code = String(json?.message ?? json?.msg ?? (res.ok ? 'SUCCESS' : res.status))
    const bodyCode = typeof json?.code === 'number' ? json.code : undefined
    return {
      ok: res.ok && json?.payload !== undefined && json?.payload !== null,
      status: res.status,
      code,
      bodyCode,
      error: json?.error ?? '',
      data: (json?.payload ?? null) as T | null,
    }
  } catch (e) {
    return { ok: false, status: 0, code: 'NETWORK', error: (e as Error).message, data: null }
  }
}

export class Api {
  constructor(
    private apiBase: string,
    private publicKey: string,
  ) {}

  private officePath(service: string) {
    return `${this.apiBase}/api/ai/office/${encodeURIComponent(this.publicKey)}/service/${encodeURIComponent(service)}`
  }

  pageConfigURL(): string {
    return `${this.apiBase}/api/ai/office/${encodeURIComponent(this.publicKey)}/page-config`
  }

  pageConfig(): Promise<Result<PageConfig>> {
    return call<PageConfig>(this.pageConfigURL(), { method: 'GET' })
  }

  /**
   * ขอตั๋วจาก host ด้วย token ผู้ใช้ของ host เอง (D-87)
   * backend ไม่เคยเห็น token นี้ — host ตรวจเองแล้วส่งตัวตนที่ยืนยันแล้วไปขอตั๋วแทน
   */
  hostSession(url: string, scheme: string, token: string): Promise<Result<{ ticket: string; expires_at: number }>> {
    return call(url, {
      method: 'POST',
      headers: { Authorization: `${scheme} ${token}`, 'Content-Type': 'application/json' },
      body: '{}',
      credentials: 'omit',
    })
  }

  /** โหมด browser: ขอตั๋วเอง (ไม่มี host ออกให้) · ส่งแค่ตัวตน + ลายนิ้วมือ token ไม่ส่ง token */
  browserSession(service: string, user: HostUser, tokenFp: string): Promise<Result<{ ticket: string; expires_at: number }>> {
    return call(this.officePath(service) + '/browser-session', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ user, token_fp: tokenFp }),
    })
  }

  /** โหมด browser: ส่งผลที่ยิงหลังบ้านได้กลับไปให้ backend */
  relay(service: string, ticket: string, id: string, status: number, body: string): Promise<Result<null>> {
    return call(this.officePath(service) + '/chat/relay/' + encodeURIComponent(id), {
      method: 'POST',
      headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ status, body }),
    })
  }

  bootstrapURL(service: string) {
    return this.officePath(service) + '/bootstrap'
  }

  bootstrap(service: string, ticket: string): Promise<Result<Bootstrap>> {
    return call<Bootstrap>(this.bootstrapURL(service), { headers: { Authorization: `Bearer ${ticket}` } })
  }

  history(service: string, ticket: string): Promise<Result<{ data: HistoryItem[] }>> {
    return call(this.officePath(service) + '/conversations?days=7', { headers: { Authorization: `Bearer ${ticket}` } })
  }

  conversation(service: string, ticket: string, id: string): Promise<Result<{ messages: HistoryMessage[]; conversation: { id: string; closed: boolean } }>> {
    return call(this.officePath(service) + '/conversations/' + encodeURIComponent(id), { headers: { Authorization: `Bearer ${ticket}` } })
  }

  close(service: string, ticket: string, id: string, reason: string): void {
    // best effort — keepalive ให้ส่งได้แม้หน้ากำลังเปลี่ยน
    void fetch(this.officePath(service) + '/conversations/' + encodeURIComponent(id) + '/close', {
      method: 'POST',
      headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ reason }),
      keepalive: true,
    }).catch(() => {})
  }

  chatURL(service: string) {
    return this.officePath(service) + '/chat'
  }
}

export type ChatEvent =
  | { type: 'status'; data: { phase: string; text: string; conversation_id?: string } }
  | { type: 'card'; data: Card }
  | { type: 'fetch'; data: import('./hostfetch').FetchOrder }
  | { type: 'token'; data: { text: string } }
  | { type: 'done'; data: { message_id: string; conversation_id: string; latency_ms: number } }
  | { type: 'error'; data: { code: string; message: string } }

/**
 * ยิง /chat แล้วอ่าน SSE จาก fetch (EventSource ส่ง POST ไม่ได้)
 * คืน status ของ HTTP ถ้าไม่ใช่ stream (เช่น 401 ตั๋วหมดอายุ) ให้ผู้เรียกต่ออายุแล้วลองใหม่
 */
export async function streamChat(
  url: string,
  ticket: string,
  body: { text: string; conversation_id?: string },
  onEvent: (e: ChatEvent) => void,
  signal: AbortSignal,
): Promise<{ status: number; code: string }> {
  let res: Response
  try {
    res = await fetch(url, {
      method: 'POST',
      headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json', Accept: 'text/event-stream' },
      body: JSON.stringify(body),
      signal,
    })
  } catch (e) {
    if ((e as Error).name === 'AbortError') return { status: -1, code: 'ABORTED' }
    return { status: 0, code: 'NETWORK' }
  }
  const ct = res.headers.get('Content-Type') ?? ''
  if (!ct.includes('text/event-stream') || !res.body) {
    const json = (await res.json().catch(() => null)) as { message?: string } | null
    return { status: res.status, code: json?.message ?? String(res.status) }
  }
  const reader = res.body.getReader()
  const dec = new TextDecoder()
  let buf = ''
  try {
    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      buf += dec.decode(value, { stream: true })
      let idx: number
      while ((idx = buf.indexOf('\n\n')) >= 0) {
        const block = buf.slice(0, idx)
        buf = buf.slice(idx + 2)
        const ev = parseBlock(block)
        if (ev) onEvent(ev)
      }
    }
  } catch (e) {
    if ((e as Error).name === 'AbortError') return { status: -1, code: 'ABORTED' }
    return { status: 0, code: 'NETWORK' }
  }
  return { status: res.status, code: 'SUCCESS' }
}

export function parseBlock(block: string): ChatEvent | null {
  let type = ''
  const data: string[] = []
  for (const line of block.split('\n')) {
    if (line.startsWith(':')) continue // ping
    if (line.startsWith('event:')) type = line.slice(6).trim()
    else if (line.startsWith('data:')) data.push(line.slice(5).trimStart())
  }
  if (!type) return null
  try {
    return { type, data: JSON.parse(data.join('\n') || '{}') } as ChatEvent
  } catch {
    return null
  }
}
