import type { Bootstrap } from './types'

export interface UI {
  launcher: HTMLButtonElement
  panel: HTMLDivElement
  head: { avatar: HTMLDivElement; name: HTMLDivElement; site: HTMLDivElement }
  log: HTMLDivElement
  input: HTMLInputElement
  send: HTMLButtonElement
}

export function buildUI(root: ShadowRoot): UI {
  const launcher = el('button', 'launcher') as HTMLButtonElement
  launcher.type = 'button'
  launcher.setAttribute('aria-label', 'เปิดผู้ช่วยหลังบ้าน')

  const panel = el('div', 'panel') as HTMLDivElement
  panel.dataset.open = 'false'

  const head = el('div', 'head')
  const avatar = el('div', 'avatar') as HTMLDivElement
  const nameWrap = el('div')
  const name = el('div', 'name') as HTMLDivElement
  nameWrap.appendChild(name)
  const site = el('div', 'site') as HTMLDivElement
  const close = el('button', 'x') as HTMLButtonElement
  close.type = 'button'
  close.textContent = '×'
  close.setAttribute('aria-label', 'ปิด')
  head.append(avatar, nameWrap, site, close)

  const log = el('div', 'log') as HTMLDivElement

  const foot = el('div', 'foot')
  const input = el('input') as HTMLInputElement
  input.type = 'text'
  input.placeholder = 'พิมพ์คำถาม…'
  const send = el('button') as HTMLButtonElement
  send.type = 'button'
  send.textContent = '↑'
  foot.append(input, send)

  panel.append(head, log, foot)
  root.append(launcher, panel)

  close.addEventListener('click', () => (panel.dataset.open = 'false'))

  return { launcher, panel, head: { avatar, name, site }, log, input, send }
}

/** ข้อความทุกชิ้นลงด้วย textContent — ไม่มี innerHTML กับข้อความที่ไม่ได้มาจากเรา */
export function addBubble(log: HTMLDivElement, who: 'me' | 'ai', text: string): HTMLDivElement {
  const row = el('div', 'row ' + who)
  const b = el('div', 'bubble') as HTMLDivElement
  b.textContent = text
  row.appendChild(b)
  log.appendChild(row)
  log.scrollTop = log.scrollHeight
  return b
}

export function addNote(log: HTMLDivElement, text: string) {
  const n = el('div', 'note')
  n.textContent = text
  log.appendChild(n)
  log.scrollTop = log.scrollHeight
}

/** วางปุ่มลอยและ panel ตาม placement ที่ server ส่งมา */
export function applyPlacement(ui: UI, b: Bootstrap) {
  const { position, offset_x, offset_y } = b.placement
  const left = position === 'bottom-left'

  for (const node of [ui.launcher, ui.panel]) {
    node.style.left = ''
    node.style.right = ''
  }

  ui.launcher.style.bottom = px(offset_y)
  ui.panel.style.bottom = px(offset_y + 68)

  if (left) {
    ui.launcher.style.left = px(offset_x)
    ui.panel.style.left = px(offset_x)
  } else {
    ui.launcher.style.right = px(offset_x)
    ui.panel.style.right = px(offset_x)
  }
}

export function applyAppearance(ui: UI, b: Bootstrap) {
  ui.head.name.textContent = b.display_name || 'ผู้ช่วยหลังบ้าน'
  // ป้ายชื่อ service ค้างบนหัวตลอด — แอดมินต้องรู้ตลอดว่ากำลังคุยเรื่อง service ไหน
  ui.head.site.textContent = b.service_label || b.service_id || ''

  ui.head.avatar.textContent = ''
  ui.launcher.textContent = ''
  if (b.avatar_url) {
    ui.head.avatar.appendChild(img(b.avatar_url))
    ui.launcher.appendChild(img(b.avatar_url))
  } else {
    ui.head.avatar.textContent = 'AI'
    ui.launcher.textContent = 'AI'
  }
}

function img(src: string): HTMLImageElement {
  const i = document.createElement('img')
  i.src = src
  i.alt = ''
  i.referrerPolicy = 'no-referrer'
  return i
}

function px(n: number) {
  return `${n | 0}px`
}

function el(tag: string, cls?: string): HTMLElement {
  const e = document.createElement(tag)
  if (cls) e.className = cls
  return e
}
