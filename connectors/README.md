# connectors/ — ปลั๊กต่อชนิดหลังบ้าน (kind)

> เอกสาร dev · ใช้คู่กับ `Docs-workflow/w-17-ai-office/spec-v2.md` §5.3, §6, §7.4 และ `contract-host-ai.md`
> 1 โฟลเดอร์ = 1 kind (`office-v10x`, `office-abatech`) · **ทุกอย่างที่ต่างกันระหว่างหลังบ้านอยู่ที่นี่เท่านั้น** — โค้ดกลาง (`internal/core`, `widget/src`) ห้ามมีชื่อ/พฤติกรรมของ kind ใด (`test/plugin_lint_test.go` บังคับ)

```
connectors/<kind>/
├── host.yaml            วิธีอ่านล็อกอินบนหน้า · envelope ของ host · timezone · ข้อเท็จจริงคงที่
├── permissions.yaml     rule สิทธิ์ (mirror UI เดิมของ host · D-91)
├── statuses.yaml        ตารางสถานะ + "ต้องทำอะไรต่อ" (BQ-29)
├── menus.yaml           เมนู/แท็บจริง + วิธีทำ (guides) — ไม่ hardcode ในโค้ด (BQ-13, 48, 49, 51)
├── questions/*.yaml     tool อ่านอย่างเดียว (1 ไฟล์ต่อเรื่อง)
└── contract-tests/*.yaml response ตัวอย่างของ host → การ์ดที่คาดหวัง (C10)
```

โหลดตอน boot (`CONNECTORS_DIR`, ค่าเริ่มต้น `./connectors`) · **ผิดรูปแบบ = ไม่ boot** พร้อมบอก `ไฟล์:บรรทัด` · ตรวจเองได้ด้วย `go test ./test/ -run RealConnectors`

---

## 0. โหมดการดึงข้อมูล (`host.yaml` → `mode`)

| mode | ใครยิงหลังบ้าน | หลังบ้านต้องแก้ | ใช้เมื่อ |
|---|---|---|---|
| **`browser`** (ทางที่ทีมเลือก 24/09) | widget ในหน้าแอดมิน ยิง **API เดิม** ด้วย token ของแอดมินเอง แล้วส่งผลให้ backend | **ไม่ต้อง** | ค่าเริ่มต้นของ office ใหม่ |
| `host` | backend ยิง `/api/ai/read/*` ด้วยกุญแจดอกเล็ก | ต้องทำตาม `contract-host-ai.md` | ต้องการตัดข้อมูลอ่อนไหวที่ต้นทาง / คำถามที่ API เดิมทำไม่ได้ |

โหมด browser:
- widget อ่านตัวตนจาก `page_auth.identity` (decode JWT ของหน้า หรือยิง path ที่ระบุ) → ขอตั๋วจาก backend เอง (`/browser-session`) — ไม่ตรวจลายเซ็น แต่ข้อมูลทุกชิ้นยังคุมโดยหลังบ้านเพราะยิงด้วย token ของแอดมินคนนั้น
- ประวัติแชทผูกกับลายนิ้วมือ token (รอบล็อกอิน) — อ้างชื่อผู้อื่นแล้วเปิดประวัติเขาไม่ได้
- tool call ชี้ **path เดิมของหลังบ้าน** (สัมพัทธ์กับ `host_api_base`) · ต้องมี `{service}`/`{service_b64}` ใน path/query/body หรือประกาศ `scope: office` (ข้อมูลระดับ office เช่นบิล)
- ใช้เฉพาะ endpoint **อ่านอย่างเดียว** (ห้าม endpoint ที่เขียน DB/แคชรายงาน/ยืนยันบิล) · field อ่อนไหวไม่ขึ้นการ์ดและไม่ถึงโมเดล (backend เลือกแค่ค่าที่ประกาศใน `values`)
- ไม่ต้องมี `session_path` / `host_api.scoped_token_*`

```yaml
mode: browser
page_auth:
  token: {source: localStorage, key: headertoken, format: raw}
  service: {source: query, key: service, encoding: base64}
  host_api_base: "{origin}/api"          # dev ที่ API คนละโดเมน → snippet ใส่ data-host-api-base
  extra_headers: [{name: headertoken, source: token}]   # header เพิ่มที่ API เดิมต้องการ
  identity:
    source: jwt                           # หรือ request + method/path (ยิงหลังบ้านถามว่าใครล็อกอิน)
    root: result                          # object ผู้ใช้ใน payload
    id: _id
    username: username
    display_name: full_name
    level: level
    dept: deptcode
    permissions: {path: role.permission, pluck: code, where_field: isactive, where_value: 1}
host_api:
  envelope: {code_field: code, ok_codes: [0], data_field: data}
```

ตัวอย่างครบ: `internal/core/connector/testdata/sample-browser/`

## 1. `host.yaml`

```yaml
kind: office-v10x                    # ต้องตรงชื่อโฟลเดอร์
label: หลังบ้าน v10x
page_auth:                           # widget อ่านตามนี้ (ส่งผ่าน page-config · ไม่มีข้อมูลลับ)
  token:   {source: localStorage, key: <ชื่อช่อง>, format: raw | json-expiration, value_field: value, expiration_field: expiration}
  service: {source: localStorage | sessionStorage | query, key: <ชื่อช่อง>, encoding: none | base64}
  host_api_base: "{origin}/api"      # snippet ทับได้ด้วย data-host-api-base
  session_path: "/ai/session/{service}"
  auth_scheme: Bearer
host_api:
  scoped_token_path: /api/ai/scoped-token
  scoped_token_header: X-AI-Scoped-Token     # ห้ามเป็น Authorization
  envelope: {code_field: code, ok_codes: [0], data_field: data, message_field: message}
  timeout_ms: 8000
timezone: Asia/Bangkok
day_cutoff: "00:00"
support_message: ติดต่อทีม support ในห้องแชทประจำ
facts:                               # host_facts — ข้อเท็จจริงคงที่ + ข้อที่ "ระบบทำให้ไม่ได้" (ตอบตามจริง)
  - {id: no_forecast, title: คาดการณ์เครดิตหมด, text: ..., keywords: [หมดเมื่อไหร่], questions: [BQ-43]}
```

## 2. `permissions.yaml`

```yaml
rules:
  withdraw: {any_of: [P004], menu: รายการถอน, grant_hint: ให้ผู้ดูแลระบบเปิดสิทธิ์เมนูรายการถอน}
  dashboard: {all_of: [DASHBOARDALL], min_level: 6, menu: ...}
  credit_system: {logged_in: true}
```
- `any_of` มีตัวใดตัวหนึ่ง · `all_of` ต้องมีครบ · `min_level` ≥ n · `logged_in` แค่ล็อกอิน · rule ว่าง = **ไม่มีใครผ่าน** (fail closed)
- code ต้องเป็น code ที่ host ใส่ใน `user.permissions` ตอนขอตั๋ว และต้องตรงกับ `AIRequire` ของ route ฝั่ง host (ไม่งั้น backend ผ่านแต่ host ตอบ 403 = การ์ด "สิทธิ์ไม่ถึง")
- `menu` + `grant_hint` = สิ่งที่ผู้ใช้เห็นเมื่อสิทธิ์ไม่ถึง (B-12 · ห้ามบอกวิธีข้าม)

## 3. `statuses.yaml`

```yaml
unknown_message: สถานะนี้ยังไม่มีคำอธิบายในระบบ ...
tables:
  withdraw:
    label: สถานะรายการถอน
    source: office-v10x src/components/History/HistoryWithdraw.vue:88-102   # ที่มาให้คนรีวิวตามได้
    questions: [BQ-29]
    entries:
      - {code: 1, label: สำเร็จ, aliases: [โอนแล้ว], meaning: ..., next_action: ..., final: true}
```
- ใช้ใน `format: "status:withdraw"` (ป้ายบนการ์ด) · `model_context type: labels` · built-in `explain_status`
- code ที่ไม่มีในตาราง → แสดง "ไม่ทราบสถานะ (N)" + `unknown_message` (B-10 · ห้ามเดา)

## 4. `menus.yaml`

```yaml
menus:
  - {id: withdraw, name: รายการถอน, path: /withdraw, group: ธุรกรรม, permission: withdraw, keywords: [ถอน], description: ..., tabs: [{name: ..., permission: ...}]}
guides:
  - {id: add_credit, title: เพิ่มเครดิตให้สมาชิก, menu: credit, tab: แจกโบนัสฟรี, steps: [...], fields: [...], note: ..., questions: [BQ-48, BQ-51], keywords: [...]}
```
- `permission` = ชื่อ rule (ไม่ใช่ code) · "" = ทุกคนที่ล็อกอิน · built-in `lookup_menu` กรองตามสิทธิ์ของผู้ถาม
- ชื่อเมนู/แท็บ/steps **โมเดลพูดซ้ำได้** (อยู่ใน allowlist ของ output guard) แต่ **ตัวเลข** ใน steps/facts/meaning จะถูก guard ตัด → อย่าใส่ตัวเลขในข้อความเหล่านี้

## 5. `questions/*.yaml` — tool

```yaml
questions: [BQ-03]          # ระดับไฟล์ (ข้อมูลประกอบ)
tools:
  - name: withdraw_pending_by_status        # a-z 0-9 _ · ห้ามชนชื่อ built-in
    description: รายการถอนที่ยังไม่จบ แยกตามสถานะ (บอกโมเดลว่าใช้เมื่อไหร่)
    questions: [BQ-03]                       # ใช้คำนวณ coverage 31 ข้อ
    permission: withdraw                     # ชื่อ rule
    freshness: summary | live                # live = ยอดเงินสด ห้ามแคช (B-7)
    input:                                   # ค่าที่โมเดลส่งมา — string ต้องมี pattern หรือ enum
      username: {type: string, required: true, pattern: "^[A-Za-z0-9_.@-]{2,40}$", description: ...}
      date: {type: date, description: YYYY-MM-DD}
    calls:
      - id: list
        method: GET | POST
        path: /api/ai/read/withdraw-pending-summary/{service}   # ต้องขึ้นต้น /api/ai/read/ และลงท้าย /{service}
        query: {date: "{date:input.date|date:today}"}
        body: {start_date: "{date:today-6}"}                   # POST เท่านั้น
        cache_ttl: 60                                           # 0–60 · live ห้ามแคช
        timeout_ms: 8000
        envelope: ""                                            # "" = ตาม host · raw = ใช้ body ทั้งก้อน
        not_found_codes: [404]                                  # HTTP status หรือ body code ที่แปลว่า "ไม่พบ"
        optional: false                                         # true = ล้มได้ไม่ล้มทั้ง tool
        schema: {...}                                           # บังคับ (P-11) — ดู §5.3
    values: [...]                                               # §5.4
    not_found: {when: "empty(rows)", message: ไม่มีรายการถอนค้าง}
    card: {...}                                                 # §5.5
    model_context: [...]                                        # §5.6
```

### 5.1 template (path/query/body/link)

| placeholder | ค่า |
|---|---|
| `{service}` / `{service_b64}` | service ของตั๋ว (ไม่ใช่ค่าจากผู้ใช้) / base64 ของมัน |
| `{input.x}` | ค่า input ที่ผ่านการตรวจแล้ว |
| `{date:today}` `{date:yesterday}` `{date:today-6}` `{date:month_start}` `{date:prev_month_end}` … | `YYYY-MM-DD` ตาม timezone ของ connector |
| `{datetime:today}` / `{datetime_end:today}` | `YYYY-MM-DD 00:00:00` / `23:59:59` |
| `{month:today}` · `{unix:now}` | `YYYY-MM` · วินาที |
| `a\|b` | ใช้ a ถ้ามีค่า ไม่งั้น b — เช่น `{date:input.date\|date:today}` |

ค่าใน body ที่เป็น placeholder ตัวเดียวล้วนจะคงชนิดเดิม (ตัวเลขยังเป็นตัวเลข)

### 5.2 response ของ host → data

`DecodeResponse` (`internal/core/connector/decode.go` — ตัวเดียวกับ contract test):
HTTP 401/403/429 → unauthorized/denied/busy · HTTP อื่นนอก 2xx → **ไม่พบ** ถ้าอยู่ใน `not_found_codes` ไม่งั้น `http_error` · 2xx → `code_field` ต้องอยู่ใน `ok_codes` (หรือ `not_found_codes` = ไม่พบ) → เอา `data_field` · แล้วตรวจ schema

### 5.3 schema (JSON Schema ชุดย่อย)

`type` (object array string number integer boolean null หรือ list) · `properties` · `required` · `items` · `nullable` · field ที่ไม่ได้ประกาศยอมให้มี · required หาย/ผิดชนิด = `schema_mismatch` = การ์ด "ดึงไม่ได้" **ไม่มีตัวเลขบางส่วน**

### 5.4 values (คำนวณตามลำดับ)

```yaml
- {name: rows, from: list, path: items, filter: "item.status != 1", sort_by: count, order: desc, limit: 10}
- {name: total, from: list, path: items, agg: sum, field: amount}        # count sum min max avg first last
- {name: by_bank, from: list, path: items, group_by: bank_code, field: amount}   # ได้ [{key, count, sum}]
- {name: diff, expr: "today_total - yesterday_total"}
- {name: x, from: c, path: a.b, default: 0}
```
expression: ตัวเลข · `'str'` · `true false null` · ชื่อ (`a`, `item.status`, `input.x`, `a[0]`) · `+ - * / %` `== != < <= > >=` `&& || !` · ฟังก์ชัน `len count empty sum(x,"f") min max abs round(x,n) coalesce contains in lower`

### 5.5 card (ค่าจากระบบตรง ๆ — ไม่ผ่านโมเดล)

```yaml
card:
  title: รายการถอนที่ยังไม่สำเร็จ
  fields: [{label: รวม, value: total, format: money, prefix: "", suffix: " บาท", when: "total > 0"}]
  table: {rows: rows, max_rows: 20, columns: [{label: สถานะ, field: status, format: "status:withdraw"}, {label: ยอด, field: amount, format: money}]}
  note: ข้อความคงที่ใต้การ์ด
  link: {label: เปิดรายการถอน, path: "/withdraw"}      # บังคับ · path ในหลังบ้าน
```
format: `text int number money percent ratio_percent datetime date bool status:<table>` (เวลาแสดงเป็นเวลาไทย)

### 5.6 model_context — สิ่งเดียวที่โมเดลเห็นจาก tool (B-4 · AC-14)

```yaml
- {name: has_pending, type: bool, expr: "total > 0"}
- {name: statuses, type: labels, from: rows, pluck: status, format: "status:withdraw"}
- {name: note, type: text, text: แสดงยอดในการ์ดแล้ว}      # ห้ามมีตัวเลข
```
**ห้าม** ตัวเลข/ชื่อคน/เลขบัญชี/ยูสเซอร์ · ชื่อ context ห้ามซ้ำชื่อ value ที่ขึ้นการ์ด

## 6. `contract-tests/<tool>.yaml` (C10)

```yaml
tool: withdraw_pending_by_status
cases:
  - name: มีค้าง 2 สถานะ
    service: K11S
    now: "2026-09-24T14:32:00+07:00"        # ไม่ใส่ = ค่านี้
    input: {}
    responses:
      list: {status: 200, body: {code: 0, message: SUCCESS, data: {...}}}   # ตามที่ host ตอบจริง (catalog)
    expect:
      status: ok                             # ok | not_found | error | denied
      calls: {list: {path: /api/ai/read/.../K11S, query: {date: "2026-09-24"}}}
      fields: {รวมทั้งหมด: "9 รายการ"}
      table_rows: 2
      table_cells: [[รอโอน, "7 รายการ", "142,000.00"]]
      model_context: {has_pending: true}
      link: /withdraw
  - name: host ตอบ 404
    responses: {list: {status: 404, body: {code: 404, message: NOT_FOUND}}}
    expect: {status: not_found}
```
ทุก tool ต้องมี case `ok` + case ที่ไม่ใช่ ok · runner ตรวจเพิ่มเองทุก case: การ์ดที่ไม่ใช่ ok ต้องไม่มี field · ต้องมีลิงก์ · model_context ไม่มีตัวเลข/ค่าบนการ์ด

## 7. built-in (มีให้ทุก kind — ไม่ต้องประกาศ)

| ชื่อ | ใช้ |
|---|---|
| `answer_directly` | ทักทาย · ถามว่าเป็นใคร · นอกขอบเขต · ขอให้แก้ข้อมูล (ปฏิเสธ) · ขอเว็บอื่น (หมวด E) |
| `lookup_menu` | ค้น `menus.yaml` (เมนู + guides) กรองตามสิทธิ์ · ได้การ์ด reference |
| `explain_status` | อธิบาย `statuses.yaml` |
| `host_facts` | ค้น `facts` ใน `host.yaml` |

## 8. เพิ่ม kind ใหม่

1. host ทำ 3 อย่างตาม `contract-host-ai.md` (`/ai/session`, `/ai/scoped-token`, group `/api/ai/read`) + เขียน catalog
2. สร้าง `connectors/<kind>/` ครบ 5 ไฟล์ + contract-tests จาก catalog
3. `go test ./test/ -run RealConnectors` ผ่าน (ต้องเพิ่ม kind ใน `wantKinds` ของ `test/connectors_test.go`)
4. ที่คอนโซล: office → เลือก kind → ออก secret ต่อ service → ใส่ที่ host (`AI_SERVICE_SECRETS`)
