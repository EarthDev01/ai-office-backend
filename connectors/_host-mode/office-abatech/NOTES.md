# connector `office-abatech` — บันทึกสำหรับ dev

> host = `GOTOPOPOFFICE` (module `abaoffice`, branch `feature/w17-ai-session`, package `ai/`) · UI = `office-abatech` (Nuxt 2 SPA)
> แหล่งหลัก: `Docs-workflow/w-17-ai-office/host-read-catalog-abatech.md` + `GOTOPOPOFFICE/ai/read_*.go` (ชื่อ field ตามโค้ดจริง)
> ตรวจ: `go run ./_cmd/connector-check connectors/office-abatech` → โหลดผ่าน · ครอบ 26 ข้อรอบแรก · contract 22 ไฟล์ 80 case ผ่าน

## 1. BQ → tool / guide / fact

| BQ | ตอบด้วย | host route | rule |
|---|---|---|---|
| 01 | `dashboard_today_vs_yesterday` (live) | R1 GET dashboard-summary | P001 |
| 02 | `deposit_summary`, `deposit_find` | R2, R3 | P003 |
| 03 | `withdraw_pending_by_status` | R5 | P004 |
| 04 | `withdraw_oldest_pending` | R6 | P004 |
| 05 | `member_info` | R9 | P006 |
| 06 | `deposit_find`, `deposit_by_id`, `withdraw_find`, `withdraw_by_id` (+ fact `no_ip_in_transactions`) | R3, R4, R7, R8 | P003 / P004 |
| 07 | `member_new_count` | R11 | P006 |
| 08 | fact `no_unverified_member_count` | — | — |
| 09 | `bank_pending_summary` | R12 | P061 |
| 10 | `deposit_bank_accounts` | R13 | P013 |
| 11 | `promotions_active` | R14 | P010 |
| 12 | `profit_loss_range` (POST, `from`/`to`, default ต้นเดือน..วันนี้) | R15 (H4) | P019 |
| 13 | guide `find_menu` + `menu_access_check` | R16 GET menus | logged_in |
| 29 | `statuses.yaml` ตาราง `withdraw` (16) · `deposit` · `bank_statement` | — | — |
| 35 | `staff_working_log`, `staff_login_log` | R17, R18 | M005 |
| 41 | `system_credit_balance` (live, `reload=1`) | R19 | logged_in |
| 42 | `sms_credit_balance` (live) | R20 | logged_in |
| 43 | fact `no_credit_forecast` | — | — |
| 44 | `bills_outstanding` (live) | R21 | min_level 7 |
| 45 | fact + guide `topup_system_credit` | — | — |
| 46 | fact `topup_not_arrived` | — | — |
| 47 | `member_credit` (live) | R10 | P006 |
| 48, 51 | guide `add_credit` (เมนู ระบบจัดการเครดิตสมาชิก › แท็บ แจกโบนัสฟรี · ฟอร์มจริง `components/Credit/FreeBonus.vue`) | — | — |
| 49 | guide `credit_tabs` (แท็บจริง 11 แท็บ `pages/Credit/index.vue:96-160`) | — | — |
| 50 | `member_credit_history` | R22 | P008 |
| 53 · 55 | fact `about_assistant` · fact `emergency_buttons` | — | — |

## 2. การตัดสินใจ

- **page_auth**: token = `localStorage.headertoken` แบบ raw (`pages/login.vue` เก็บ `res.data.header` · `store/index.js` ส่ง `Authorization: Bearer <headertoken>`) · service = query `service` base64 (ทุกหน้า `atob(this.$route.query.service)`)
- **host_api_base `{origin}/api`**: prod `axios.defaults.baseURL = "/api"` (`plugins/config.js` `API_URL_PRODUCTUION`) = origin เดียวกับหน้า · **dev** ใช้ `config.API_URL` คนละ origin → snippet บน dev ต้องตั้ง `data-host-api-base`
- **ลิงก์การ์ด**: `/<route>?service={service_b64}&web={service}` ตามที่ sidebar สร้าง (`SideBar.vue:54-98`) · `web` = service เพราะ `TopNavbar.vue:395` หา `serviceObj[web]` ด้วยชื่อ service · หน้าระดับ office (`/Logs/Working`, `/Employee`, `/BillPayment`) ไม่ใส่ service
- **freshness**: live = R1 (B-7 ระบุ BQ-01 เป็นยอดเงิน ห้ามแคช), R10, R19, R20, R21 · ที่เหลือ summary แคช 15–60 วิ
- **R19 ใส่ `reload=1` ทุกครั้ง** (B-7 ยอดสด) → host upsert snapshot (D-89) · ถูกจำกัด ≤ 1 ครั้ง/15 วิ/เว็บ — เกินแล้ว host คืน snapshot + `reload.ok=false` การ์ดแสดง "รีเฟรชยอดล่าสุดสำเร็จ: ไม่ใช่"
- **BQ-13**: R16 ส่งแค่ code (ไม่มีชื่อเมนู) · ชื่อ/route/แท็บอยู่ใน `menus.yaml` (จาก `plugins/menulist.js` 52 + `menumain.js` 10 + หน้าบิล) · ใส่ "รหัสสิทธิ์ Pxxx" ใน `description` ของเมนูให้โมเดลส่งต่อเข้า `menu_access_check` (ตรวจว่าเปิดในระบบนี้ + ผู้ใช้มีสิทธิ์ + สิทธิ์ดูของกลุ่มบริการเสริม)
- **permissions.yaml**: rule ของ tool ตรง `readRouteTable()` ทุกเส้น · rule `menu_*`/`tab_*` ใช้กรอง `lookup_menu` เท่านั้น (tab ของ /Credit = code 8001–8009)
- **สถานะถอน**: label ตามตารางที่แอดมินใช้ (`WithdrawStatementV2.vue`) · ชื่อจากจออื่นเป็น aliases (KI-3: สถานะ 10 = "ไม่ได้รับ OTP" vs "OTP ไม่ถูกต้อง", 2 = "รอโอน" vs "รอคิว") · next_action จากปุ่มจริงในตาราง/modal (appendix-abatech-auth §5.1)
- **สถานะฝาก**: 1/2/5/6 ตาม UI + 0 = "ไม่สำเร็จ" (UI fallback · worker: agent ล้ม) · ค่าอื่น → "ยังไม่มีคำอธิบาย" (UI จริงแสดง "ไม่สำเร็จ" กับทุกค่าที่ไม่รู้จัก)
- **BQ-55**: brief บอกว่า abatech ไม่มีปุ่มฉุกเฉิน แต่โค้ดจริงมี — `components/Dashboard/Layout/TopNavbar.vue:53-78` ปุ่ม **SOS** (ยืนยัน "ต้องการ ปิด SERVER ใช่หรือไม่" → เขียน Firebase `topup-group/<GROUP>.status=0` → `layouts/menu.vue` ทุกคนในกลุ่มถูกพาออกจากหลังบ้าน) และปุ่ม **Disabled** (ปิดบัญชีพนักงานตัวเองผ่าน `/SetActiveUser`) → fact ตอบตามโค้ด
- **ตัวเลขใน model_context**: ใช้แค่ bool / labels จากตารางสถานะ (กรองเฉพาะ code ที่อยู่ในตาราง) / ข้อความคงที่

## 3. DSL gaps (ทำใกล้เคียงที่สุดแล้ว)

1. ~~**`menus.yaml` path ใส่ placeholder ไม่ได้**~~ (แก้แล้ว 24/09: `lookup_menu` render `{service}`/`{service_b64}` ใน path แล้ว · menus.yaml ใส่ `?service={service_b64}&web={service}` ให้หน้าต่อเว็บแล้ว) — `lookup_menu` ใช้ `Menu.Path` ตรง ๆ แต่หน้าต่อเว็บของ abatech ต้องมี `?service=<base64>` → การ์ดอ้างอิงของ lookup_menu เปิดหน้าโดยไม่มี service (หน้าโหลดข้อมูลไม่ได้) · ต้องการ: render `{service_b64}`/`{service}` ใน path ของเมนู
2. **labels จากค่าเดี่ยวไม่ได้** (`model_context type: labels` ต้องเป็น array) → `deposit_by_id`/`withdraw_by_id` ใช้ bool (`is_success`, `is_stuck_credit` …) แทนป้ายสถานะ
3. **ป้ายของสถานะที่ไม่รู้จักมีตัวเลข** (`สถานะ 0 (ยังไม่มีคำอธิบาย)`) → contract runner ถือว่า model_context รั่วตัวเลข · แก้ด้วย value กรองเฉพาะ code ที่รู้จัก + bool `has_unknown_status` · ต้องการ: ป้าย unknown ที่ไม่มีเลขสำหรับโมเดล
4. **แปลงค่าแสดงผลแบบมีเงื่อนไขไม่ได้** — `operator_name == ""` ควรแสดง "auto" เหมือน UI → ใช้ note "ผู้ทำรายการว่าง = ระบบอัตโนมัติ"
5. **ตรวจ input ข้าม field ไม่ได้** — `from`/`to` ≤ 62 วัน, `start_date`/`end_date` ต้องมาคู่ → บอกใน description · ผิดแล้ว host ตอบ 400 = การ์ด "ดึงไม่ได้"
6. **ไม่มี "error when expr"** — SMS `configured:true` แต่ `balance:null` (provider ตอบไม่ใช่ 200) ควรเป็นการ์ดดึงไม่ได้ → แสดง "—" + bool `balance_known`
7. **การ์ดมีตารางได้ตารางเดียว** — `deposit_summary` แสดงตารางบัญชีฝาก ส่วนรายการตกค้างแยกตามสาเหตุ (comment) ขึ้นได้แค่ยอดรวม + ป้ายสถานะ
8. **join code → ชื่อเมนูจาก response ไม่ได้** — จึงให้โมเดลส่ง code (ข้อ 2 ของ §2)

## 4. สมมติฐานที่ยังยืนยันไม่ได้

- `agent-balance.updated_at` เป็น unix **วินาที** (catalog R19 "อนุมาน") — การ์ดแสดงเป็นวันเวลา
- หน่วย SMS balance = เครดิตของ provider ไม่ใช่จำนวนข้อความ (อนุมานใน catalog)
- ความหมาย `bank_statement.status` 0/1/2/3/99 มาจาก `Docs/go_deposit_mootui.md` (worker สาย mootui เขียน collection เดียวกัน) · ป้าย "ตกค้าง"/"ตกค้างโปรโมชั่น" เป็นชื่อของ connector เอง (UI แท็บรายการตกค้างแสดงแค่ comment ไม่มีป้ายสถานะ)
- `member_account.active` ค่าอื่นนอกจาก 0/1/99 (เช่น 4 = ลบ?) ยังยืนยันไม่ได้ → แสดงเป็น "ยังไม่มีคำอธิบาย"
- `bills` `end_date`/`datetime` ไม่ถูกแปลง TZ โดย host (รูปแบบ upstream ไม่มี struct) · `GetBillPaymentByService` คืนทุกสถานะหรือเฉพาะค้าง (OQ-6) → tool กรอง 3/5 เอง
- โปร `withdraw_fix` ใน `bonus_config` เป็นยอดหรือจำนวนเท่า — แสดงเป็นตัวเลขเฉย ๆ
- เลขตรงหน้าจอเดิมเมื่อ pod `TZ=Asia/Bangkok` เท่านั้น (OQ-1)
- BQ-45/46 "ติดต่อ support" มาจาก spec ภาคผนวก ก (ไม่มีโค้ดเติมเครดิตระบบให้ตรวจ)
- ingress ส่ง header `X-AI-Scoped-Token` ถึง pod (OQ-3)

## 5. ข้อสังเกตในโค้ด host/UI ที่กระทบการใช้กับ AI

- R10 `member-credit` เขียน `working_log` ทุกครั้ง (D-89) และ R19 `reload=1` upsert snapshot — โมเดลเรียกซ้ำจะสร้าง log/เรียก gateway เพิ่ม (มี rate limit ฝั่ง host)
- R19 ใช้ `logged_in` → ผู้ใช้ทุกคนที่ล็อกอินสั่ง reload เครดิตระบบได้ (ตรงกับแถบบนของ UI แต่เป็นการเขียน)
- นิยามโบนัส R1 (ตัด bonus_id 105/106) ≠ R15 (ตัด 105/106/111) — ตัวเลขโบนัสสองการ์ดอาจต่างกัน
- `TopNavbar.vue:684-685` hardcode email/password ของ Firebase สำหรับปุ่ม SOS ในโค้ด frontend (ไม่เกี่ยวกับ AI แต่ควรแจ้งทีม)
