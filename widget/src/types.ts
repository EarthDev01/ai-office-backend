export type Position = 'bottom-right' | 'bottom-left'
export type ThemeMode = 'auto' | 'light' | 'dark'

export interface Placement {
  position: Position
  offset_x: number
  offset_y: number
}

/** สิ่งเดียวที่ widget ได้เห็นจาก server — ไม่มี allowlist/quota/model/origin อยู่ในนี้ */
export interface Bootstrap {
  enabled: boolean
  reason?: 'office_disabled' | 'service_disabled' | 'not_in_allowlist' | 'wrong_office' | 'no_service'
  office_id: string
  /** service ที่แอดมินกำลังดูอยู่ — มาจาก session ฝั่ง server ไม่ใช่จาก snippet */
  service_id: string
  service_label: string
  is_hidden: boolean
  avatar_url: string
  display_name: string
  greeting: string
  theme: ThemeMode
  placement: Placement
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

/** การ์ดข้อมูลที่ระบบสร้างจากผล API — ค่าทั้งหมดอยู่ที่นี่ ไม่ผ่าน LLM */
export interface Card {
  id: string
  kind: 'ok' | 'not_found' | 'denied' | 'error' | 'reference'
  tool: string
  title: string
  fields: { label: string; display: string }[] | null
  table?: { columns: { label: string }[]; rows: { display: string }[][] }
  note?: string
  fetched_at: string
  link?: { label: string; path: string }
  cached?: boolean
}
