# connector `office-v10x` (โหมด browser) — บันทึก dev · 24/09/2026

> host = `office-api-v10` (`goofficev10`, ตรวจที่ `e7babdd4` branch `feature/w17-ai-session` — ไม่นับไฟล์ `aihost/`, `controller/ai_*`, `middlewares/ai_scoped*`, `route/ai*` ที่ยังไม่ commit) · frontend = `office-v10x` `561548f63` (branch `feature/w17-ai-widget`)
> ทุก call = endpoint อ่านอย่างเดียวที่หน้า v10x เรียกอยู่แล้ว ผ่าน `location.origin + /api` ด้วย token ของแอดมิน · ของเดิมโหมด host อยู่ที่ `connectors/_host-mode/office-v10x/`
> ตรวจ: `go run ./_cmd/connector-check connectors/office-v10x` → โหลดผ่าน · ครอบ 26 ข้อรอบแรก · contract 22 ไฟล์ 83 case ผ่าน

## 1. BQ → tool → endpoint เดิม

| BQ | tool / guide / fact | endpoint (หน้าที่เรียก) |
|---|---|---|
| 01 | `dashboard_summary` (live) | `GET /dashboard-summary-select-day?select_day=A~B&service={service}` (DashboardView — UI ไม่ส่ง service = รวมทุกเว็บ · AI ส่งเพื่อผูกเว็บ) |
| 02 | `deposit_summary` | `GET /GetDepositStatementv2/{service}` ×2 (ทุกสถานะ · `status=1`) + `GET /GetDepositStatementRemainToday/{service}` |
| 03 | `withdraw_pending_by_status` | `GET /GetWithdrawStatementv2/{service}` (limit 300 แล้ว group ตามสถานะ · count จริงจาก server) |
| 04 | `withdraw_pending_oldest` | `GET /GetWithdrawStatementv2/{service}?sort=datetime&order=1&limit=5` |
| 05 | `member_lookup` + fact `no_online_time` | `GET /get-member-list-v3/{service}?search=<ยูส>&username=username` (MemberListView) |
| 06 | `deposit_find` · `withdraw_find` · `withdraw_history_find` · `withdraw_by_id` + fact `no_ip_answer`, `no_deposit_by_id` | `GetDepositStatementv2?search=` · `GetWithdrawStatementv2?username=` (ยังไม่จบ) · `GetWithdrawStatementHistory?search=&username=username` (จบแล้ว) · `GetWithdrawStatementByID?id=` (มี timeline) |
| 07 · 08 | `member_stats` | `GET /get-member-all-count/{service}` + `GET /member-summary/{service}` (`uncomfirm_account`) |
| 09 | `bank_member_pending` | `GET /bank-member/check-approve/{service}` |
| 10 | `bank_accounts_deposit` · `bank_accounts_withdraw` + guide `bank_accounts` | `GET /GetBankConfigDeposit/{service}` · `GET /GetBankConfigWithdraw/{service}` |
| 11 | `promotions_open` | `GET /bonus-list/{service}?sort=order&order=1` (กรอง status ใน values) |
| 12 | fact `profit_report_not_available` + guide `profit_report` + `dashboard_summary` แบบช่วงวัน (ฝาก−ถอน ไม่มี COMPANY) | — (`POST /report-profit` ไม่ใช้ ดู §5) |
| 13 | `menus.yaml` + guide `find_menu_permission` | — |
| 29 | ตาราง `withdraw` `deposit` `bill` (+ `member_*`, `bank_*`, `bonus_status`) | — |
| 35 | `worklog_search` + fact `no_ip_answer` | `GET /worklog-list?service={service}&search=&operator_username=` |
| 41 · 43 | `credit_agent_balance` (snapshot + เวลา) + fact `agent_credit_snapshot`, `no_credit_forecast` + guide `refresh_agent_credit` | `GET /credit-agent-balance/{service}` |
| 42 | `credit_sms_balance` | `GET /credit-sms-balance/{service}` |
| 44 | `bills_unpaid` (สถานะ 3, 5) | `GET /get-bill-payment/{service}` (`scope: office` — ได้บิลทุกเว็บเหมือนหน้าบิล) |
| 45 | fact + guide `topup_system_credit` | — |
| 46 | `bills_pending_confirm` (สถานะ 4) + fact `topup_not_arrived` | เส้นเดียวกับ 44 |
| 47 | `member_credit_live` | `POST /credit-check/{service}` body `{username}` |
| 48 · 49 · 51 | guide `add_credit` · `manage_turn_bonus_point` · `refuse_add_credit` | — |
| 50 | `credit_history` · `credit_wallet_history` + guide | `GET /get-credit-list/{service}` · `GET /get-credit-real-wallet-list/{service}` |
| 52–54 · 56 | fact `assistant_scope` | — |
| 55 | fact `emergency_page` + เมนู `emergency` (อธิบายตามจริง ไม่แนะนำให้กด) | — |

## 2. อ่อนกว่าโหมด host

- **สิทธิ์ใช้ไม่ได้จริงตอนนี้** — ดู §3 ข้อ 1 (ทุก tool ที่ใช้ code จะตอบ "สิทธิ์ไม่ถึง" · ใช้ได้แค่ `credit_agent_balance` / `credit_sms_balance` + built-in)
- ไม่มี BQ-12 แบบ COMPANY/กำไรสุทธิ · ไม่มีรีเฟรชเครดิตเอเจ้นสด (reload = เส้นเขียน)
- BQ-03 นับแยกสถานะจากรายการเก่าสุดไม่เกิน 300 รายการ (ถ้าค้างมากกว่านั้นการ์ดบอก + `counted_partially`) · BQ-06 ฝากค้นตาม id ไม่ได้ · ถอนต้องแยก 2 tool (ยังไม่จบ / จบแล้ว)
- BQ-05 ไม่มี `last_deposit` / วันที่มีรายการ (เส้นที่มีคือ `GetMemberByUsername(V2)` ซึ่งเขียน DB ได้) · ค้นยูสใช้ regex แล้วกรองตรงตัว — ถ้ามียูสขึ้นต้นเหมือนกันเกิน 10 คนอาจไม่เจอ
- `member_stats` ได้เฉพาะ "วันนี้" ตามนาฬิกาของ pod · `credit_wallet_history` กรองยูสได้จาก 50 รายการล่าสุดเท่านั้น
- body ทั้งก้อน (รวม field ลับ) ผ่าน relay ของ backend แม้ไม่ขึ้นการ์ด — โหมด host ตัดที่ต้นทางได้ (§5)
- error ที่หลังบ้านกลืนเป็น code 0 (bank config, worklog, check-approve, SMS) → แยก "ไม่พบ/ศูนย์" ออกจาก "ล้ม" ไม่ได้

## 3. ช่องว่างของ DSL / widget

1. 🔴 **permission ของแอดมินไม่มีในที่ที่ widget อ่านได้** — JWT `result.role.permission` = `null` เสมอ (ล้างก่อนเซ็นทุกจุด: `employee.go` login-verify/line, `config.go` GetPrefixOfficeMainSuperCom, `callbackLogin.go`) · Pinia `authStore.permission` ไม่ persist · ไม่มี endpoint เดียวที่คืนทั้ง `id/username` และ permission (`GET /employees-permission` คืน `data = Role.Permission[]` อย่างเดียว; `GetPrefixOfficeMainSuperCom` ออก JWT ใหม่ = ห้ามใช้) → `identity` ตั้งเป็น `source: jwt` + `permissions: role.permission` ไว้ถูกต้องตามรูป แต่ได้ list ว่าง = rule ที่มี code ไม่ผ่าน (fail closed)
   ต้องการ: `identity.permissions` แบบยิงแยก เช่น `{request: {method: GET, path: /employees-permission}, path: data, pluck: code, where_field: action.is_view, where_value: true}` (อ่านอย่างเดียว · `GetDynamicUser` อ่าน OFFICE employees หรือ SUPERCOM)
2. `where_field` ซ้อน (`action.is_view`) ใช้ได้ใน widget (`getPath` แยกด้วยจุด) แต่ backend ไม่ได้ตรวจรูป
3. envelope `raw` ตรวจ `code == 0` ไม่ได้ (schema ไม่มี const/enum) — พึ่ง required field แทน · response ที่ code ≠ 0 แต่มี field ครบจะผ่าน
4. expression อ้าง `{service}` ไม่ได้ → กรองบิลตามเว็บใน values ไม่ได้ (การ์ดแสดงคอลัมน์ service แทน)
5. รวม array จากหลาย call ไม่ได้ → ถอนยังไม่จบ/จบแล้วต้องเป็น 2 tool · ไม่มีตัวแปร `now` → ตัดโปรที่พ้นวันสิ้นสุด และซ่อนวันที่ปีแรกสุด (`0001-01-01`) ไม่ได้
6. calls รันขนาน ไม่มี `after:` (ไม่กระทบตอนนี้เพราะไม่ใช้ reload)

## 4. ยังยืนยันไม่ได้

- TZ ของ pod: `range_dt`/`select_day` ส่งเป็นเวลาไทย แต่หลังบ้าน parse ด้วย `time.Local` — ถ้า pod เป็น UTC ช่วงวันจะเลื่อน 7 ชั่วโมง (เท่ากับที่หน้าจอเจอ)
- `profit` ของ dashboard อ่าน `report_active_user_day` ที่ service อื่นเขียน (รอบ/เวลาตัดวันไม่รู้)
- ป้าย `member_bank` 1/2 อนุมานจาก `approveBank.go` · สถานะถอน 14/15 ตามตารางเดิม · หน่วยเครดิต Sms · ความหมาย `bonus` ใน get-credit-list = "ยอดเครดิต" ตาม `ConfigCredit.ts`
- ชื่อเมนู sidebar มาจาก SUPERCOM (ไม่อยู่ใน repo) — `menus.yaml` ใช้ชื่อหน้า/แท็บในโค้ด
- dev ที่ API คนละโดเมน (`base-url.ts` URLDev) ต้องตั้ง `data-host-api-base` และหลังบ้านต้องยอม CORS จาก origin ของหน้า — prod ใช้ origin เดียวกัน
- ยังไม่ได้ยิงจริง — response ใน contract-tests สร้างจาก struct/handler (json tag) ของ checkout นี้
- `deposit_summary` ส่ง `status=1` ซึ่งหน้าจอไม่ได้ส่ง (ใช้ equality filter ของ `helper.FilterField` ที่ handler รองรับ)

## 5. endpoint เดิมที่ดูไม่ปลอดภัย (ไม่ได้ใช้ หรือใช้แบบระวัง)

- ไม่ใช้: `GET /GetMemberByUsername(V2)` (goroutine เขียน `member_account.hydra_adv`) · `GET /credit-agent-reload` (upsert OFFICE) · `GET /bill-payment-status-update` (ยืนยันบิล) · `POST /report-profit`, `GET /report-profit-table` (เขียนแคช Redis + ยิง agent ทีละวัน) · `GET /GetPrefixOfficeMainSuperCom` (ออก JWT ใหม่)
- ใช้แต่ควรรู้: `GetBankConfigDeposit/Withdraw` คืน `access_token/refresh_token/device_id/secret_token/citizen_id/birthdate` ให้ OFFICE-X · `worklog-list` คืน `ip` และ `data_old` ไม่ล้าง · `get-member-list-v3` คืนเบอร์ (mask ถ้าไม่มีสิทธิ์) — ทั้งหมดไม่ขึ้นการ์ด แต่ body เก็บใน cache relay ของ backend ≤ 90 วินาที → ถ้าจะเปิดใช้จริงควรให้ relay/widget ตัด field ที่ไม่ได้ประกาศก่อนส่ง
- `get-member-list-v3` มี rate limit ต่อพนักงาน (เกิน = แบน 3 ชั่วโมง ตอบ HTTP 401) — AI ใช้โควตาเดียวกับหน้าจอ
- `GetWithdrawStatementv2` เรียก PAYMENT_API/Redis (read-through cache) ทุกครั้ง · `GetDepositStatementv2` โหลดทุกแถวของช่วงเข้าหน่วยความจำ — อย่าถามช่วงยาวมาก
- `office-api-v10/.env` ถูก track ใน git (ไม่ได้เปิดอ่าน)
