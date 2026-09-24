import { describe, it, expect } from 'vitest'
import { Window } from 'happy-dom'
import { readFileSync } from 'fs'
import { resolve } from 'path'
import { installFetch, login, standardRoutes, until, BOOT, type Handler } from './fake'

const BUNDLE = resolve(__dirname, '../../static/widget/ai-office.v1.js')
const KEY = 'pk_demo_local'

/**
 * ทดสอบเส้นทางที่หน้า office ใช้จริง — โหลด bundle ที่ build แล้วเป็น <script>
 * แล้วดูว่ามันขอ page-config → ตั๋วจาก host → bootstrap และสร้าง widget เองได้ไหม
 */
async function boot(opts: { key?: string; loggedIn?: boolean; service?: string; routes?: Array<[RegExp, Handler]>; attrs?: Record<string, string> } = {}) {
  const win = new Window({ url: 'https://office.example.com/deposit' })
  const doc = win.document
  const fx = installFetch(opts.routes ?? standardRoutes(), win)
  ;(win as any).console.info = () => {}
  doc.body.innerHTML = '<div id="office">เนื้อหาเดิม</div>'
  if (opts.loggedIn !== false) login(win.localStorage as unknown as Storage, 'host-user-token', opts.service ?? 'K11S')

  const s = doc.createElement('script')
  // inline script ไม่มี .src ให้แกะ key — จำลอง snippet ด้วย data-public-key แทน
  s.setAttribute('data-public-key', opts.key ?? KEY)
  s.setAttribute('data-api-base', 'https://ai.example.com')
  for (const [k, v] of Object.entries(opts.attrs ?? {})) s.setAttribute(k, v)
  s.textContent = readFileSync(BUNDLE, 'utf8')
  doc.head.appendChild(s)
  await new Promise((r) => setTimeout(r, 30))
  return { win, doc, fx }
}

const hasWidget = (doc: any) => !!doc.querySelector('[data-ai-office-host]')

describe('auto-boot จาก <script>', () => {
  it('page-config → ตั๋วจาก host → bootstrap(ตั๋ว) แล้วสร้าง widget เอง', async () => {
    const { doc, fx } = await boot()
    await until(() => hasWidget(doc))
    expect(fx.calls[0].url).toBe(`https://ai.example.com/api/ai/office/${KEY}/page-config`)
    expect(fx.to(/\/api\/ai\/session\//)[0].url).toBe('https://office.example.com/api/ai/session/K11S')
    expect(fx.to(/\/bootstrap$/)[0].url).toBe(`https://ai.example.com/api/ai/office/${KEY}/service/K11S/bootstrap`)
    expect(doc.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
  })

  it('data-host-api-base ทับที่อยู่ API ของ host ได้ (host API คนละโดเมนกับหน้า)', async () => {
    const { doc, fx } = await boot({ attrs: { 'data-host-api-base': 'https://api.office.example.com/api' } })
    await until(() => hasWidget(doc))
    expect(fx.to(/\/api\/ai\/session\//)[0].url).toBe('https://api.office.example.com/api/ai/session/K11S')
  })

  it('bootstrap enabled=false → ไม่มี DOM ของ widget เลย', async () => {
    const { doc, fx } = await boot({ routes: standardRoutes({ bootstrap: () => ({ json: { payload: { enabled: false, reason: 'service_disabled' } } }) }) })
    await until(() => fx.to(/\/bootstrap$/).length === 1)
    await new Promise((r) => setTimeout(r, 20))
    expect(hasWidget(doc)).toBe(false)
  })

  it('ยังไม่ล็อกอิน → ขอแค่ page-config ไม่ขอตั๋ว ไม่โผล่', async () => {
    const { doc, fx } = await boot({ loggedIn: false })
    expect(fx.calls.map((c) => c.url)).toEqual([`https://ai.example.com/api/ai/office/${KEY}/page-config`])
    expect(hasWidget(doc)).toBe(false)
  })

  it('ล็อกอินหลังโหลดหน้าแล้ว (SPA) → ปุ่มโผล่เองไม่ต้อง refresh', async () => {
    const { win, doc, fx } = await boot({ loggedIn: false })
    expect(hasWidget(doc)).toBe(false)
    login(win.localStorage as unknown as Storage)
    await until(() => hasWidget(doc), 3000)
    expect(fx.to(/\/api\/ai\/session\//)).toHaveLength(1)
  })

  it('สลับ service แล้ว URL เปลี่ยนตาม โดย snippet ไม่ต้องแก้', async () => {
    const { doc, fx } = await boot({ service: 'PG99' })
    await until(() => hasWidget(doc))
    expect(fx.to(/\/bootstrap$/)[0].url).toContain(`/api/ai/office/${KEY}/service/PG99/bootstrap`)
  })

  it('ไม่มี public key → ไม่ยิง API และไม่สร้าง DOM', async () => {
    const { doc, fx } = await boot({ key: '' })
    expect(fx.calls).toHaveLength(0)
    expect(hasWidget(doc)).toBe(false)
  })

  it('ai-office-backend ล่ม → หน้า office ยังอยู่ครบ ไม่พังตาม', async () => {
    const { doc } = await boot({ routes: standardRoutes({ config: () => ({ throws: true }) }) })
    expect(doc.getElementById('office')!.innerHTML).toBe('เนื้อหาเดิม')
    expect(hasWidget(doc)).toBe(false)
  })

  it('ใช้ bootstrap ตามที่ server ส่ง (ชื่อเว็บบนหัว)', async () => {
    const { doc } = await boot({ routes: standardRoutes({ bootstrap: () => ({ json: { payload: { ...BOOT, service_label: 'เว็บทดสอบ' } } }) }) })
    await until(() => hasWidget(doc))
    const cfg = (doc.defaultView as any).__aiOffice.__config()
    expect(cfg.service_label).toBe('เว็บทดสอบ')
  })
})

describe('bundle', () => {
  it('ไม่มีชื่อช่อง/ชื่อหลังบ้านเฉพาะ kind ฝังอยู่ (กฎปลั๊ก spec §5.3)', () => {
    const code = readFileSync(BUNDLE, 'utf8')
    for (const s of ['auth_token', 'web-service', 'headertoken', 'office-v10x', 'office-abatech', 'office-api-v10', 'GOTOPOPOFFICE']) {
      expect(code).not.toContain(s)
    }
  })
})
