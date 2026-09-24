import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount, boot, shutdown } from '../src/index'
import { genericReader, hostSessionURL } from '../src/pageauth'
import { parseBlock } from '../src/api'
import { renderCard, safeHref } from '../src/ui'
import type { PageAuth } from '../src/types'
import { BOOT, PAGE_AUTH, ev, installFetch, login, standardRoutes, tick, until, type FakeResponse } from './fake'

const base = { ...BOOT, theme: 'light' as const, placement: { position: 'bottom-right' as const, offset_x: 12, offset_y: 12 } }
const w = () => (window as any).__aiOffice
const ui = () => w().__ui()
const logText = () => (ui()?.log.textContent ?? '') as string
const DS = { publicKey: 'pk_abc', apiBase: 'https://ai.example.com' }

let fx: ReturnType<typeof installFetch> | null = null

beforeEach(() => {
  shutdown()
  document.body.innerHTML = ''
  document.head.innerHTML = ''
  localStorage.clear()
  sessionStorage.clear()
  delete (window as any).__aiOffice
})

afterEach(() => {
  shutdown()
  fx?.restore()
  fx = null
  vi.restoreAllMocks()
})

async function type(text: string) {
  ui().input.value = text
  ui().send.click()
}

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
    await mount({ bootstrap: { ...base, placement: { position: 'bottom-left', offset_x: 40, offset_y: 24 } } })
    const s = w().__launcherStyle()
    expect(s.left).toBe('40px')
    expect(s.bottom).toBe('24px')
    expect(s.right).toBe('')
  })

  it('โหมดปกติไม่ฟัง postMessage เปลี่ยน placement', async () => {
    await mount({ bootstrap: { ...base, placement: { position: 'bottom-left', offset_x: 40, offset_y: 24 } } })
    window.postMessage(
      { type: 'ai-office:preview-config', config: { placement: { position: 'bottom-right', offset_x: 12, offset_y: 12 } } },
      window.location.origin,
    )
    await tick()
    expect(w().__launcherStyle().left).toBe('40px')
  })
})

describe('โหมด preview', () => {
  it('mount ในกล่อง ไม่ลอยมุมจอ และไม่ยิง API', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    fx = installFetch([])
    await mount({ dataset: { previewMount: '#box' } })
    const host = document.querySelector('#box [data-ai-office-host]') as HTMLElement
    expect(host).not.toBeNull()
    expect(host.getAttribute('data-preview')).toBe('true')
    await type('ยอดวันนี้')
    expect(fx.calls).toHaveLength(0)
    expect(logText()).toContain('โหมดตัวอย่าง')
  })

  it('รับ config หน้าตาสดทาง postMessage', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    await mount({ dataset: { previewMount: '#box' } })
    window.postMessage({ type: 'ai-office:preview-config', config: { display_name: 'ผู้ช่วยเว็บ K11S' } }, window.location.origin)
    await tick()
    expect(w().__config().display_name).toBe('ผู้ช่วยเว็บ K11S')
  })

  it('ไม่รับ field ที่เป็นเรื่องความปลอดภัย', async () => {
    document.body.innerHTML = '<div id="box"></div>'
    await mount({ dataset: { previewMount: '#box' }, bootstrap: { ...base } })
    window.postMessage(
      { type: 'ai-office:preview-config', config: { enabled: false, allowlist: ['x'], model: 'evil', notice: 'quota_exceeded' } as any },
      window.location.origin,
    )
    await tick()
    const cfg = w().__config()
    expect(cfg.enabled).toBe(true)
    expect(cfg).not.toHaveProperty('allowlist')
    expect(cfg).not.toHaveProperty('model')
    expect(cfg.notice).toBeUndefined()
  })

  it('โหมดปกติไม่ฟัง postMessage เลย', async () => {
    await mount({ bootstrap: base })
    window.postMessage({ type: 'ai-office:preview-config', config: { display_name: 'ถูกแก้จากข้างนอก' } }, window.location.origin)
    await tick()
    expect(w().__config().display_name).toBe('ผู้ช่วยหลังบ้าน')
  })
})

describe('ความปลอดภัยของการ render', () => {
  it('ข้อความจาก server ไม่ถูกตีความเป็น HTML', async () => {
    await mount({ bootstrap: { ...base, greeting: '<img src=x onerror=alert(1)>' } })
    const host = w().__host() as HTMLElement
    expect(host.outerHTML).not.toContain('onerror')
  })

  it('ลิงก์ในการ์ดรับเฉพาะ path ในเว็บเดียวกัน', () => {
    expect(safeHref('/withdraw?service=K11S')).toBe('/withdraw?service=K11S')
    expect(safeHref('#/report')).toBe('#/report')
    expect(safeHref('javascript:alert(1)')).toBe('')
    expect(safeHref('//evil.example/x')).toBe('')
    expect(safeHref('https://evil.example/x')).toBe('')
    expect(safeHref('/\\evil.example')).toBe('')
  })

  it('การ์ด: ค่าลงด้วย textContent + มีเวลาที่ดึงและลิงก์', () => {
    const c = renderCard({
      id: 'c1', kind: 'ok', tool: 't', title: 'ถอนค้าง <b>x</b>',
      fields: [{ label: 'รวม', display: '<i>169,500.00</i>' }],
      fetched_at: new Date().toISOString(),
      link: { label: 'เปิดรายการถอน', path: '/withdraw' },
    })
    expect(c.innerHTML).not.toContain('<i>')
    expect(c.textContent).toContain('<i>169,500.00</i>')
    expect(c.textContent).toContain('ข้อมูล ณ')
    expect(c.querySelector('a')!.getAttribute('href')).toBe('/withdraw')
  })

  it('การ์ดที่ลิงก์อันตราย → ไม่มี <a>', () => {
    const c = renderCard({ id: 'c', kind: 'ok', tool: 't', title: '', fields: [], fetched_at: '', link: { label: 'x', path: 'javascript:alert(1)' } })
    expect(c.querySelector('a')).toBeNull()
  })

  it('การ์ดแบบตาราง', () => {
    const c = renderCard({
      id: 'c', kind: 'ok', tool: 't', title: 'ล่าสุด', fields: [], fetched_at: new Date().toISOString(),
      table: { columns: [{ label: 'ยูส' }, { label: 'ยอด' }], rows: [[{ display: 'somchai' }, { display: '500.00' }]] },
    })
    expect(c.querySelectorAll('th')).toHaveLength(2)
    expect(c.querySelector('td')!.textContent).toBe('somchai')
  })
})

describe('ตัวอ่านหน้าแบบกลาง (page_auth จาก connector)', () => {
  const pa = PAGE_AUTH as PageAuth

  it('json-expiration: อ่าน token ที่ยังไม่หมดอายุ', () => {
    localStorage.setItem('tok', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) + 600 }))
    expect(genericReader.readToken(pa)).toBe('jwt-abc')
  })

  it('json-expiration: หมดอายุแล้วถือว่าไม่มี (ทั้งวินาทีและมิลลิวินาที)', () => {
    localStorage.setItem('tok', JSON.stringify({ value: 'jwt-abc', expiration: Math.floor(Date.now() / 1000) - 1 }))
    expect(genericReader.readToken(pa)).toBe('')
    localStorage.setItem('tok', JSON.stringify({ value: 'jwt-abc', expiration: Date.now() - 1000 }))
    expect(genericReader.readToken(pa)).toBe('')
  })

  it('ค่าที่พังใน storage ต้องไม่ throw', () => {
    localStorage.setItem('tok', 'ไม่ใช่ json')
    expect(genericReader.readToken(pa)).toBe('')
  })

  it('raw token + service จาก query แบบ base64', () => {
    const pa2: PageAuth = {
      ...pa,
      token: { source: 'localStorage', key: 'rawtok', format: 'raw' },
      service: { source: 'query', key: 'service', encoding: 'base64' },
    }
    localStorage.setItem('rawtok', 'abc.def.ghi')
    history.replaceState(null, '', '/deposit?service=' + btoa('PG99'))
    expect(genericReader.readToken(pa2)).toBe('abc.def.ghi')
    expect(genericReader.readService(pa2)).toBe('PG99')
    history.replaceState(null, '', '/')
  })

  it('service รูปแบบแปลก → ไม่รับ (กันยัดค่าลง URL)', () => {
    localStorage.setItem('svc', '../../admin')
    expect(genericReader.readService(pa)).toBe('')
    localStorage.setItem('svc', 'K11S')
    expect(genericReader.readService(pa)).toBe('K11S')
  })

  it('URL ขอตั๋ว: {origin} + override จาก snippet', () => {
    expect(hostSessionURL(pa, 'K11S')).toBe(location.origin + '/api/ai/session/K11S')
    expect(hostSessionURL(pa, 'K11S', 'https://api.office.example/api/')).toBe('https://api.office.example/api/ai/session/K11S')
  })
})

describe('ขั้นตอนเปิดผู้ช่วย: page-config → host session → bootstrap(ตั๋ว)', () => {
  it('ส่ง token ผู้ใช้ไปที่ host เท่านั้น · backend ได้แค่ตั๋ว', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes())
    await mount({ dataset: DS })

    const sess = fx.to(/\/api\/ai\/session\//)
    expect(sess).toHaveLength(1)
    expect(sess[0].url).toBe(location.origin + '/api/ai/session/K11S')
    expect(sess[0].method).toBe('POST')
    expect(sess[0].headers.authorization).toBe('Bearer host-user-token')

    const boot = fx.to(/\/bootstrap$/)
    expect(boot[0].url).toBe('https://ai.example.com/api/ai/office/pk_abc/service/K11S/bootstrap')
    expect(boot[0].headers.authorization).toBe('Bearer tk-K11S-1')

    // ██ token ของ host ห้ามไปถึง ai-office-backend (D-87)
    for (const c of fx.calls.filter((c) => c.url.startsWith('https://ai.example.com'))) {
      expect(JSON.stringify(c)).not.toContain('host-user-token')
    }
    expect(document.querySelector('[data-ai-office-host]')).not.toBeNull()
  })

  it('data-service-id ถูกทิ้งและเตือน · ไม่ไปถึง request ใด', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(standardRoutes())
    await mount({ dataset: { ...DS, serviceId: 'HACK' } })
    expect(warn).toHaveBeenCalled()
    expect(JSON.stringify(fx.calls)).not.toContain('HACK')
  })

  it('ยังไม่ล็อกอิน → ไม่ขอตั๋ว ไม่โผล่ + บอกชื่อช่องที่หา', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    fx = installFetch(standardRoutes())
    await mount({ dataset: DS })
    expect(fx.to(/\/api\/ai\/session\//)).toHaveLength(0)
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
    expect(info.mock.calls.flat().join(' ')).toContain('localStorage["tok"]')
  })

  it('ล็อกอินแล้วแต่ยังไม่เลือกเว็บ → ไม่ขอตั๋ว', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage, 'host-user-token', '')
    fx = installFetch(standardRoutes())
    await mount({ dataset: DS })
    expect(fx.to(/\/api\/ai\/session\//)).toHaveLength(0)
    expect(info.mock.calls.flat().join(' ')).toContain('localStorage["svc"]')
  })

  it('host/backend ปฏิเสธ (นอก allowlist) → ไม่โผล่ + บอกวิธีแก้', async () => {
    const info = vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(standardRoutes({ session: () => ({ status: 403, json: { code: 403, message: 'SESSION_REFUSED', payload: { reason: 'not_in_allowlist' } } }) }))
    await mount({ dataset: DS })
    expect(fx.to(/\/bootstrap$/)).toHaveLength(0)
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
    expect(info.mock.calls.flat().join(' ')).toContain('allowlist')
  })

  it('ไม่มี secret → ไม่โผล่', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(standardRoutes({ session: () => ({ status: 403, json: { code: 403, message: 'SESSION_REFUSED', payload: { reason: 'no_secret' } } }) }))
    await mount({ dataset: DS })
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('page-config: office ยังไม่เลือก kind → ไม่อ่านอะไรจากหน้าเลย', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(standardRoutes({ config: () => ({ json: { payload: { enabled: false, reason: 'kind_not_set' } } }) }))
    await mount({ dataset: DS })
    expect(fx.calls).toHaveLength(1)
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('ai-office-backend ล่ม → หน้าหลังบ้านอยู่ครบ ไม่ throw', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    document.body.innerHTML = '<div id="office">เนื้อหาเดิม</div>'
    login(localStorage)
    fx = installFetch(standardRoutes({ config: () => ({ throws: true }) }))
    await expect(mount({ dataset: DS })).resolves.toBeUndefined()
    expect(document.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
  })

  it('โควตาเต็ม → เห็นปุ่ม + ข้อความโควตา + พิมพ์ไม่ได้', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes({ bootstrap: () => ({ json: { payload: { ...BOOT, notice: 'quota_exceeded' } } }) }))
    await mount({ dataset: DS })
    expect(document.querySelector('[data-ai-office-host]')).not.toBeNull()
    expect(logText()).toContain('โควตาของเดือนนี้ใช้ครบแล้ว')
    expect(ui().input.disabled).toBe(true)
  })
})

describe('host ตอบแบบ middleware เดิม (ก่อนถึง handler)', () => {
  it('HTTP 200 {"error":"unauthorized"} ตอนต่ออายุ → "กรุณาเข้าสู่ระบบใหม่" ไม่ใช่ "ปิดใช้งาน"', async () => {
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        session: (_c, n) =>
          n === 1 ? { json: { payload: { ticket: 'tk-1', expires_at: Math.floor(Date.now() / 1000) + 600 } } } : { json: { error: 'unauthorized' } },
        chat: () => ({ status: 401, json: { message: 'TICKET_EXPIRED' } }),
      }),
    )
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('กรุณาเข้าสู่ระบบหลังบ้านใหม่')
    expect(logText()).not.toContain('ปิดใช้งาน')
  })
})

describe('แชทจริง (SSE)', () => {
  const card = {
    id: 'card1', kind: 'ok', tool: 'withdraw_pending_by_status', title: 'รายการถอนค้าง',
    fields: [{ label: 'รวมทั้งหมด', display: '10 รายการ · 169,500.00' }],
    fetched_at: new Date().toISOString(), link: { label: 'เปิดรายการถอน', path: '/withdraw' }, cached: false,
  }

  it('แถบสถานะ → การ์ด → ข้อความทีละคำ → จำห้องไว้ถามต่อ', async () => {
    login(localStorage)
    let resolveHold!: () => void
    const hold = new Promise<void>((r) => (resolveHold = r))
    fx = installFetch(
      standardRoutes({
        chat: async (_c, n) => {
          if (n === 1) await hold
          return {
            sse: [
              ev('status', { phase: 'thinking', text: 'กำลังตรวจสอบคำถาม…', conversation_id: 'conv-1' }),
              ev('status', { phase: 'fetching', text: 'กำลังดึงข้อมูลจากระบบ…' }),
              ev('card', card),
              ev('token', { text: 'ยังมีรายการถอน' }),
              ev('token', { text: 'ค้างอยู่ตามการ์ดครับ' }),
              ev('done', { conversation_id: 'conv-1', message_id: 'm1' }),
            ],
          }
        },
      }),
    )
    await mount({ dataset: DS })
    await type('มีถอนค้างกี่รายการ')
    await until(() => logText().includes('กำลังส่งคำถาม'))
    expect(ui().send.disabled).toBe(true)
    resolveHold()
    await until(() => !w().__state().streaming)

    const t = logText()
    expect(t).toContain('มีถอนค้างกี่รายการ')
    expect(t).toContain('ยังมีรายการถอนค้างอยู่ตามการ์ดครับ')
    expect(t).toContain('169,500.00')
    expect(t).toContain('ข้อมูล ณ')
    expect(t).not.toContain('กำลังดึงข้อมูล') // แถบสถานะหายเมื่อตอบเสร็จ
    expect(w().__state().conversation_id).toBe('conv-1')
    expect(ui().send.disabled).toBe(false)

    const chat = fx.to(/\/chat$/)
    expect(chat[0].headers.authorization).toBe('Bearer tk-K11S-1')
    expect(JSON.parse(chat[0].body)).toEqual({ text: 'มีถอนค้างกี่รายการ' })

    await type('แล้วเมื่อวานล่ะ')
    await until(() => fx!.to(/\/chat$/).length === 2 && !w().__state().streaming)
    expect(JSON.parse(fx.to(/\/chat$/)[1].body).conversation_id).toBe('conv-1')
  })

  it('error event (busy) → ข้อความสุภาพ ไม่ค้าง', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes({ chat: () => ({ sse: [ev('error', { code: 'busy', message: 'ตอนนี้มีคนใช้ผู้ช่วยพร้อมกันเต็มแล้ว กรุณาลองใหม่อีกครั้งในอีกสักครู่' })] }) }))
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('พร้อมกันเต็มแล้ว')
    expect(ui().send.disabled).toBe(false)
  })

  it('ตั๋วหมดอายุ → ต่ออายุเงียบ ๆ แล้วถามต่อได้', async () => {
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        chat: (c): FakeResponse =>
          c.headers.authorization === 'Bearer tk-K11S-1'
            ? { status: 401, json: { message: 'TICKET_EXPIRED' } }
            : { sse: [ev('token', { text: 'ตอบแล้ว' }), ev('done', { conversation_id: 'c9' })] },
      }),
    )
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(fx.to(/\/api\/ai\/session\//)).toHaveLength(2)
    expect(logText()).toContain('ตอบแล้ว')
    expect(logText()).not.toContain('เข้าสู่ระบบ')
  })

  it('ต่ออายุไม่ได้เพราะ token หลังบ้านหมด → "กรุณาเข้าสู่ระบบใหม่"', async () => {
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        session: (_c, n) =>
          n === 1
            ? { json: { payload: { ticket: 'tk-1', expires_at: Math.floor(Date.now() / 1000) + 600 } } }
            : { status: 403, json: { code: 403, message: 'SESSION_REFUSED', payload: { reason: 'expired' } } },
        chat: () => ({ status: 401, json: { message: 'TICKET_EXPIRED' } }),
      }),
    )
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('กรุณาเข้าสู่ระบบหลังบ้านใหม่')
  })

  it('ระหว่างคุย service ถูกปิด → "บริการ AI ของเว็บนี้ปิดใช้งานอยู่"', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes({ chat: () => ({ status: 403, json: { message: 'SESSION_REFUSED', payload: { reason: 'service_disabled' } } }) }))
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('บริการ AI ของเว็บนี้ปิดใช้งานอยู่ ติดต่อผู้ดูแล')
  })

  it('backend ล่มระหว่างคุย → "ผู้ช่วยไม่พร้อมใช้งานชั่วคราว" ไม่ค้าง', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes({ chat: () => ({ throws: true }) }))
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('ผู้ช่วยไม่พร้อมใช้งานชั่วคราว')
    expect(ui().send.disabled).toBe(false)
  })

  it('stream หลุดกลางทาง (ไม่มี done) → บอกว่าคำตอบอาจไม่ครบ', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes({ chat: () => ({ sse: [ev('token', { text: 'กำลังตอบ' })] }) }))
    await mount({ dataset: DS })
    await type('ยอดวันนี้')
    await until(() => !w().__state().streaming)
    expect(logText()).toContain('คำตอบอาจไม่ครบ')
  })

  it('Enter ส่ง · Shift+Enter ไม่ส่ง', async () => {
    login(localStorage)
    fx = installFetch(standardRoutes())
    await mount({ dataset: DS })
    ui().input.value = 'บรรทัดแรก'
    ui().input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', shiftKey: true }))
    await tick(5)
    expect(fx.to(/\/chat$/)).toHaveLength(0)
    ui().input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter' }))
    await until(() => fx!.to(/\/chat$/).length === 1)
  })
})

describe('สลับ service / ออกจากระบบ (B-15 · AC-10)', () => {
  it('สลับกลาง stream → ยกเลิก stream + ปิดห้องเดิมด้วยตั๋วเดิม + ห้องใหม่ไม่มีคำตอบเก่า', async () => {
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        chat: () => ({ sse: [ev('status', { phase: 'thinking', text: 'กำลังตรวจสอบคำถาม…', conversation_id: 'conv-K11S' }), ev('token', { text: 'คำตอบของ K11S' })], hang: true }),
      }),
    )
    boot(DS, 20)
    await until(() => !!document.querySelector('[data-ai-office-host]'))
    await type('ยอดวันนี้')
    await until(() => logText().includes('คำตอบของ K11S'))
    expect(w().__state().streaming).toBe(true)

    localStorage.setItem('svc', 'PG99')
    await until(() => w().__state()?.service === 'PG99')

    expect(w().__state().streaming).toBe(false)
    const t = logText()
    expect(t).not.toContain('คำตอบของ K11S')
    expect(t).toContain('เปลี่ยนจากเว็บ K11S ไป เว็บ PG99')
    expect(ui().head.site.textContent).toBe('เว็บ PG99')

    const close = fx.to(/\/close$/)
    expect(close).toHaveLength(1)
    expect(close[0].url).toContain('/service/K11S/conversations/conv-K11S/close')
    expect(close[0].headers.authorization).toBe('Bearer tk-K11S-1')
    expect(JSON.parse(close[0].body).reason).toBe('switch_service')

    // ตั๋วใหม่ผูก PG99
    expect(fx.to(/\/bootstrap$/).pop()!.headers.authorization).toMatch(/^Bearer tk-PG99-/)
  })

  it('สลับไปเว็บที่ไม่ได้เปิด AI → ปุ่มหาย', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        session: (c) =>
          c.url.endsWith('/PG99')
            ? { status: 403, json: { code: 403, message: 'SESSION_REFUSED', payload: { reason: 'service_disabled' } } }
            : { json: { payload: { ticket: 'tk-K11S-1', expires_at: Math.floor(Date.now() / 1000) + 600 } } },
      }),
    )
    boot(DS, 20)
    await until(() => !!document.querySelector('[data-ai-office-host]'))
    localStorage.setItem('svc', 'PG99')
    await until(() => !document.querySelector('[data-ai-office-host]'))
  })

  it('ออกจากระบบ → ปิดห้อง (logout) + เก็บปุ่มทันที', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    login(localStorage)
    fx = installFetch(standardRoutes())
    boot(DS, 20)
    await until(() => !!document.querySelector('[data-ai-office-host]'))
    await type('สวัสดี')
    await until(() => w().__state().conversation_id === 'c1' && !w().__state().streaming)
    localStorage.removeItem('tok')
    await until(() => !document.querySelector('[data-ai-office-host]'))
    const close = fx.to(/\/close$/)
    expect(close).toHaveLength(1)
    expect(JSON.parse(close[0].body).reason).toBe('logout')
  })

  it('ล็อกอินหลังโหลดหน้า (SPA) → ปุ่มโผล่เองไม่ต้อง refresh', async () => {
    vi.spyOn(console, 'info').mockImplementation(() => {})
    fx = installFetch(standardRoutes())
    boot(DS, 20)
    await tick(60)
    expect(document.querySelector('[data-ai-office-host]')).toBeNull()
    login(localStorage)
    await until(() => !!document.querySelector('[data-ai-office-host]'))
  })
})

describe('ประวัติ 7 วัน', () => {
  it('เปิดรายการ → เลือกห้องที่ปิดแล้ว → อ่านอย่างเดียว · ถามใหม่ = ห้องใหม่', async () => {
    login(localStorage)
    fx = installFetch(
      standardRoutes({
        list: () => ({ json: { payload: { data: [{ id: 'old-1', title: 'ยอดฝากเมื่อวาน', opened_at: new Date().toISOString(), last_message_at: new Date().toISOString(), closed: true }] } } }),
        conv: () => ({
          json: {
            payload: {
              conversation: { id: 'old-1', closed: true },
              messages: [
                { id: 'u1', role: 'user', text: 'ยอดฝากเมื่อวาน', cards: [], created_at: new Date().toISOString() },
                { id: 'a1', role: 'assistant', text: 'ตามการ์ดครับ', cards: [{ id: 'k', kind: 'ok', tool: 't', title: 'ยอดฝาก', fields: [{ label: 'ยอดฝาก', display: '1,284,500.00' }], fetched_at: new Date().toISOString() }], created_at: new Date().toISOString() },
              ],
            },
          },
        }),
      }),
    )
    await mount({ dataset: DS })
    ui().head.history.click()
    await until(() => ui().hist.list.textContent.includes('ยอดฝากเมื่อวาน'))
    expect(fx.to(/\/conversations\?days=7$/)[0].headers.authorization).toBe('Bearer tk-K11S-1')
    ;(ui().hist.list.querySelector('button') as HTMLButtonElement).click()
    await until(() => logText().includes('1,284,500.00'))
    expect(logText()).toContain('อ่านอย่างเดียว')
    expect(ui().hist.box.hidden).toBe(true)

    await type('แล้ววันนี้ล่ะ')
    await until(() => fx!.to(/\/chat$/).length === 1 && !w().__state().streaming)
    expect(JSON.parse(fx.to(/\/chat$/)[0].body).conversation_id).toBeUndefined()
    expect(logText()).not.toContain('1,284,500.00')
  })
})

describe('snippet แบบไม่มี key', () => {
  it('.../widget/v1/ai-office.js → auto (backend หา office จากโดเมน)', async () => {
    const { keyFromScriptURL } = await import('../src/index')
    expect(keyFromScriptURL('https://ai.example.com/widget/v1/ai-office.js')).toBe('auto')
    expect(keyFromScriptURL('https://ai.example.com/widget/v1/pk_abc/ai-office.js')).toBe('pk_abc')
  })
})

describe('โหมด browser (หลังบ้านไม่ต้องแก้)', () => {
  const b64url = (o: unknown) => btoa(JSON.stringify(o)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  const jwt = 'h.' + b64url({ result: { _id: 'emp_1', username: 'adm_ploy', level: 7, role: { permission: [{ code: 'P004', active: 1 }, { code: 'P099', active: 0 }] } } }) + '.sig'
  const BROWSER_PA = {
    token: { source: 'localStorage', key: 'hdt', format: 'raw' },
    service: { source: 'localStorage', key: 'svc', encoding: 'none' },
    host_api_base: '{origin}/api',
    session_path: '',
    auth_scheme: 'Bearer',
    extra_headers: [{ name: 'headertoken', source: 'token' }],
    identity: { source: 'jwt', root: 'result', id: '_id', username: 'username', level: 'level', permissions: { path: 'role.permission', pluck: 'code', where_field: 'active', where_value: 1 } },
  }
  function browserRoutes(chat: any) {
    return standardRoutes({
      config: () => ({ json: { payload: { enabled: true, kind: 'k', mode: 'browser', page_auth: BROWSER_PA } } }),
      chat,
    }).concat([
      [/\/browser-session$/, () => ({ json: { payload: { ticket: 'tk-b', expires_at: Math.floor(Date.now() / 1000) + 600 } } })],
      [/\/chat\/relay\//, () => ({ json: { payload: null } })],
      [/\/api\/GetWithdraw/, () => ({ json: { code: 0, data: { rows: [1, 2] } } })],
    ] as any)
  }

  it('ขอตั๋วเองจาก backend: ส่งตัวตน + ลายนิ้วมือ token ไม่ส่ง token · ไม่เรียก /ai/session ของหลังบ้าน', async () => {
    localStorage.setItem('hdt', jwt)
    localStorage.setItem('svc', 'K11S')
    fx = installFetch(browserRoutes(undefined))
    await mount({ dataset: DS })
    expect(fx.to(/\/api\/ai\/session\//)).toHaveLength(0)
    const s = fx.to(/\/browser-session$/)
    expect(s).toHaveLength(1)
    const body = JSON.parse(s[0].body)
    expect(body.user).toEqual({ id: 'emp_1', username: 'adm_ploy', display_name: 'adm_ploy', level: 7, permissions: ['P004'] })
    expect(body.token_fp).toMatch(/^[0-9a-f]{64}$/)
    expect(JSON.stringify(fx.calls.filter((c) => c.url.startsWith('https://ai.example.com')))).not.toContain(jwt)
    expect(document.querySelector('[data-ai-office-host]')).not.toBeNull()
  })

  it('backend สั่ง fetch → ยิง API เดิมด้วย token แอดมิน → ส่งผลกลับ relay', async () => {
    localStorage.setItem('hdt', jwt)
    localStorage.setItem('svc', 'K11S')
    fx = installFetch(
      browserRoutes(() => ({
        sse: [ev('fetch', { id: 'f1', method: 'GET', path: '/GetWithdraw/K11S', query: { date: '2026-09-24' } }), ev('token', { text: 'ตอบ' }), ev('done', { conversation_id: 'c1' })],
      })),
    )
    await mount({ dataset: DS })
    await type('ถอนค้าง')
    await until(() => fx!.to(/\/chat\/relay\/f1$/).length === 1)
    const hostCall = fx.to(/\/api\/GetWithdraw/)[0]
    expect(hostCall.url).toBe(location.origin + '/api/GetWithdraw/K11S?date=2026-09-24')
    expect(hostCall.headers.authorization).toBe('Bearer ' + jwt)
    expect(hostCall.headers.headertoken).toBe(jwt)
    const relay = fx.to(/\/chat\/relay\/f1$/)[0]
    expect(relay.headers.authorization).toBe('Bearer tk-b')
    expect(JSON.parse(relay.body)).toEqual({ status: 200, body: JSON.stringify({ code: 0, data: { rows: [1, 2] } }) })
  })

  it('path แปลก (โดเมนอื่น / javascript:) → ไม่ยิงออกไปเด็ดขาด', async () => {
    const { makeHostFetcher } = await import('../src/hostfetch')
    const { genericReader } = await import('../src/pageauth')
    localStorage.setItem('hdt', jwt)
    fx = installFetch([])
    const f = makeHostFetcher(BROWSER_PA as any, genericReader)
    for (const p of ['//evil.example/x', 'https://evil.example', 'javascript:alert(1)', '/\\evil']) {
      expect((await f({ method: 'GET', path: p })).status).toBe(400)
    }
    expect(fx.calls).toHaveLength(0)
  })
})

describe('SSE parser', () => {
  it('ข้าม ping และรวม data หลายบรรทัด', () => {
    expect(parseBlock(': ping')).toBeNull()
    expect(parseBlock('event: token\ndata: {"text":"ก"}')).toEqual({ type: 'token', data: { text: 'ก' } })
    expect(parseBlock('event: x\ndata: {bad')).toBeNull()
  })
})
