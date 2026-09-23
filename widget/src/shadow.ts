import { CSS } from './styles'

export interface ShadowHandle {
  host: HTMLElement
  root: ShadowRoot
}

/**
 * สร้าง host + shadow root แบบ closed
 *
 * closed แปลว่าโค้ดของหน้า office เข้าถึง DOM ข้างในไม่ได้ผ่าน el.shadowRoot
 * และ CSS ของ office ก็ข้ามเข้ามาไม่ได้เช่นกัน
 */
export function createShadow(mountInto: Element | null): ShadowHandle {
  const host = document.createElement('div')
  host.setAttribute('data-ai-office-host', '')
  host.style.all = 'initial'

  if (mountInto) {
    // โหมด preview — วางในกล่องที่หน้า settings เตรียมไว้
    const cs = getComputedStyle(mountInto as HTMLElement)
    if (cs.position === 'static') (mountInto as HTMLElement).style.position = 'relative'
    host.style.position = 'absolute'
    host.style.inset = '0'
    mountInto.appendChild(host)
  } else {
    document.body.appendChild(host)
  }

  const root = host.attachShadow({ mode: 'closed' })

  const style = document.createElement('style')
  style.textContent = CSS
  root.appendChild(style)

  return { host, root }
}

/** โหลดฟอนต์เข้า shadow root — ไม่แตะ <head> ของ office */
export function injectFonts(root: ShadowRoot) {
  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.href =
    'https://fonts.googleapis.com/css2?family=Anuphan:wght@600;700&family=IBM+Plex+Sans+Thai:wght@400;500;600&family=IBM+Plex+Mono:wght@400;500&display=swap'
  root.appendChild(link)
}
