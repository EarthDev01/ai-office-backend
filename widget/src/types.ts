export type Position = 'bottom-right' | 'bottom-left'
export type ThemeMode = 'auto' | 'light' | 'dark'

export interface Placement {
  position: Position
  offset_x: number
  offset_y: number
}

/** สิ่งเดียวที่ widget ได้เห็นเรื่องหน้าตา — ไม่มี allowlist/quota/model/origin อยู่ในนี้ */
export interface Bootstrap {
  enabled: boolean
  reason?: string
  /** เปิดใช้ได้แต่ตอนนี้ถามไม่ได้ เช่น quota_exceeded — โชว์ข้อความแทนการซ่อนปุ่ม */
  notice?: string
  office_id: string
  service_id: string
  service_label: string
  kind?: string
  is_hidden: boolean
  avatar_url: string
  display_name: string
  greeting: string
  theme: ThemeMode
  placement: Placement
  ticket_expires_at?: number
}

/** field หน้าตาที่โหมด preview ยอมให้แก้จากภายนอกได้ */
export const PREVIEW_ALLOWED_FIELDS = [
  'avatar_url',
  'display_name',
  'greeting',
  'theme',
  'placement',
  'service_label',
  'is_hidden',
] as const

export type PreviewField = (typeof PREVIEW_ALLOWED_FIELDS)[number]
export type PreviewConfig = Partial<Pick<Bootstrap, PreviewField>>

/** วิธีอ่านล็อกอินบนหน้าหลังบ้าน — มาจาก connectors/<kind>/host.yaml ผ่าน page-config */
export interface PageAuth {
  token: {
    source: 'localStorage' | 'sessionStorage'
    key: string
    format: 'raw' | 'json-expiration'
    value_field?: string
    expiration_field?: string
  }
  service: {
    source: 'localStorage' | 'sessionStorage' | 'query'
    key: string
    encoding: 'none' | 'base64'
  }
  /** template เช่น "{origin}/api" · snippet ทับได้ด้วย data-host-api-base */
  host_api_base: string
  /** template เช่น "/ai/session/{service}" */
  session_path: string
  auth_scheme?: string
  /** โหมด browser: header เพิ่มที่แนบตอนยิง API เดิมของหลังบ้าน */
  extra_headers?: { name: string; source: 'token' | 'localStorage' | 'sessionStorage'; key?: string; format?: 'raw' | 'json-expiration' }[]
  /** โหมด browser: อ่านตัวตนแอดมินจากไหน */
  identity?: IdentitySpec
}

export interface IdentitySpec {
  source: 'jwt' | 'request'
  method?: string
  path?: string
  root?: string
  id: string
  username: string
  display_name?: string
  level?: string
  dept?: string
  permissions?: { path: string; pluck?: string; where_field?: string; where_value?: unknown }
}

export interface HostUser {
  id: string
  username: string
  display_name?: string
  permissions?: string[]
  level?: number
  dept?: string
}

export interface PageConfig {
  enabled: boolean
  reason?: string
  kind?: string
  /** host = หลังบ้านออกตั๋ว (/ai/session) · browser = widget ขอตั๋วเองแล้วยิง API เดิมให้ AI */
  mode?: 'host' | 'browser'
  page_auth?: PageAuth
}

export interface CardField {
  label: string
  display: string
  format?: string
}

export interface CardCell {
  display: string
}

/** การ์ด = ค่าจากระบบโดยตรง ไม่ผ่านการพิมพ์ของโมเดล · มีเวลาที่ดึง + ลิงก์หน้าจริงเสมอ */
export interface Card {
  id: string
  kind: 'ok' | 'not_found' | 'error' | 'denied' | 'reference'
  tool: string
  title: string
  fields: CardField[]
  table?: { columns: { label: string }[]; rows: CardCell[][] }
  note?: string
  fetched_at: string
  link?: { label: string; path: string }
  cached?: boolean
}

export interface HistoryItem {
  id: string
  title: string
  opened_at: string
  last_message_at: string
  closed: boolean
}

export interface HistoryMessage {
  id: string
  role: 'user' | 'assistant'
  text: string
  cards: Card[]
  created_at: string
}
