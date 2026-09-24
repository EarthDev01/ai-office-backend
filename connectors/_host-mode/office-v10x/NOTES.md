# connector `office-v10x` — บันทึก dev (24/09/2026)

> host = `office-api-v10` (module `goofficev10`, branch `feature/w17-ai-session` · working changes) · frontend = `office-v10x`
> แหล่งอ้างอิง: `Docs-workflow/w-17-ai-office/host-read-catalog-v10x.md` (R1–R23) + โค้ดจริง `route/ai.go`, `controller/ai_read.go`, `aihost/*.go`, handler legacy (`controller/Agent.go`, `bonus.go`, `credit.go`)
> ตรวจ: `go run ./_cmd/connector-check connectors/office-v10x` → โหลดผ่าน · ครอบ 26 ข้อรอบแรก · contract 23 ไฟล์ 74 case ผ่าน

## 1. BQ → tool / guide / fact / ตารางสถานะ

| BQ | ตอบด้วย | host route |
|---|---|---|
| 01 | `dashboard_summary` (live) | R1 `GET dashboard` |
| 02 | `deposit_summary` + `explain_status:deposit` | R2 |
| 03 | `withdraw_pending_by_status` | R5 |
| 04 | `withdraw_pending_oldest` | R6 |
| 05 | `member_lookup` + fact `no_online_time` | R9 |
| 06 | ฝาก `deposit_find` / `deposit_by_id` · ถอน `withdraw_find` / `withdraw_by_id` · fact `no_ip_answer` | R3 R4 R7 R8 |
| 07 · 08 | `member_stats` (`new_registered`, `unverified`, `by_active`) | R10 |
| 09 | `bank_member_pending` | R11 |
| 10 | `bank_accounts_deposit` · `bank_accounts_withdraw` · guide `bank_accounts` | R12 R13 |
| 11 | `promotions_open` | R14 (legacy) |
| 12 | fact `profit_report_pending` + guide `profit_report` — **ไม่มี tool** (R15 ปฏิเสธเสมอ) ⏳ รอ code เมนู `/report/profit` | — |
| 13 | `menus.yaml` (18 เมนู) + guide `find_menu_permission` / `profit_report` | — |
| 29 | ตาราง `withdraw` · `deposit` · `bill` (+ `member_active`, `member_bank`) | — |
| 35 | `worklog_search` + fact `no_ip_answer` | R16 |
| 41 | `credit_agent_balance` (snapshot) · `credit_agent_balance_refresh` (R18+R17) | R17 R18 |
| 42 | `credit_sms_balance` | R19 |
| 43 | `credit_agent_balance` (แสดงเวลาอัปเดต) + fact `no_credit_forecast` | R17 |
| 44 | `bills_unpaid` (status `3,5`) | R20 |
| 45 | fact `topup_system_credit` + guide `topup_system_credit` | — |
| 46 | `bills_pending_confirm` (status `4`) + fact `topup_not_arrived` | R20 |
| 47 | `member_credit_live` (POST) + guide `check_member_credit` | R21 |
| 48 | guide `add_credit` (จัดการเครดิต › แท็บ จัดการเครดิต › แท็บย่อย เพิ่มเครดิต · ฟิลด์ตาม `components/credit2/Addcredit.vue`) | — |
| 49 | guide `manage_turn_bonus_point` (แท็บตาม `assets/webconfig/menu_credit2.ts` + เมนูหน้าข้อมูลสมาชิก `menu_credit.ts`) | — |
| 50 | `credit_history` · `credit_wallet_history` + guide `credit_history` | R22 R23 |
| 51 | guide `refuse_add_credit` + `add_credit` | — |
| 52–54 · 56 | fact `assistant_scope` | — |
| 55 | fact `emergency_page` + เมนู `emergency` (อธิบายตามจริง ไม่แนะนำให้กด) | — |

## 2. การตัดสินใจ

- **permissions.yaml mirror `AIRequire` ตรงตัว**: `dashboard` = `all_of [DASHBOARDALL] + min_level 6` · `bank_deposit/withdraw` = `all_of [BANK_MANAGE, BANK_MANAGE_*]` · `credit_check` = `all_of [TRANSECTION_MANAGECREDIT] + any_of [_CHECK, _CHECKPOCKET]` · R17–R19 = `logged_in` · rule อื่นที่ไม่มี tool (`bank_manage`, `member_block`, `employee`, `role`, `deposit_insert`, `credit_add`, `logged_in`) ใช้กรองเมนู/แท็บใน `lookup_menu` เท่านั้น
- **ช่วงวัน**: ทุก tool ที่มีวันส่ง `from` + `to` เสมอ (`from = input.date_from|today`, `to = input.date_to|input.date_from|today`) — ไม่ใช้ `date` เพื่อเลี่ยง host 400 เมื่อส่งคู่กัน · host ตีความวันล้วนเป็นวันไทยและ `to` รวมทั้งวัน · มีเคสข้ามเที่ยงคืนทุก tool ที่มีวัน
- **envelope raw** ใช้กับ wrapper ที่ข้อมูลสำคัญอยู่นอก `data` (`count`, `summary`): deposit-list, withdraw-*, bank-config-*, worklog, bills, get-credit-list(-real-wallet) · wrapper ตอบ HTTP 200 = `code 0` เสมอ · legacy R22/R23 ที่ตอบ `code 1` จะมี `data:null` → schema ผิด = "ดึงไม่ได้" (ไม่ใช่ศูนย์)
- **ไม่พบ**: wrapper ค้นตาม id/username → `not_found_codes: [404]` · list ว่าง → `not_found.when: empty(rows)` · R11 count ศูนย์ / R2 ไม่มีฝากและตกค้าง / R12-13 ไม่มีบัญชี → not_found พร้อมข้อความ "ไม่มี…" · legacy R17 `code 1` (ไม่มีเอกสาร) และ R21 `code 1` → not_found (ใช้ `not_found.when: "false"` เพื่อกำหนดข้อความ)
- **freshness live**: R1 (B-7), R17/R18, R19, R20, R21 · อื่นเป็น summary `cache_ttl` ≤ 60 (member_lookup 30, worklog 30, ค้นรายการ 0)
- **R18 reload** แยกเป็น tool `credit_agent_balance_refresh` (reload = optional, envelope raw เพราะตอบ `{"code":0,"msg":"Success"}` ไม่มี `data`) — ไม่ยิง reload ทุกครั้งที่ถามยอด
- **ลิงก์การ์ด**: path ของ router v10x (`/dashboard`, `/transection/deposit`, `/transection/withdraw`, `/member/{username}`, `/member/memberlist`, `/member/approvebank`, `/statement/managebank`, `/setting/bonus`, `/worklog`, `/bill`, `/transection/managecredit?menu=credit`) · v10x เลือก service จาก `localStorage["web-service"]` ไม่ใช่ query จึงไม่ใส่ `{service}` ในลิงก์ · เครดิตเอเจ้น/SMS อยู่ที่ navbar ไม่มีหน้าเฉพาะ → ลิงก์ `/`
- **page_auth**: `auth_token` = JSON `{value, expiration}` (expiration = JWT `exp` หน่วยวินาที · `helper-function.ts` `getItemWithExpireV2` เทียบวินาที · widget รองรับทั้งวินาที/มิลลิวินาที) · `web-service` = ชื่อ service ตรงตัว · prod base = `location.origin + "/api"` (`base-url.ts`)
- **ป้ายสถานะ**: ถอน = `helper.withdrawStatus` (+ alias จาก appendix §6.4) · ฝาก = slot status ของ `DepositTransection.vue` (UI แสดง "ไม่สำเร็จ" ให้ทุกค่าที่ไม่ใช่ 1/2/5/6 — ตารางใส่เฉพาะ 0 ที่ยืนยันได้ ค่าอื่นขึ้น "ยังไม่มีคำอธิบาย") · บิล = `StatusBill.vue` · โปรฯ คอลัมน์ "เทิร์น(เท่า)" = `fix_multiple`, "อั้นถอน" = `fix_withdraw` (ตาม `BonusList.vue` ไม่ใช่ `withdraw_fix` ตามที่ catalog เขียน)

## 3. DSL gaps

1. **ไม่มี `now` ใน expression / เทียบสตริงวันที่ไม่ได้** → `promotions_open` ตัดโปรฯ ที่ `end_date` ผ่านแล้วไม่ได้ (catalog R14 ระบุว่า tool ต้องตัดเอง) · ตอนนี้แสดงคอลัมน์สิ้นสุดกิจกรรม + note ให้ดูเอง · ต้องการ: ตัวแปร `now` (RFC3339) + การเทียบเวลา หรือฟังก์ชัน `before(a,b)`
2. **calls รันขนานเสมอ (`toolrunner.go`)** → `credit_agent_balance_refresh` อ่าน R17 พร้อมกับ R18 ยอดอาจเป็นค่าก่อน reload · ต้องการ: `after: <call id>` เพื่อรันต่อกัน
3. **ไม่มี call แบบมีเงื่อนไข / ค่าคงที่ใน template** → ใช้ tool แยก (`bills_unpaid` / `bills_pending_confirm`, `credit_agent_balance` / `_refresh`) แทน input enum
4. **not_found ตาม body message ไม่ได้** → R21 `code 1` แปลได้ 3 อย่าง (ไม่พบยูส · ไม่มีสิทธิ์ `_CHECK` เมื่อมีแค่ `_CHECKPOCKET` · ระบบเกมไม่ตอบ) → ถือเป็น "ไม่พบ" และข้อความบอกทั้งสองกรณีตามจริง · ต้องการ: `not_found_messages` หรือให้ host ทำ wrapper
5. **label ของสถานะที่ไม่รู้จักมีตัวเลข** (`สถานะ N (ยังไม่มีคำอธิบาย)`) → หลุดเข้า `model_context` ชนิด `labels` · contract runner จับได้ (ตัด case นั้นออกแล้ว) แต่ตอนรันจริง engine ไม่กรอง · ควรแก้ที่ `Formatter.StatusLabel`/labels ให้ใช้ป้ายไม่มีเลข (โค้ดกลาง — ไม่ได้แก้)
6. **สร้าง array จากค่าเดี่ยวไม่ได้** → การ์ดรายละเอียดฝาก/ถอนส่งสถานะให้โมเดลเป็น bool (`is_success`, `needs_bank_check` …) แทน labels
7. ตัวเลขใน array ของ string (`tags`, `active_dates_7d` ของ R9) แสดงเป็น field ไม่ได้สวย → ไม่แสดง

## 4. ยังยืนยันไม่ได้

- **ชื่อเมนูใน sidebar** มาจาก SUPERCOM (`GetPrefixOfficeMainSuperCom` → `menu_list.name_th`) ไม่อยู่ใน repo · `menus.yaml` ใช้ชื่อหน้า/แท็บที่เจอในโค้ด (`menu_navbar.ts`, `menu_credit2.ts`, `menu_bank.ts`, หัวข้อหน้า) · ชื่อที่ **ตั้งจากชื่อ route/ไฟล์** (ไม่เจอหัวข้อในหน้า): "รายการฝาก", "รายการถอน", "รายการสมาชิก", "อนุมัติบัญชีสมาชิก", "สมาชิก Block", "โบนัส/โปรโมชั่น", "สรุปกำไรขาดทุน", "รายงานโบนัส", "รายงานสมาชิกออนไลน์" → ควร dump `/MenuSuperCom/:service` บน dev มาเทียบ
- `member_bank` ป้าย 1 "อนุมัติแล้ว" / 2 "ไม่อนุมัติ" อนุมานจาก `approveBank.go` (`SUCCESS`/`UNSUCCESS`) — UI ใช้ `verify_status` ผ่าน `utilities/bankStatus.ts`
- ความหมายของสถานะถอน 14/15 และ next_action ของ 3 อนุมานตาม appendix §6.1
- หน่วยของเครดิต Sms (`GetBalanceSMS` คืนตัวเลขเปล่า) · `CheckCredit.currentCredit` = "เครดิตคงเหลือ" ตาม `components/credit/CheckCredit.vue` (อนุมานว่า UI แสดงค่านี้)
- rank / user_type ของสมาชิก — ไม่มีตารางความหมาย แสดงเลขดิบ
- ยังไม่เคยยิงกับ host จริง — response ใน contract-tests สร้างจาก catalog + struct ใน `ai_read.go` / `model/*.go` (json tag)

## 5. สิ่งที่เห็นในโค้ด host ที่ควรรู้ (ไม่ได้แก้)

- R19 `GetBalanceSMS`: ผู้ให้บริการตอบ `Code != 200` → คืน `code 0, data 0` (ศูนย์ปลอม ไม่ใช่ error) — การ์ดใส่ note เตือนไว้
- R18 `ReloadBalanceAgent` ตอบ `Success` เสมอแม้ดึงไม่ได้ · agent ล่ม = panic → 500
- R21 `code 1` ปนหลายความหมาย (§3 ข้อ 4) · rule ของ route ยอม `_CHECKPOCKET` อย่างเดียวได้ แต่ handler (ไม่ส่ง `asset_code`) ต้องการ `_CHECK` → ผู้ใช้ที่มีแค่ `_CHECKPOCKET` จะได้ "ไม่พบ" แทน "สิทธิ์ไม่ถึง"
- R14 `GetBonusList` DB error เขียน JSON สองก้อน (→ `bad_json`) · `end_date` ที่ไม่ได้ตั้งออกมาเป็น `0001-01-01T00:00:00Z` (ไม่ใช่ null) การ์ดจะแสดงปีแรกสุด
- R17 `updated_at = 0` จะแสดงเป็นปี 1970 (ไม่ได้ทำเป็น null)
- R16 `include_login` คืน log เข้าระบบระดับ office ทั้งชุด (catalog §10 ข้อ 3 ยังค้าง)
- R1 ยอด "วันนี้" ขึ้นกับรอบของตัวเขียน `report_active_user_day` (ไม่อยู่ใน office-api-v10) — การ์ดมี note
- R15 ปิดอยู่จนกว่าจะใส่ code เมนู `/report/profit` แทน `PermReportProfitTBD` — เมื่อเปิดแล้วค่อยเพิ่ม tool (POST body `{from,to}`, timeout ≥ 60 วิ, ระวัง Redis cache side effect)
