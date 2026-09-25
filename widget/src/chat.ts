import { hostFetch, type FetchCommand } from './hostfetch'
import type { ChatSession } from './session'
import type { Card } from './types'

export interface ChatHandlers {
  status: (text: string) => void
  card: (card: Card) => void
  token: (text: string) => void
}

export class ChatError extends Error {}

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
  readToken: () => string
  conversationID: string
  text: string
  on: ChatHandlers
}): Promise<string> {
  const { apiBase, service, session, on } = opts
  const ticket = await session.ticket(service)
  const base = `${apiBase}/api/ai/widget/service/${encodeURIComponent(service)}`
  let conversationID = opts.conversationID

  const res = await fetch(`${base}/chat`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${ticket}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ conversation_id: conversationID, text: opts.text }),
  })
  if (!res.ok || !res.body) {
    if (res.status === 401) session.invalidate()
    const json = (await res.json().catch(() => null)) as { error?: string } | null
    throw new ChatError(json?.error || `เซิร์ฟเวอร์ตอบ ${res.status}`)
  }

  // คำสั่ง fetch ทำคู่ขนานได้ (tool หลายตัว) — ไม่หยุดอ่าน stream ระหว่างรอหลังบ้าน
  const relays: Promise<void>[] = []
  const relay = (cmd: FetchCommand) =>
    relays.push(
      hostFetch(session.hostApiBase(), cmd, opts.readToken())
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
  await readSSE(res.body, (event, data) => {
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
        failed = new ChatError(String(d.message ?? 'ผู้ช่วยตอบไม่สำเร็จ'))
        break
    }
  })
  await Promise.all(relays)
  if (failed) throw failed
  return conversationID
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
