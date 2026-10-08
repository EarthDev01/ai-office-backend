import { describe, it, expect } from 'vitest'
import { renderCard } from '../src/ui'
import type { Card } from '../src/types'

const base = (over: Partial<Card>): Card => ({
  id: 'c', kind: 'ok', tool: 't', title: 'รายการฝากของสมาชิก', fields: [], fetched_at: new Date().toISOString(),
  link: { label: 'เปิดรายการฝาก', path: '/Deposit' }, ...over,
})
const table = {
  columns: [{ label: 'เวลา', format: 'datetime' }, { label: 'ยอดฝาก', format: 'money' }, { label: 'โบนัส', format: 'money' }, { label: 'สถานะ', format: 'status:deposit' }],
  rows: [[{ display: '01/10 10:06' }, { display: '10.00' }, { display: '0.10' }, { display: 'สำเร็จ', tone: 'ok' as const }]],
}

describe('แบบการ์ด', () => {
  it('stat — field แรกตัวใหญ่ ที่เหลือเป็นป้าย', () => {
    const box = renderCard(base({ style: 'stat', fields: [
      { label: 'ยอดฝากสำเร็จ', display: '700.00 บาท' }, { label: 'สำเร็จ', display: '2 รายการ' },
    ] }))
    expect(box.querySelector('.dc-statval')!.textContent).toBe('700.00 บาท')
    expect(box.querySelector('.chip')!.textContent).toBe('สำเร็จ 2 รายการ')
  })

  it('list — เวลา | เงินตัวใหญ่ + ค่าอื่นบรรทัดเล็ก | ป้ายสถานะตามสี', () => {
    const box = renderCard(base({ style: 'list', fields: [{ label: 'พบ', display: '1 รายการ' }], table }))
    expect(box.querySelector('.dc-li-time')!.textContent).toBe('01/10 10:06')
    expect(box.querySelector('.dc-li-val')!.textContent).toBe('10.00')
    expect(box.querySelector('.dc-li-sub')!.textContent).toBe('โบนัส 0.10')
    expect(box.querySelector('.pill.t-ok')!.textContent).toBe('สำเร็จ')
    expect(box.querySelector('.dc-meta')!.textContent).toBe('พบ 1 รายการ')
  })

  it('single — สถานะเป็นหัว ค่าที่เหลือเป็นช่อง', () => {
    const box = renderCard(base({ style: 'single', table }))
    expect(box.querySelector('.dc-single-head .pill')!.textContent).toBe('สำเร็จ')
    expect([...box.querySelectorAll('.dc-cell span')].map((x) => x.textContent)).toEqual(['เวลา', 'ยอดฝาก', 'โบนัส'])
  })

  it('table — สถานะเป็นป้ายสี · ไม่ระบุแบบ = ตาราง', () => {
    const box = renderCard(base({ table: { ...table, rows: [[...table.rows[0].slice(0, 3), { display: 'ไม่สำเร็จ', tone: 'bad' }]] } }))
    expect(box.querySelector('table')).not.toBeNull()
    expect(box.querySelector('td .pill.t-bad')!.textContent).toBe('ไม่สำเร็จ')
  })

  it('ไม่พบข้อมูล ใช้กรอบเดิมแม้ตั้งแบบไว้', () => {
    const box = renderCard(base({ kind: 'not_found', style: 'stat', note: 'ไม่พบ' }))
    expect(box.querySelector('.dc-statval')).toBeNull()
    expect(box.querySelector('.dc-note')!.textContent).toBe('ไม่พบ')
  })
})
