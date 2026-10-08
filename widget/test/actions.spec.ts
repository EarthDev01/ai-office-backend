import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { canRun, runPageAction } from '../src/actions'
import { guestID } from '../src/page'
import { renderCard } from '../src/ui'
import type { PageAction } from '../src/page'

const w = window as unknown as { $nuxt?: unknown }

describe('ปุ่มใต้คำตอบ — สั่งหน้าเว็บตามที่ connector ประกาศเท่านั้น', () => {
  afterEach(() => {
    delete w.$nuxt
    document.body.innerHTML = ''
  })

  it('bv_modal → $nuxt.$bvModal.show(id)', () => {
    const show = vi.fn()
    w.$nuxt = { $bvModal: { show } }
    expect(runPageAction({ id: 'promotion', label: 'ดูโปรโมชั่น', open: { bv_modal: 'bv-modal-promotion' } })).toBe(true)
    expect(show).toHaveBeenCalledWith('bv-modal-promotion')
  })

  it('bv_modal แต่หน้าไม่ใช่ Nuxt → ทำไม่ได้ (ให้แชทบอกผู้ใช้)', () => {
    expect(runPageAction({ id: 'x', label: 'x', open: { bv_modal: 'bv-modal-x' } })).toBe(false)
  })

  it('click → กดปุ่มจริงของหน้า · ไม่มีปุ่มในหน้า = ไม่วาดปุ่ม', () => {
    const a: PageAction = { id: 'deposit', label: 'ฝากเงิน', open: { click: '#buttonFooterdeposit a' } }
    expect(canRun(a)).toBe(false)
    document.body.innerHTML = '<div id="buttonFooterdeposit"><a>ฝาก</a></div>'
    const clicked = vi.fn()
    document.querySelector('#buttonFooterdeposit a')!.addEventListener('click', clicked)
    expect(canRun(a)).toBe(true)
    expect(runPageAction(a)).toBe(true)
    expect(clicked).toHaveBeenCalledOnce()
  })

  it('selector ผิดรูปไม่ทำให้พัง', () => {
    expect(canRun({ id: 'x', label: 'x', open: { click: '##[' } })).toBe(false)
  })

  it('path → router ของหน้า · ห้ามออกนอกเว็บ', () => {
    const push = vi.fn(() => undefined)
    w.$nuxt = { $router: { push } }
    expect(runPageAction({ id: 'guide', label: 'คู่มือ', open: { path: '/guide' } })).toBe(true)
    expect(push).toHaveBeenCalledWith('/guide')
    expect(runPageAction({ id: 'evil', label: 'x', open: { path: '//evil.test/x' } })).toBe(false)
  })
})

describe('guest_id ของผู้ที่ยังไม่ล็อกอิน', () => {
  beforeEach(() => localStorage.clear())

  it('สุ่มครั้งแรกแล้วใช้ค่าเดิมตลอด (ผ่านรูปแบบที่ server รับ)', () => {
    const a = guestID()
    expect(a).toMatch(/^[A-Za-z0-9_-]{16,64}$/)
    expect(guestID()).toBe(a)
    expect(localStorage.getItem('ai-office:guest_id')).toBe(a)
  })

  it('ค่าที่ถูกแก้ใน storage จนผิดรูป → สุ่มใหม่', () => {
    localStorage.setItem('ai-office:guest_id', 'bad id!')
    expect(guestID()).toMatch(/^[A-Za-z0-9_-]{16,64}$/)
  })
})

describe('การ์ดแบบหน้าเว็บผู้เล่น (inline)', () => {
  it('ตารางเป็นรายการในฟอง · ไม่มีลิงก์', () => {
    const box = renderCard(
      {
        id: 'c1', kind: 'ok', tool: 'list_promotions', title: 'โปรโมชั่นที่รับได้ตอนนี้', fields: [],
        table: {
          columns: [{ label: 'โปรโมชั่น' }, { label: 'โบนัส' }, { label: 'ฝากขั้นต่ำ' }],
          rows: [[{ display: 'โบนัสทุกยอดฝาก' }, { display: '10%' }, { display: '100.00' }]],
        },
        fetched_at: new Date().toISOString(),
        link: { label: 'ไปหน้าหลัก', path: '/' },
      },
      { inline: true },
    )
    expect(box.classList.contains('inline')).toBe(true)
    expect(box.querySelector('table')).toBeNull()
    expect(box.querySelector('a')).toBeNull()
    expect(box.querySelector('.dc-item b')!.textContent).toBe('โบนัสทุกยอดฝาก')
    expect(box.querySelector('.dc-sub')!.textContent).toBe('โบนัส 10% · ฝากขั้นต่ำ 100.00')
  })
})
