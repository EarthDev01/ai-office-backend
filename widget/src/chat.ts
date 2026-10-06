import { hostFetch, type FetchCommand } from './hostfetch'
import type { ChatSession } from './session'
import type { Card } from './types'

export interface ChatHandlers {
  status: (text: string) => void
  card: (card: Card) => void
  token: (text: string) => void
}

/** code = รหัสจาก server (message ของ envelope หรือ code ของ SSE error) — '' = ไม่รู้สาเหตุ */
const UNAVAILABLE = 'ผู้ช่วยไม่พร้อมใช้งานชั่วคราว กรุณาลองใหม่ภายหลัง'

export class ChatError extends Error {
  constructor(
    message: string,
    readonly code = '',
  ) {
    super(message)
  }
}

/**
 * ข้อความที่แอดมินเห็นตามรหัสจาก server (spec §8) — ไม่โชว์ข้อความเทคนิคของ server ตรง ๆ
 * รหัสที่ไม่อยู่ในนี้ใช้ข้อความที่ server ส่งมา (SSE error ของแชทเขียนให้คนอ่านอยู่แล้ว)
 */
export const USER_MESSAGES: Record<string, string> = {
  SERVICE_DISABLED: 'ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่',
  CHAT_DISABLED: 'ผู้ช่วยของเว็บนี้ถูกปิดใช้งานอยู่',
  LLM_NOT_CONFIGURED: 'ผู้ช่วยยังไม่พร้อมใช้งาน — ผู้ดูแลระบบยังตั้งค่าไม่เสร็จ',
  SERVICE_NOT_ALLOWED: 'บัญชีนี้ไม่มีสิทธิ์ใช้ผู้ช่วยในเว็บนี้',
  SESSION_EXPIRED: 'การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่',
  NOT_AUTHENTICATED: 'การล็อกอินหลังบ้านหมดอายุ กรุณาล็อกอินใหม่',
  BACKOFFICE_UNAVAILABLE: 'ตรวจสอบสิทธิ์กับหลังบ้านไม่ได้ชั่วคราว กรุณาลองใหม่อีกครั้ง',
  TICKET_INVALID: 'การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า',
  TICKET_SERVICE_MISMATCH: 'การเชื่อมต่อกับผู้ช่วยหมดอายุ กรุณารีเฟรชหน้า',
  ORIGIN_NOT_REGISTERED: 'ผู้ช่วยยังไม่ได้เปิดให้หลังบ้านนี้',
}

/** รหัสที่แปลว่าผู้ช่วยของเว็บนี้ใช้ต่อไม่ได้ — widget เก็บปุ่มและปิดช่องพิมพ์ */
export const DISABLED_CODES = new Set(['SERVICE_DISABLED', 'CHAT_DISABLED', 'SERVICE_NOT_ALLOWED', 'ORIGIN_NOT_REGISTERED'])

/** ตั๋วใช้ไม่ได้ — ขอใหม่แล้วลองซ้ำ 1 ครั้งโดยผู้ใช้ไม่ต้องรู้ */
const TICKET_CODES = new Set(['TICKET_INVALID', 'TICKET_SERVICE_MISMATCH'])

/** แปลงคำตอบ error ของ API (envelope {message, error}) เป็น ChatError ที่มีข้อความให้คนอ่าน */
export async function apiError(res: Response, fallback: string): Promise<ChatError> {
  const json = (await res.json().catch(() => null)) as { message?: string; error?: string } | null
  const code = json?.message ?? ''
  return new ChatError(USER_MESSAGES[code] ?? fallback, code)
}

/**
 * ส่งคำถาม 1 ข้อแล้วเล่นบทตาม SSE จาก backend
 *
 *   status  แสดงสถานะ · fetch  ยิง API หลังบ้านแล้วส่งผลกลับที่ relay
 *   card    วาดการ์ด · token  ต่อข้อความ · done  จบ · error  แจ้ง
 *
 * คืน conversation_id ที่ backend ใช้ (ส่งกลับไปในข้อความถัดไปของห้องเดิม)
 */
export async function runChat(opts: {
  apiBase: string
  service: string
  session: ChatSession
  conversationID: string
  text: string
  on: ChatHandlers
  /** ยกเลิกคำตอบที่กำลังส่ง (เช่น สลับเว็บ) — ยกเลิกแล้วไม่ยิงหลังบ้าน/relay ต่อ */
  signal?: AbortSignal
}): Promise<string> {
  const { apiBase, service, session, on } = opts
  const base = `${apiBase}/api/ai/widget/service/${encodeURIComponent(service)}`
  let conversationID = opts.conversationID

  const start = async (ticket: string) =>
    fetch(`${base}/chat`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ conversation_id: conversationID, text: opts.text }),
      signal: opts.signal,
    })
  let ticket = await session.ticket(service)
  let res = await start(ticket)
  if (!res.ok || !res.body) {
    let err: ChatError | null = await apiError(res, UNAVAILABLE)
    if (TICKET_CODES.has(err.code)) {
      // ตั๋วหมด/ใช้ไม่ได้ (เช่น เปิดหน้าทิ้งไว้นาน) → ขอใหม่เงียบ ๆ แล้วส่งซ้ำ 1 ครั้ง
      session.invalidate()
      ticket = await session.ticket(service)
      res = await start(ticket)
      err = res.ok && res.body ? null : await apiError(res, UNAVAILABLE)
    }
    if (err) {
      if (TICKET_CODES.has(err.code)) session.invalidate()
      throw err
    }
  }

  // คำสั่ง fetch ทำคู่ขนานได้ (tool หลายตัว) — ไม่หยุดอ่าน stream ระหว่างรอหลังบ้าน
  const relays: Promise<void>[] = []
  const relay = (cmd: FetchCommand) =>
    !opts.signal?.aborted && // ยกเลิกแล้ว (สลับเว็บ) = ไม่ยิงหลังบ้านด้วยคำสั่งของห้องเก่า
    relays.push(
      hostFetch(session.hostApiBase(), cmd, session.readToken(), session.authScheme())
        .then((r) =>
          fetch(`${base}/chat/relay/${encodeURIComponent(cmd.id)}`, {
            method: 'POST',
            headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json' },
            body: JSON.stringify(r),
          }),
        )
        .then(() => undefined)
        .catch(() => undefined), // ส่งไม่ถึง = backend หมดเวลารอแล้วแจ้งเอง
    )

  let failed: ChatError | null = null
  await readSSE(res.body!, (event, data) => {
    const d = data as Record<string, unknown>
    switch (event) {
      case 'status':
        if (typeof d.conversation_id === 'string') conversationID = d.conversation_id
        on.status(String(d.text ?? ''))
        break
      case 'fetch':
        relay(d as unknown as FetchCommand)
        break
      case 'card':
        on.card(d as unknown as Card)
        break
      case 'token':
        on.token(String(d.text ?? ''))
        break
      case 'error':
        failed = new ChatError(String(d.message ?? 'ผู้ช่วยตอบไม่สำเร็จ'), String(d.code ?? ''))
        break
    }
  })
  await Promise.all(relays)
  if (failed) throw failed
  return conversationID
}

/** ห้องแชทย้อนหลังของตัวเอง (server กรองตามแอดมิน + session หลังบ้านเดียวกัน · ≤ 7 วัน) */
export interface ConversationSummary {
  id: string
  title: string
  updated_at: string
}

export interface HistoryMessage {
  role: 'user' | 'assistant'
  text: string
  cards?: Card[]
  status?: string
}

/** GET ด้วยตั๋วแชท — ตั๋วหมดขอใหม่เงียบ ๆ 1 ครั้งเหมือนตอนถาม */
async function ticketGet<T>(apiBase: string, service: string, session: ChatSession, path: string): Promise<T> {
  const url = `${apiBase}/api/ai/widget/service/${encodeURIComponent(service)}${path}`
  const call = async () => fetch(url, { headers: { Authorization: `Bearer ${await session.ticket(service)}` } })
  let res = await call()
  if (!res.ok) {
    let err: ChatError | null = await apiError(res, UNAVAILABLE)
    if (TICKET_CODES.has(err.code)) {
      session.invalidate()
      res = await call()
      err = res.ok ? null : await apiError(res, UNAVAILABLE)
    }
    if (err) throw err
  }
  const json = (await res.json().catch(() => null)) as { payload?: T } | null
  if (!json?.payload) throw new ChatError(UNAVAILABLE)
  return json.payload
}

export function listConversations(apiBase: string, service: string, session: ChatSession, days = 7) {
  return ticketGet<ConversationSummary[]>(apiBase, service, session, `/conversations?days=${days}`)
}

export function loadConversation(apiBase: string, service: string, session: ChatSession, id: string) {
  return ticketGet<{ id: string; title: string; messages: HistoryMessage[] }>(
    apiBase,
    service,
    session,
    `/conversations/${encodeURIComponent(id)}`,
  )
}

/** ข้อความ 1 ชิ้นในห้องคุยของผู้ช่วยคอนโซล — ห้องอยู่ที่หน้าเว็บ ส่งไปทั้งชุดทุกครั้ง */
export interface ConsoleTurn {
  role: 'user' | 'assistant'
  text: string
}

/**
 * ถามผู้ช่วยในคอนโซล AI Office — ใช้ token ของคอนโซล ไม่มี fetch/relay ไปหลังบ้านลูกค้า
 * (server ดึงข้อมูลในระบบเองตามสิทธิ์ของคนถาม) · event ชุดเดียวกับแชทปกติ
 */
export async function runConsoleChat(opts: {
  apiBase: string
  token: string
  messages: ConsoleTurn[]
  page: string
  on: ChatHandlers
}): Promise<void> {
  const res = await fetch(`${opts.apiBase}/api/ai/admin/assistant`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${opts.token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages: opts.messages, page: opts.page }),
  })
  if (!res.ok || !res.body) {
    const json = (await res.json().catch(() => null)) as { error?: string } | null
    throw new ChatError(json?.error || `เซิร์ฟเวอร์ตอบ ${res.status}`)
  }
  let failed: ChatError | null = null
  await readSSE(res.body, (event, data) => {
    const d = data as Record<string, unknown>
    if (event === 'status') opts.on.status(String(d.text ?? ''))
    else if (event === 'card') opts.on.card(d as unknown as Card)
    else if (event === 'token') opts.on.token(String(d.text ?? ''))
    else if (event === 'error') failed = new ChatError(String(d.message ?? 'ผู้ช่วยตอบไม่สำเร็จ'))
  })
  if (failed) throw failed
}

/** อ่าน text/event-stream จาก fetch — EventSource ใช้ไม่ได้เพราะต้อง POST + ส่ง Authorization */
export async function readSSE(body: ReadableStream<Uint8Array>, onEvent: (event: string, data: unknown) => void) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buf = ''
  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    let i: number
    while ((i = buf.indexOf('\n\n')) >= 0) {
      const block = buf.slice(0, i)
      buf = buf.slice(i + 2)
      let event = 'message'
      let data = ''
      for (const line of block.split('\n')) {
        if (line.startsWith('event:')) event = line.slice(6).trim()
        else if (line.startsWith('data:')) data += line.slice(5).trim()
      }
      if (data) onEvent(event, JSON.parse(data))
    }
  }
}
