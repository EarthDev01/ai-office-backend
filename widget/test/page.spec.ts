import { describe, it, expect, beforeEach } from 'vitest'
import { readToken, readService, describeSource } from '../src/page'

beforeEach(() => localStorage.clear())

describe('token ที่เก็บพร้อมคำนำหน้า (เช่น @nuxtjs/auth เก็บ "Bearer eyJ…")', () => {
  const src = { source: 'localStorage', key: 'auth._token.local', format: 'raw' }
  it('ตัด Bearer ออก — widget ใส่ scheme ให้เองตอนยิง', () => {
    localStorage.setItem(src.key, 'Bearer eyJabc.def.ghi')
    expect(readToken(src)).toBe('eyJabc.def.ghi')
  })
  it('ตัวพิมพ์เล็ก/ช่องว่างเกินก็ตัด', () => {
    localStorage.setItem(src.key, '  bearer   eyJabc  ')
    expect(readToken(src)).toBe('eyJabc')
  })
  it('ออกจากระบบ (false / "Bearer " ว่าง) = ไม่มี token', () => {
    localStorage.setItem(src.key, 'false')
    expect(readToken(src)).toBe('')
    localStorage.setItem(src.key, 'Bearer ')
    expect(readToken(src)).toBe('')
  })
  it('token ปกติไม่มีคำนำหน้า → ไม่แตะ', () => {
    localStorage.setItem(src.key, 'eyJraw')
    expect(readToken(src)).toBe('eyJraw')
  })
})

describe('service แบบ 1 โดเมน = 1 เว็บ', () => {
  it('ใช้ค่าที่ backend ส่งมาใน page-config', () => {
    expect(readService({ source: 'office', key: '', value: 'TANGTEM' })).toBe('TANGTEM')
  })
  it('office ยังไม่มี service ที่เปิด → ว่าง (ไม่โชว์ปุ่ม)', () => {
    expect(readService({ source: 'office', key: '' })).toBe('')
  })
  it('ข้อความบอกสาเหตุให้คนติดตั้งเข้าใจ', () => {
    expect(describeSource({ source: 'office', key: '' })).toContain('คอนโซล')
  })
})
