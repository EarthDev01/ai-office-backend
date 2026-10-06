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
  /** รูปปุ่มเปิดแชท (ฟองแชท 3D) asset:launchers/<ไฟล์> — widget ย้อมสีตามสีของ widget · ว่าง = ไอคอนในตัว */
  launcher_icon?: string
  /** คำโปรยใต้ชื่อบนหัวแชท · ว่าง = ไม่แสดง */
  tagline?: string
  /** พื้นหลังห้องแชท: '' | pattern:<id> | asset:<ไฟล์> | URL รูป (server ตรวจแล้ว · widget ตรวจซ้ำก่อนใส่ CSS) */
  background?: string
  theme: ThemeMode
  /** สีหลัก #rrggbb ที่ตั้งจากคอนโซล · ว่าง = สีตั้งต้นของ widget */
  accent_color?: string
  /** สีไล่ 2–4 สี (เลือกเองในคอนโซล) · ว่าง = ใช้ accent_color สีเดียว (domain เก่า) */
  accent_colors?: string[]
  /** site = อ่านสีของแบรนด์จากหน้าเว็บ (ตาม page_colors ใน page-config) · ว่าง = ใช้สีที่ตั้งในคอนโซล */
  color_source?: string
  placement: Placement
}

/** field หน้าตาที่โหมด preview ยอมให้แก้จากภายนอกได้ */
export const PREVIEW_ALLOWED_FIELDS = [
  'avatar_url',
  'display_name',
  'greeting',
  'tagline',
  'launcher_icon',
  'background',
  'theme',
  'accent_color',
  'accent_colors',
  'color_source',
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
