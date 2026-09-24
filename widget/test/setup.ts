import { Window } from 'happy-dom'

// Node 22+ มี localStorage/sessionStorage ของตัวเอง (webstorage) ซึ่งเป็น undefined ถ้าไม่ได้ตั้ง --localstorage-file
// และบังของ happy-dom ไว้ → ใส่ของ happy-dom กลับเข้าไป (ใช้ได้ทุกเวอร์ชัน Node ไม่ต้องพึ่ง flag)
const g = globalThis as unknown as Record<string, unknown>
for (const k of ['localStorage', 'sessionStorage'] as const) {
  let ok = false
  try {
    ok = !!g[k] && typeof (g[k] as Storage).getItem === 'function'
  } catch {
    ok = false
  }
  if (!ok) {
    const store = (new Window({ url: location.href }) as unknown as Record<string, Storage>)[k]
    Object.defineProperty(globalThis, k, { value: store, configurable: true, writable: true })
  }
}
