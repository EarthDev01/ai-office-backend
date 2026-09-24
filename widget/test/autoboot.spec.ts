import { describe, it, expect } from 'vitest'
import { Window } from 'happy-dom'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const BUNDLE = resolve(__dirname, '../../static/widget/ai-office.v1.js')

/**
 * ทดสอบเส้นทางที่หน้า office ใช้จริง — โหลด bundle ที่ build แล้วเป็น <script>
 * แล้วดูว่ามันเรียก /api/ai/widget/service/:id/bootstrap และสร้าง widget เองได้ไหม
 *
 * unit test ตัวอื่นเรียก mount() ตรง ๆ จึงไม่ครอบส่วนนี้
 */
async function boot(payload: unknown, opts: { attrs?: string; loggedIn?: boolean; service?: string } = {}) {
  const win = new Window({ url: 'https://office.example.com/deposit' })
  const doc = win.document
  const calls: string[] = []

  ;(win as any).fetch = async (url: string) => {
    calls.push(String(url))
    return { ok: true, json: async () => ({ payload }) }
  }

  doc.body.innerHTML = '<div id="office">เนื้อหาเดิม</div>'

  // จำลองสภาพหน้า office ที่ล็อกอินแล้วและเลือกเว็บไว้
  if (opts.loggedIn !== false) {
    win.localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    win.localStorage.setItem('web-service', opts.service ?? 'K11S')
  }

  const code = readFileSync(BUNDLE, 'utf8')
  const s = doc.createElement('script')
  s.setAttribute('data-api-base', 'https://ai.example.com')
  if (opts.attrs) {
    for (const pair of opts.attrs.split(' ')) {
      const [k, v] = pair.split('=')
      s.setAttribute(k, v.replace(/"/g, ''))
    }
  }
  s.textContent = code
  doc.head.appendChild(s)

  await new Promise((r) => setTimeout(r, 20))
  return { win, doc, calls }
}

const enabled = {
  enabled: true,
  office_id: 'demo',
  service_id: 'K11S',
  service_label: 'เว็บ K11S',
  is_hidden: false,
  avatar_url: '',
  display_name: 'ผู้ช่วยหลังบ้าน',
  greeting: 'สวัสดีครับ',
  theme: 'light',
  placement: { position: 'bottom-right', offset_x: 12, offset_y: 12 },
}

describe('auto-boot จาก <script>', () => {
  it('เรียก /api/ai/widget/service/:id/bootstrap แล้วสร้าง widget ขึ้นมาเอง (snippet ไม่มี key)', async () => {
    const { doc, calls } = await boot(enabled)

    expect(calls.some((u) => u.endsWith('/api/ai/widget/service/K11S/bootstrap'))).toBe(true)
    expect(doc.querySelector('[data-ai-office-host]')).not.toBeNull()
    expect(doc.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
  })

  it('enabled=false → ไม่มี DOM ของ widget โผล่ในหน้า office เลย', async () => {
    const { doc } = await boot({ enabled: false, reason: 'disabled' })
    expect(doc.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('ยังไม่ล็อกอินในหน้า office → ไม่ยิงและไม่โผล่', async () => {
    const { doc, calls } = await boot(enabled, { loggedIn: false })
    expect(calls).toHaveLength(0)
    expect(doc.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('ล็อกอินหลังโหลดหน้าแล้ว (SPA) → ปุ่มโผล่เองไม่ต้อง refresh', async () => {
    const { win, doc, calls } = await boot(enabled, { loggedIn: false })
    // ก่อนล็อกอิน: ยังไม่มีปุ่มและยังไม่ยิง
    expect(doc.querySelector('[data-ai-office-host]')).toBeNull()
    expect(calls).toHaveLength(0)

    // จำลอง login ใน SPA — set localStorage โดยไม่ reload หน้า
    win.localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    win.localStorage.setItem('web-service', 'K11S')

    // widget เฝ้า session เอง (poll 800ms) — รอให้รอบ poll ทำงาน
    await new Promise((r) => setTimeout(r, 1000))

    expect(calls.some((u) => u.endsWith('/api/ai/widget/service/K11S/bootstrap'))).toBe(true)
    expect(doc.querySelector('[data-ai-office-host]')).not.toBeNull()
  })

  it('สลับ service แล้ว URL เปลี่ยนตาม โดย snippet ไม่ต้องแก้', async () => {
    const { calls } = await boot(enabled, { service: 'PG99' })
    expect(calls[0]).toContain('/api/ai/widget/service/PG99/bootstrap')
  })

  it('bootstrap ล้มเหลว → หน้า office ยังอยู่ครบ ไม่พังตาม', async () => {
    const win = new Window({ url: 'https://office.example.com/deposit' })
    ;(win as any).fetch = async () => {
      throw new Error('network down')
    }
    win.document.body.innerHTML = '<div id="office">เนื้อหาเดิม</div>'
    win.localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    win.localStorage.setItem('web-service', 'K11S')

    const s = win.document.createElement('script')
    s.setAttribute('data-api-base', 'https://ai.example.com')
    s.textContent = readFileSync(BUNDLE, 'utf8')
    win.document.head.appendChild(s)
    await new Promise((r) => setTimeout(r, 20))

    expect(win.document.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
    expect(win.document.querySelector('[data-ai-office-host]')).toBeNull()
  })
})
