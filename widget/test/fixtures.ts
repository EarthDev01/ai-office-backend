/** page-config ตาม connectors/<kind>/host.yaml (page_auth) — backend ส่งแบบนี้ให้ widget */
export const V10X_PAGE = {
  kind: 'office-v10x',
  mode: 'browser',
  host_api_base: 'https://office.test/api',
  token: { source: 'localStorage', key: 'auth_token', format: 'json-expiration', value_field: 'value', expiration_field: 'expiration' },
  service: { source: 'localStorage', key: 'web-service', encoding: 'none' },
  auth_scheme: 'Bearer',
}

export const ABATECH_PAGE = {
  kind: 'office-abatech',
  mode: 'browser',
  host_api_base: 'http://localhost:7777/api',
  token: { source: 'localStorage', key: 'headertoken', format: 'raw' },
  service: { source: 'query', key: 'service', encoding: 'base64' },
  auth_scheme: 'Bearer',
  identity: {
    root: 'result',
    permissions_token: { source: 'localStorage', key: 'token', format: 'raw' },
    permissions: { path: 'role.permission', pluck: 'code', where_field: 'Isactive', where_value: 1 },
  },
}

export const isPageConfig = (url: unknown) => String(url).endsWith('/api/ai/widget/page-config')

/** JWT ที่ payload อ่านได้ (ลายเซ็นปลอม — widget ไม่ตรวจลายเซ็น) */
export function fakeJWT(payload: unknown): string {
  const b64 = (s: string) => btoa(unescape(encodeURIComponent(s))).replace(/=+$/, '').replace(/\+/g, '-').replace(/\//g, '_')
  return `${b64('{"alg":"HS256","typ":"JWT"}')}.${b64(JSON.stringify(payload))}.sig`
}

export function loginV10x(service = 'K11S', token = 'jwt-abc') {
  localStorage.setItem('auth_token', JSON.stringify({ value: token, expiration: Math.floor(Date.now() / 1000) + 600 }))
  if (service) localStorage.setItem('web-service', service)
}
