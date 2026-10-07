import { describe, it, expect, vi } from 'vitest'
import { lookURL, buildUI, applyAppearance, applyAccent, readSiteColors } from '../src/ui'

const base = {
  enabled: true, office_id: 'o', service_id: 'S1', service_label: 'เว็บ S1', is_hidden: false, avatar_url: '',
  display_name: 'น้องผู้ช่วย', greeting: 'สวัสดี', theme: 'light' as const,
  placement: { position: 'bottom-right' as const, offset_x: 12, offset_y: 12 },
}

describe('ค่ารูป/พื้นหลังจากคอนโซล', () => {
  it('asset: → URL ในคลังรูปของระบบ AI', () => {
    expect(lookURL('asset:avatars/dara.png', 'https://ai.test')).toBe('https://ai.test/widget/v1/assets/avatars/dara.png')
  })
  it.each(['asset:../x.png', 'asset:avatars/a b.png', 'javascript:alert(1)', 'https://x.test/a.png")', "https://x.test/a'.png", 'https://x.test/a (1).png', 'http://evil.test/a.png'])(
    '%s ถูกปฏิเสธ',
    (v) => expect(lookURL(v, 'https://ai.test')).toBe(''),
  )
  it('https และ localhost ตอน dev ผ่าน', () => {
    expect(lookURL('https://cdn.test/a.png', '')).toBe('https://cdn.test/a.png')
    expect(lookURL('http://localhost:6767/x.png', '')).toBe('http://localhost:6767/x.png')
  })
})

describe('หัวแชทแบบมีรูปผู้ช่วย + พื้นหลัง', () => {
  const build = () => buildUI(document.createElement('div').attachShadow({ mode: 'open' }))

  it('ไม่ตั้งอะไร = หน้าตาเดิม (ไม่มีหัวไล่สี ไม่มีพื้นหลัง)', () => {
    const ui = build()
    applyAppearance(ui, base, 'https://ai.test')
    expect(ui.panel.dataset.rich).toBeUndefined()
    expect(ui.log.dataset.bg).toBeUndefined()
    expect(ui.head.site.textContent).toBe('เว็บ S1')
  })

  it('ตั้งรูปจากคลัง + คำโปรย + ลาย → หัวไล่สี รูปจากคลัง คำโปรยขึ้น', () => {
    const ui = build()
    applyAppearance(ui, { ...base, avatar_url: 'asset:avatars/dara.png', tagline: 'ผู้ช่วยดูแลลูกค้า · ตอบทันที 24 ชม.', background: 'pattern:dots' }, 'https://ai.test')
    expect(ui.panel.dataset.rich).toBe('true')
    expect(ui.head.avatar.querySelector('img')!.getAttribute('src')).toBe('https://ai.test/widget/v1/assets/avatars/dara.png')
    expect(ui.head.tag.textContent).toContain('ผู้ช่วยดูแลลูกค้า')
    expect(ui.log.dataset.bg).toBe('dots')
  })

  it('พื้นหลังเป็นรูป → ใส่เป็นตัวแปร CSS · ค่าอันตรายไม่ถูกใส่', () => {
    const ui = build()
    applyAppearance(ui, { ...base, background: 'asset:backgrounds/blue.webp' }, 'https://ai.test')
    expect(ui.log.dataset.bg).toBe('image')
    expect(ui.log.style.getPropertyValue('--log-img')).toBe('url("https://ai.test/widget/v1/assets/backgrounds/blue.webp")')
    applyAppearance(ui, { ...base, background: 'https://x.test/a.png");background:red' }, 'https://ai.test')
    expect(ui.log.dataset.bg).toBeUndefined()
    expect(ui.log.style.getPropertyValue('--log-img')).toBe('')
  })

  it('หน้าเว็บผู้เล่น → ไม่โชว์ป้ายรหัสเว็บ', () => {
    const ui = build()
    applyAppearance(ui, base, '', false)
    expect(ui.head.site.style.display).toBe('none')
  })
})

describe('สีไล่ 2 สี + สีของแบรนด์จากหน้าเว็บ', () => {
  it('ไล่ 2–4 สี → --accent-stops · สีเดียว (domain เก่า) = ไล่สีเดียวกัน · เกิน 4 ตัดทิ้ง · ค่าแปลกไม่ใส่', () => {
    const h = document.createElement('div')
    applyAccent(h, ['#f5d76e', '#22c55e', '#2563eb'], 'dark')
    expect(h.style.getPropertyValue('--accent-stops')).toBe('#f5d76e, #22c55e, #2563eb')
    expect(h.style.getPropertyValue('--accent')).toBe('#f5d76e')
    applyAccent(h, '#0f6e63', 'light')
    expect(h.style.getPropertyValue('--accent-stops')).toBe('#0f6e63, #0f6e63')
    applyAccent(h, ['#111111', '#222222', '#333333', '#444444', '#555555'], 'light')
    expect(h.style.getPropertyValue('--accent-stops').split(',').length).toBe(4)
    applyAccent(h, ['red;x', 'url(x)'], 'light')
    expect(h.style.getPropertyValue('--accent-stops')).toBe('')
  })
  // การแปลงสีจากตัวแปร CSS ตรวจใน Chrome จริง (happy-dom ไม่คำนวณสีแบบเบราว์เซอร์)
  it('หน้าเว็บยังไม่ตั้งสี / ค่าแปลก → null (ใช้สีจากคอนโซลแทน)', () => {
    expect(readSiteColors({ accent: '--not-set' })).toBeNull()
    document.documentElement.style.setProperty('--evil', 'red;background:url(x)')
    expect(readSiteColors({ accent: '--evil' })).toBeNull()
  })
})

describe('ใช้สีของเว็บ แต่หน้าเว็บยังไม่ตั้งสี', () => {
  it('ซ่อนปุ่มระหว่างรอ (ไม่ให้เห็นสีตั้งต้นแวบ) · รอไม่เกิน 4 วิ แล้วโชว์ด้วยสีตั้งต้น', async () => {
    vi.useFakeTimers()
    try {
      const { mount, unmount } = await import('../src/index')
      await mount({ bootstrap: { ...base, color_source: 'site' }, apiBase: 'https://ai.test' })
      const style = () => (window as any).__aiOffice.__launcherStyle() as CSSStyleDeclaration
      expect(style().visibility).toBe('hidden')
      vi.advanceTimersByTime(1000)
      expect(style().visibility).toBe('hidden')
      vi.advanceTimersByTime(3500)
      expect(style().visibility).toBe('')
      unmount()
    } finally {
      vi.useRealTimers()
    }
  })
})
