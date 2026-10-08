import type { PageAction } from './page'

/** สิ่งที่ปุ่มใช้จากหน้าเว็บ Nuxt 2 (window.$nuxt) — ไม่มีก็ข้าม */
interface NuxtLike {
  $bvModal?: { show(id: string): void }
  $router?: { push(path: string): unknown }
}

function query(selector: string): HTMLElement | null {
  try {
    return document.querySelector(selector) as HTMLElement | null
  } catch {
    return null // selector ผิดรูป
  }
}

/** วาดปุ่มนี้ได้ไหม — ปุ่มแบบ click ต้องมีปุ่มจริงอยู่ในหน้าตอนนี้ (แบรนด์ที่ไม่มีเมนูนั้นจะไม่เห็นปุ่ม) */
export function canRun(a: PageAction): boolean {
  if (a.open.click) return query(a.open.click) !== null
  return true
}

/**
 * สั่งหน้าเว็บตามที่ connector ประกาศ (id มาจาก server · วิธีสั่งมาจาก page-config) — คืน false = ทำไม่ได้
 * ไม่ eval อะไรจาก config: มีแค่ 3 แบบนี้
 */
export function runPageAction(a: PageAction): boolean {
  const o = a.open
  const nuxt = (window as unknown as { $nuxt?: NuxtLike }).$nuxt
  try {
    if (o.bv_modal) {
      if (!nuxt?.$bvModal) return false
      nuxt.$bvModal.show(o.bv_modal)
      return true
    }
    if (o.click) {
      const el = query(o.click)
      if (!el) return false
      el.click()
      return true
    }
    if (o.path && o.path.startsWith('/') && !o.path.startsWith('//')) {
      if (nuxt?.$router) {
        // vue-router บางรุ่นคืน promise ที่ reject เมื่ออยู่หน้าเดิม — ไม่ต้องสนใจ
        void Promise.resolve(nuxt.$router.push(o.path)).catch(() => undefined)
      } else {
        location.assign(o.path)
      }
      return true
    }
  } catch {
    /* หน้าเว็บพัง ไม่ลาก widget พังด้วย */
  }
  return false
}
