import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount } from '../src/index'

const base = {
  enabled: true,
  office_id: 'demo',
  service_id: 'K11S',
  service_label: 'เว็บ K11S',
  is_hidden: false,
  avatar_url: '',
  display_name: 'ผู้ช่วยหลังบ้าน',
  greeting: 'สวัสดีครับ',
  theme: 'light' as const,
  placement: { position: 'bottom-right' as const, offset_x: 12, offset_y: 12 },
}

const w = () => (window as any).__aiOffice

beforeEach(() => {
  document.body.innerHTML = ''
  document.head.innerHTML = ''
  localStorage.clear()
  delete (window as any).__aiOffice
})

describe('isolation', () => {
  it('ไม่แตะ DOM ของ office นอก shadow root ของตัวเอง', async () => {
    document.body.innerHTML = '<div id="office">เนื้อหาเดิม</div>'
    await mount({ bootstrap: base })
    expect(document.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
  })

  it('shadow root เป็น closed — office เข้าถึงข้างในไม่ได้', async () => {
    await mount({ bootstrap: base })
    const host = document.querySelector('[data-ai-office-host]')!
    expect(host.shadowRoot).toBeNull()
  })

  it('ไม่แตะ <head> ของ office', async () => {
    await mount({ bootstrap: base })
    expect(document.head.innerHTML).toBe('')
  })
})

describe('bootstrap gating', () => {
  it('enabled=false → ไม่สร้าง DOM อะไรเลย', async () => {
    await mount({ bootstrap: { ...base, enabled: false } })
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('is_hidden=true → โหลดแต่ไม่โชว์ปุ่มลอย และเปิดเองได้', async () => {
    await mount({ bootstrap: { ...base, is_hidden: true } })
    expect(document.querySelector('[data-ai-office-host]')).not.toBeNull()
    expect(w().__hasLauncher()).toBe(false)
    expect(w().__isOpen()).toBe(false)
    w().open()
    expect(w().__isOpen()).toBe(true)
  })
})

describe('placement', () => {
  it('bottom-left + offset ถูกใช้จริง', async () => {
    await mount({
      bootstrap: { ...base, placement: { position: 'bottom-left', offset_x: 40, offset_y: 24 } },
    })
    const s = w().__launcherStyle()
    expect(s.left).toBe('40px')
    expect(s.bottom).toBe('24px')
    expect(s.right).toBe('')
  })

  it('สลับกลับเป็น bottom-right แล้ว left ต้องถูกล้าง', async () => {
    await mount({ bootstrap: { ...base, placement: { position: 'bottom-left', offset_x: 40, offset_y: 24 } } })
    window.postMessage(
      { type: 'ai-office:preview-config', config: { placement: { position: 'bottom-right', offset_x: 12, offset_y: 12 } } },
      window.location.origin,
    )
    // โหมดปกติไม่ฟัง message — ค่าต้องไม่เปลี่ยน
    await tick()
    expect(w().__launcherStyle().left).toBe('40px')
  })
})

describe('ไม่รับ tenant จากหน้าเว็บ', () => {
  it('data-service-id ถูกทิ้งและเตือน', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const calls: any[] = []
    await mount({ bootstrap: base, dataset: { serviceId: 'HACK' }, onFetch: (i) => calls.push(i) })

    expect(warn).toHaveBeenCalled()
    expect(JSON.stringify(calls)).not.toContain('HACK')
    warn.mockRestore()
  })
})

describe('โหมด preview', () => {
  it('mount ในกล่อง ไม่ลอยมุมจอ และไม่ยิง API', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    const calls: any[] = []
    await mount({ dataset: { previewMount: '#box' }, onFetch: (i) => calls.push(i) })

    const host = document.querySelector('#box [data-ai-office-host]') as HTMLElement
    expect(host).not.toBeNull()
    expect(host.getAttribute('data-preview')).toBe('true')
    expect(calls).toHaveLength(0)
  })

  it('รับ config หน้าตาสดทาง postMessage', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    await mount({ dataset: { previewMount: '#box' } })

    window.postMessage(
      { type: 'ai-office:preview-config', config: { display_name: 'ผู้ช่วยเว็บ K11S' } },
      window.location.origin,
    )
    await tick()
    expect(w().__config().display_name).toBe('ผู้ช่วยเว็บ K11S')
  })

  it('ไม่รับ field ที่เป็นเรื่องความปลอดภัย', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    await mount({ dataset: { previewMount: '#box' }, bootstrap: { ...base } })

    window.postMessage(
      { type: 'ai-office:preview-config', config: { enabled: false, allowlist: ['x'], model: 'evil' } as any },
      window.location.origin,
    )
    await tick()

    const cfg = w().__config()
    expect(cfg.enabled).toBe(true)
    expect(cfg).not.toHaveProperty('allowlist')
    expect(cfg).not.toHaveProperty('model')
  })

  it('โหมดปกติไม่ฟัง postMessage เลย', async () => {
    await mount({ bootstrap: base })
    window.postMessage(
      { type: 'ai-office:preview-config', config: { display_name: 'ถูกแก้จากข้างนอก' } },
      window.location.origin,
    )
    await tick()
    expect(w().__config().display_name).toBe('ผู้ช่วยหลังบ้าน')
  })
})

describe('ความปลอดภัยของการ render', () => {
  it('ข้อความผู้ใช้ไม่ถูกตีความเป็น HTML', async () => {
    await mount({ bootstrap: { ...base, greeting: '<img src=x onerror=alert(1)>' } })
    const host = w().__host() as HTMLElement
    expect(host.outerHTML).not.toContain('onerror')
  })
})

function tick() {
  return new Promise((r) => setTimeout(r, 0))
}

describe('อ่าน token กับ service จาก localStorage ของ office', () => {
  it('อ่าน token ที่ยังไม่หมดอายุ', async () => {
    const { readOfficeToken } = await import('../src/index')
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    expect(readOfficeToken()).toBe('jwt-abc')
  })

  it('token หมดอายุแล้วถือว่าไม่มี', async () => {
    const { readOfficeToken } = await import('../src/index')
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) - 1 }))
    expect(readOfficeToken()).toBe('')
  })

  it('ค่าที่พังอยู่ใน localStorage ต้องไม่ทำให้ throw', async () => {
    const { readOfficeToken } = await import('../src/index')
    localStorage.setItem('auth_token', 'ไม่ใช่ json')
    expect(readOfficeToken()).toBe('')
  })

  it('อ่าน service ที่แอดมินเลือกอยู่', async () => {
    const { readOfficeService } = await import('../src/index')
    localStorage.setItem('web-service', 'PG99')
    expect(readOfficeService()).toBe('PG99')
  })
})

describe('การยิง bootstrap', () => {
  it('ใส่ public key และ service ลงใน path + ส่ง Bearer', async () => {
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    localStorage.setItem('web-service', 'PG99')

    const calls: any[] = []
    const origFetch = globalThis.fetch
    let sentAuth = ''
    globalThis.fetch = (async (url: any, init: any) => {
      sentAuth = init?.headers?.Authorization ?? ''
      return { ok: true, json: async () => ({ payload: null }) }
    }) as any

    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })
    globalThis.fetch = origFetch

    expect(calls[0].url).toBe('https://ai.example.com/api/ai/widget/service/PG99/bootstrap')
    expect(sentAuth).toBe('Bearer jwt-abc')
  })

  it('ยังไม่ล็อกอิน → ไม่ยิงและไม่โผล่', async () => {
    const calls: any[] = []
    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })
    expect(calls).toHaveLength(0)
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('ล็อกอินแล้วแต่ยังไม่ได้เลือกเว็บ → ไม่ยิง', async () => {
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    const calls: any[] = []
    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })
    expect(calls).toHaveLength(0)
  })
})

describe('บอกสาเหตุเมื่อไม่โผล่', () => {
  it('ยังไม่ล็อกอิน → บอกว่าไม่พบ auth_token', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })
    expect(info.mock.calls.flat().join(' ')).toContain('auth_token')
    info.mockRestore()
  })

  it('ล็อกอินแล้วแต่ยังไม่เลือกเว็บ → บอกว่าไม่พบ web-service', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt', expiration: Math.floor(Date.now() / 1000) + 600 }))
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })
    expect(info.mock.calls.flat().join(' ')).toContain('web-service')
    info.mockRestore()
  })

  it('403 ORIGIN_NOT_REGISTERED → บอกโดเมนและวิธีแก้', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt', expiration: Math.floor(Date.now() / 1000) + 600 }))
    localStorage.setItem('web-service', 'K11S')

    const orig = globalThis.fetch
    globalThis.fetch = (async () => ({
      ok: false, status: 403,
      json: async () => ({ message: 'ORIGIN_NOT_REGISTERED', error: 'โดเมนนี้ยังไม่ได้ลงทะเบียน' }),
    })) as any
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })
    globalThis.fetch = orig

    const out = info.mock.calls.flat().join(' ')
    expect(out).toContain('ORIGIN_NOT_REGISTERED')
    expect(out).toContain('โดเมนที่อนุญาต')
    expect(out).toContain(location.origin)
    info.mockRestore()
  })

  it('not_in_allowlist → บอกให้ไปเพิ่ม username', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt', expiration: Math.floor(Date.now() / 1000) + 600 }))
    localStorage.setItem('web-service', 'K11S')

    const orig = globalThis.fetch
    globalThis.fetch = (async () => ({
      ok: true, status: 200,
      json: async () => ({ payload: { enabled: false, reason: 'not_in_allowlist' } }),
    })) as any
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })
    globalThis.fetch = orig

    expect(info.mock.calls.flat().join(' ')).toContain('allowlist')
    info.mockRestore()
  })
})
