import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, __resetSessions } from '../src/index'
import { readToken, readService } from '../src/page'
import { V10X_PAGE, ABATECH_PAGE, isPageConfig, fakeJWT, loginV10x } from './fixtures'

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

const realFetch = globalThis.fetch

/** fetch จำลอง: page-config = v10x · อย่างอื่นให้ handler ตอบ (ค่าเริ่มต้น payload null) */
function mockFetch(handler: (url: string, init: any) => any = async () => ({ ok: true, json: async () => ({ payload: null }) }), page: unknown = V10X_PAGE) {
  globalThis.fetch = (async (url: any, init: any) => {
    if (isPageConfig(url)) return { ok: true, status: 200, json: async () => ({ payload: page }) }
    return handler(String(url), init)
  }) as any
}

beforeEach(() => {
  document.body.innerHTML = ''
  document.head.innerHTML = ''
  localStorage.clear()
  delete (window as any).__aiOffice
  __resetSessions()
  globalThis.fetch = realFetch
  mockFetch()
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

describe('อ่าน token กับ service ตาม page-config (v10x)', () => {
  it('อ่าน token ที่ยังไม่หมดอายุ', () => {
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    expect(readToken(V10X_PAGE.token)).toBe('jwt-abc')
  })

  it('token หมดอายุแล้วถือว่าไม่มี', () => {
    localStorage.setItem('auth_token', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) - 1 }))
    expect(readToken(V10X_PAGE.token)).toBe('')
  })

  it('ค่าที่พังอยู่ใน localStorage ต้องไม่ทำให้ throw', () => {
    localStorage.setItem('auth_token', 'ไม่ใช่ json')
    expect(readToken(V10X_PAGE.token)).toBe('')
  })

  it('อ่าน service ที่แอดมินเลือกอยู่', () => {
    localStorage.setItem('web-service', 'PG99')
    expect(readService(V10X_PAGE.service)).toBe('PG99')
  })
})

describe('อ่าน token กับ service ตาม page-config (abatech)', () => {
  it('token ดิบ · ค่า "null" ที่หน้าเขียนตอนออกจากระบบ = ไม่มี', () => {
    localStorage.setItem('headertoken', 'jwt-raw')
    expect(readToken(ABATECH_PAGE.token)).toBe('jwt-raw')
    localStorage.setItem('headertoken', 'null')
    expect(readToken(ABATECH_PAGE.token)).toBe('')
  })

  it('service มาจาก ?service= แบบ base64 · ไม่มี/พัง = ยังไม่เลือกเว็บ', () => {
    history.replaceState(null, '', '/Deposit?service=' + encodeURIComponent(btoa('DEMOSLOT')))
    expect(readService(ABATECH_PAGE.service)).toBe('DEMOSLOT')
    history.replaceState(null, '', '/Dashboard')
    expect(readService(ABATECH_PAGE.service)).toBe('')
    history.replaceState(null, '', '/Deposit?service=%%%')
    expect(readService(ABATECH_PAGE.service)).toBe('')
    history.replaceState(null, '', '/')
  })

  it('ยิง bootstrap ด้วย headertoken + service จาก URL', async () => {
    localStorage.setItem('headertoken', 'aba-jwt')
    history.replaceState(null, '', '/Deposit?service=' + btoa('DEMOSLOT'))
    const calls: any[] = []
    let sentAuth = ''
    mockFetch(async (_u, init) => {
      sentAuth = init?.headers?.Authorization ?? ''
      return { ok: true, json: async () => ({ payload: null }) }
    }, ABATECH_PAGE)

    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })
    history.replaceState(null, '', '/')

    expect(calls[0].url).toBe('https://ai.example.com/api/ai/widget/service/DEMOSLOT/bootstrap')
    expect(sentAuth).toBe('Bearer aba-jwt')
  })

  it('หน้าที่ไม่ผูกเว็บ (ไม่มี ?service=) → ไม่ยิง และบอกสาเหตุ', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    localStorage.setItem('headertoken', 'aba-jwt')
    history.replaceState(null, '', '/Dashboard')
    mockFetch(undefined, ABATECH_PAGE)
    const calls: any[] = []
    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })
    expect(calls).toHaveLength(0)
    expect(info.mock.calls.flat().join(' ')).toContain('?service=')
    info.mockRestore()
  })

  it('สิทธิ์อ่านจาก JWT อีกใบ (localStorage.token) · Isactive != 1 ถูกตัด · ไม่ยิงหลังบ้าน', async () => {
    localStorage.setItem('headertoken', 'aba-jwt')
    localStorage.setItem('token', fakeJWT({ result: { role: { permission: [
      { code: '4001', Isactive: 1 }, { code: '4002', Isactive: 0 }, { code: 'M003', Isactive: 1 },
    ] } } }))
    history.replaceState(null, '', '/Deposit?service=' + btoa('DEMOSLOT'))
    const calls: { url: string; init: any }[] = []
    mockFetch(async (u, init) => {
      calls.push({ url: u, init })
      if (u.endsWith('/browser-session')) return Response.json({ payload: { ticket: 'tkt', expires_in: 1800 } })
      if (u.endsWith('/chat')) return new Response('event: done\ndata: {}\n\n', { status: 200 })
      throw new Error('unexpected ' + u)
    }, ABATECH_PAGE)

    await mount({ bootstrap: base, apiBase: 'https://ai.test' })
    await w().__send('ยอดฝากวันนี้')
    history.replaceState(null, '', '/')

    const session = calls.find((c) => c.url.endsWith('/browser-session'))!
    expect(session.url).toBe('https://ai.test/api/ai/widget/service/DEMOSLOT/browser-session')
    expect(session.init.headers.Authorization).toBe('Bearer aba-jwt')
    expect(JSON.parse(session.init.body)).toEqual({ permissions: ['4001', 'M003'] })
    expect(calls.some((c) => c.url.startsWith('http://localhost:7777'))).toBe(false)
  })
})

describe('การยิง bootstrap', () => {
  it('ใส่ service ลงใน path + ส่ง Bearer', async () => {
    loginV10x('PG99')

    const calls: any[] = []
    let sentAuth = ''
    mockFetch(async (_u, init) => {
      sentAuth = init?.headers?.Authorization ?? ''
      return { ok: true, json: async () => ({ payload: null }) }
    })

    await mount({ dataset: { apiBase: 'https://ai.example.com' }, onFetch: (i) => calls.push(i) })

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
    loginV10x('')
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
    loginV10x('')
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })
    expect(info.mock.calls.flat().join(' ')).toContain('web-service')
    info.mockRestore()
  })

  it('403 ORIGIN_NOT_REGISTERED (ตั้งแต่ page-config) → บอกโดเมนและวิธีแก้', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    loginV10x('K11S')

    globalThis.fetch = (async () => ({
      ok: false, status: 403,
      json: async () => ({ message: 'ORIGIN_NOT_REGISTERED', error: 'โดเมนนี้ยังไม่ได้ลงทะเบียน' }),
    })) as any
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })

    const out = info.mock.calls.flat().join(' ')
    expect(out).toContain('ORIGIN_NOT_REGISTERED')
    expect(out).toContain('URL ของ domain')
    expect(out).toContain(location.origin)
    info.mockRestore()
  })

  it('not_in_allowlist → บอกให้ไปเพิ่ม username', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    loginV10x('K11S')

    mockFetch(async () => ({
      ok: true, status: 200,
      json: async () => ({ payload: { enabled: false, reason: 'not_in_allowlist' } }),
    }))
    await mount({ dataset: { apiBase: 'https://ai.example.com' } })

    expect(info.mock.calls.flat().join(' ')).toContain('allowlist')
    info.mockRestore()
  })
})

describe('สีหลักจากคอนโซล', () => {
  const host = () => w().__host() as HTMLElement

  it('ตั้งสีแล้วทับ --accent + คำนวณพื้นอ่อนและสีตัวอักษรให้', async () => {
    await mount({ bootstrap: { ...base, accent_color: '#1e66f5' } })
    expect(host().style.getPropertyValue('--accent')).toBe('#1e66f5')
    expect(host().style.getPropertyValue('--accent-soft')).toMatch(/^#[0-9a-f]{6}$/)
    expect(host().style.getPropertyValue('--on-accent')).toBe('#ffffff')
  })

  it('สีสว่าง → ตัวอักษรบนสีหลักเป็นสีเข้ม อ่านออก', async () => {
    await mount({ bootstrap: { ...base, accent_color: '#ffd400' } })
    expect(host().style.getPropertyValue('--on-accent')).toBe('#13282B')
  })

  it('ไม่ตั้ง / รูปแบบผิด → ใช้สีตั้งต้นของธีม (ไม่ใส่อะไรลง style)', async () => {
    await mount({ bootstrap: base })
    expect(host().style.getPropertyValue('--accent')).toBe('')
    await mount({ bootstrap: { ...base, accent_color: 'red;background:url(x)' } })
    expect(host().style.getPropertyValue('--accent')).toBe('')
  })

  it('หน้าตั้งค่าเปลี่ยนสีสดใน preview ได้', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    await mount({ dataset: { previewMount: '#box' } })
    window.postMessage({ type: 'ai-office:preview-config', config: { accent_color: '#aa3300' } }, window.location.origin)
    await tick()
    expect(host().style.getPropertyValue('--accent')).toBe('#aa3300')

    window.postMessage({ type: 'ai-office:preview-config', config: { accent_color: '' } }, window.location.origin)
    await tick()
    expect(host().style.getPropertyValue('--accent')).toBe('')
  })
})
