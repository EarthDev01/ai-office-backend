# connector `office-abatech` (โหมด browser) — บันทึกสำหรับ dev

> host = `GOTOPOPOFFICE` (module `abaoffice`) · UI = `office-abatech` (Nuxt 2 SPA) · ตัดสิน 24/09: **ไม่เพิ่มโค้ดฝั่งหลังบ้าน**
> widget ในหน้าแอดมินยิง API เดิมด้วย token ของแอดมิน → ส่ง JSON ให้ backend ตัดค่าตาม `values`
> ตรวจ: `go run ./_cmd/connector-check connectors/office-abatech` → โหลดผ่าน · tool 22 · ครอบ 26 ข้อรอบแรก · contract 22 ไฟล์ 91 case ผ่าน
> รุ่นโหมด host เดิมอยู่ที่ `connectors/_host-mode/office-abatech/` (statuses / menus / guides / facts นำมาใช้ต่อ)

## 1. BQ → endpoint เดิม

| BQ | tool / guide / fact | endpoint เดิม (prefix `/api`) | rule |
|---|---|---|---|
| 01 | `dashboard_today_vs_yesterday` (live) | POST `/GetDashboardSummary/:service` ×2 (วันนี้ / เมื่อวาน) | P001 |
| 02 | `deposit_bank_summary` · `deposit_remain_summary` · `deposit_status_summary` | GET `/DepositBankSummaryV2/:service` · `/DepositBankRemain/:service` · `/GetDepositStatementListByFilter/:service` ×5 (limit 1 อ่าน count) | P003 + 3001 / 3002 / 3004 |
| 03 | `withdraw_pending_by_status` | GET `/GetWithdrawStatementListByFilter/:service` ×5 (status 2, 3, 99, 4, 11 · limit 1) + `/GetWithdrawP2PRemainCount/:service` | P004 + 4003 |
| 04 | `withdraw_oldest_pending` | `/GetWithdrawStatementListByFilter` sort=datetime order=1 limit 300 | P004 + 4003 |
| 05 | `member_info` | GET `/GetMemberByUsername/:service?username=` | P006 |
| 06 | `deposit_find` · `withdraw_find` · `withdraw_by_id` · fact `no_ip_in_transactions` | list ฝาก/ถอน (search=ยูส) · GET `/WithdrawByID/:service?id=` | P003+3004 / P004+4003 |
| 07 | `member_new_count` | GET `/GetMemberCount/:service` (วันนี้เท่านั้น) | P006 |
| 08 | fact `no_unverified_member_count` | — | — |
| 09 | `bank_pending_summary` | GET `/GetMemberBankPendingSummary/:service` ×2 (ทั้งหมด / PENDING) | P061 |
| 10 | `deposit_bank_accounts` | GET `/DepositBankSummaryV2/:service?deposit_type=all` | P013 |
| 11 | `promotions_active` | GET `/Bonuslist/:service` | P010 |
| 12 | `profit_loss_range` | POST `/GetDashboardSummary/:service` ช่วงวัน (ค่าเริ่มต้นต้นเดือน..วันนี้) | P019 |
| 13 | guide `find_menu` + `lookup_menu` + `menu_access_check` | GET `/GetConfigByKey?key=submenu` / `mainmenu` (scope office) | logged_in |
| 29 | ตาราง `withdraw` · `deposit` · `bank_statement` | — | — |
| 35 | `staff_working_log` + fact `login_history` | GET `/GetWorkingLog?date=&service=` | M005 |
| 41 | `system_credit_balance` (live) + fact `system_credit_refresh` | GET `/GetBalanceAgent/:service` | logged_in |
| 42 | `sms_credit_balance` (live) | GET `/CheckCreditBalanceEnter/:service` | logged_in |
| 43 / 45 / 46 | fact `no_credit_forecast` · `topup_system_credit` (+guide) · `topup_not_arrived` | — | — |
| 44 | `bills_outstanding` (live) | GET `/GetBillPayment` (scope office) | min_level 7 |
| 47 | `member_credit` (live) | POST `/CheckCredit/:service` ✍️ working_log (D-89 ยอมรับ) | P006 |
| 48 / 51 · 49 | guide `add_credit` · `credit_tabs` | — | — |
| 50 | `member_credit_history` | `/GetWorkingLog` กรองยูส | P008 + M005 |

## 2. การตัดสินใจหลัก

- **token = `localStorage.token` ไม่ใช่ `headertoken`**: หน้าเว็บเก็บ 2 ตัว (`login.vue:256-257`, `menu.vue` ต่ออายุทุกครั้งที่เปลี่ยนหน้า) — `headertoken` (ที่หน้าเว็บส่งเป็น Bearer) ถูกล้าง `role.permission = nil` ก่อน sign (`employee/route.go:162,229`) · UI เช็คสิทธิ์จาก `jwt.decode(token).result.role.permission` · DSL อ่าน identity ได้จาก token ตัวเดียวกับ Authorization เท่านั้น จึงต้องใช้ `token` ทั้งคู่ · backend ยอมรับเพราะลายเซ็น/claims รูปเดียวกันและสิทธิ์จริงอ่านจาก DB · ผลพลอยได้: `GetMemberByUsername` ปิดเบอร์ได้จริงเมื่อผู้ใช้มีสิทธิ์ 6005 (เดิมไม่เคยทำงาน)
- **identity**: `result.id` / `result.name` (employee.User ไม่มี field `username`) / `full_name` / `level` / `deptcode` / `role.permission[].code` ที่ `Isactive == 1`
- **ไม่มี extra_headers**: API เดิมใช้แค่ `Authorization: Bearer …` (ไม่มี header ชื่อ `headertoken`)
- **envelope**: E1 `{code, msg, payload}` เป็นค่าหลัก · เส้นที่ `count` อยู่นอก payload / code 200 / body ของ upstream ใช้ `envelope: raw` + schema ที่ `required: [count]` (error ของ host ไม่มี `count` → schema_mismatch = การ์ดดึงไม่ได้)
- **ไม่พบ**: `GetMemberByUsername`, `WithdrawByID`, `CheckCredit` ตอบ code 1 → `not_found_codes: [1]` (CheckCredit code 1 = ไม่พบยูส **หรือ** ระบบเกมล้ม — ข้อความ not_found บอกทั้งสองกรณี)
- **status ฝากฐานหก**: ส่ง `"10"` = สถานะหก · list ส่ง `status=0` เสมอแล้วกรองใน connector (ว่าง = host กรองสถานะศูนย์)
- **สิทธิ์**: rule ของ tool = code เมนู + code แท็บที่ UI ใช้เปิดข้อมูลชุดนั้น (`all_of`) · ถอน/ฝากหลายเส้นไม่มี auth ที่ host → สิทธิ์บังคับที่ connector เท่านั้น
- **ข้อมูลอ่อนไหว**: ไม่ประกาศ password / otp / token / เบอร์ / ชื่อ / เลขบัญชี ใน `values` · tool ที่ก้อนดิบมีความลับ (`member_info`) `cache_ttl: 0`
- **BQ-10 ไม่ใช้ `/GetBankAll`**: ก้อนมี `citizen_id`, `refresh_token`, `token_proxy_bank`, `api_auth` (และ user/pass ธนาคารสำหรับ level สูง) — ใช้สรุปบัญชีฝาก `deposit_type=all` แทน

## 3. อ่อนกว่าโหมด host

- BQ-01/12 นิยามยอดฝาก = `deposit_statement` เติมเครดิตสำเร็จ ≠ การ์ดรายงานภาพรวม / หน้า `/report` (นับเงินเข้าบัญชี `bank_statement`) — เส้นที่ตรงหน้าจอเขียนแคช Redis (`ByDayQuery`) หรือเขียน `report_alltime` (`GetReportProfitLossQuery`, D-89 ห้าม) · วันไม่มีฝาก host คืน `payload:null` → ยอดถอนเป็น "—"
- BQ-03 ไม่มียอดเงินต่อสถานะ (อ่านแค่ count) · สถานะ 5–10, 12–16 รวมเป็นก้อนเดียว · ดูได้ทีละวัน
- BQ-04 ทีละวัน + ดูสามร้อยรายการแรกของวัน (แจ้งบนการ์ดเมื่อดูไม่ครบ)
- BQ-06 ฝั่งฝากไม่มีค้นด้วย id · ค้นยูสผ่าน regex ของ host แล้วกรองตรงตัว (สองร้อยแถว)
- BQ-07 วันนี้เท่านั้น · BQ-11 ตัดโปรพ้นวันไม่ได้ · BQ-35 ไม่มีประวัติล็อกอิน (ต้องรู้ id พนักงาน) · BQ-35/50 ทีละวัน
- BQ-41 snapshot (ไม่ reload — call วิ่งขนาน เรียง reload→get ไม่ได้ และไม่มี rate limit ฝั่ง host) · BQ-47 ทุกครั้งเขียน working_log ในชื่อผู้ใช้ และไม่มี rate limit
- JSON ดิบทั้งก้อน (รวมรหัสผ่านสมาชิก/เบอร์/เลขบัญชี) วิ่งผ่าน widget → backend ก่อนถูกตัด — โหมด host ตัดที่ต้นทาง

## 4. DSL gaps

1. **identity อ่าน token คนละตัวกับ Authorization ไม่ได้** → ต้องส่ง `token` ตัวใหญ่ (มี permission ~เจ็ดกิโลไบต์ขึ้นไป) เป็น Bearer · ถ้าเพิ่ม `identity.token: {source, key}` จะกลับไปส่ง `headertoken` ได้ตรงหน้าเว็บ
2. ไม่มีค่า "ตอนนี้" ใน expr → กรองโปรตามวันสิ้นสุด / คิดอายุค้างเป็นนาทีไม่ได้
3. template ไม่มีค่าคงที่เป็นทางเลือก (`{input.x|"0"}`) และไม่มี `{date:input.x-1}` → BQ-01 เทียบได้แค่วันนี้/เมื่อวาน · ต้องส่ง status คงที่แล้วกรองเอง
4. `envelope` ต่อ call ได้แค่ ""/raw — ระบุ code_field/ok_codes ต่อ call ไม่ได้ (code 200 ของ ResponseV2, `status.code` ของระบบบิล) · ไม่มี "error when expr"
5. call วิ่งขนาน ระบุลำดับ/เงื่อนไขไม่ได้ (reload → get) · รวม array จากหลาย call ไม่ได้ (หลายสถานะ → ตารางเดียว)
6. labels จากค่าเดี่ยวไม่ได้ · การ์ดตารางเดียว · แสดง "auto" แทนผู้ทำรายการว่างไม่ได้ (เขียนใน note)
7. ตรวจ input ข้าม field ไม่ได้ (ช่วงวันยาวเกิน)

## 5. สมมติฐานที่ยังไม่ยืนยัน

- **ขนาด header**: `token` (มี `role.permission` ครบ) ยาวกว่า `headertoken` มาก · seed `t_role_setting.json` สิทธิ์หกสิบเอ็ดรายการ ≈ 7.5 KB JSON → Bearer ≈ 11 KB+ · ถ้า ingress ใช้ค่า default ของ nginx (`large_client_header_buffers 4 8k`) จะได้ 400 ทุก call → **ต้องทดสอบบน dev ก่อน** ถ้าไม่ผ่านต้องแก้ DSL ข้อ 4.1
- `time.Local` ของ pod = Asia/Bangkok (ไม่มีหลักฐานใน repo) — ทุกเส้นที่รับ `date` ตัดวันตามนั้น
- `payload:null` ของ `GetDashboardSummary` = ไม่มีรายการฝากจริง (aggregate ฝากล้ม host panic 500 แทน)
- รูป payload ของระบบเกม (`currentCredit` / `data.{WALLET,ABATECH,UFABET}`) และระบบบิล (`status.code`, `data[]`) อนุมานจากฟิลด์ที่ frontend ใช้
- `balance_credit_agent.updated_at` เป็น unix วินาที · หน่วย SMS = เครดิตผู้ให้บริการ · `bank_statement.status` จาก `Docs/go_deposit_mootui.md`
- เปลี่ยน ISO week แล้ว token เดิมใช้ไม่ได้ (key = sha1(Host + week)) → การ์ด "ดึงไม่ได้" จนหน้าเว็บต่ออายุ token

## 6. endpoint ที่พบว่าไม่ปลอดภัย (แจ้งทีมหลังบ้าน)

- ไม่มี auth: `/GetDepositStatementListByFilter`, `/GetWithdrawStatementListByFilter` (มีเบอร์โทร + เลขบัญชี), `/GetWithdrawP2PRemainCount`, `/GetBalanceAgent`, `/ReloadBalanceAgent` (ยิง gateway + เขียน DB), `/GetBillPayment`, `/GetBillPaymentByID` (IDOR), `/GetConfigByKey`, `/GetReportProfitLossQueryExtend`
- `/GetMemberByUsername` คืน `password` / `otp_code` / `token` / `session_id` · `/GetBankAll` คืน `citizen_id` / `refresh_token` / `token_proxy_bank` / `api_auth` · `/GetWorkingLog` คืน `data_new` (อาจเป็น request body ทั้งก้อน)
- `/GetWorkingLog` ไม่ส่ง `service` = log ของทุกเว็บ · middleware ไม่เทียบ `:service` กับเว็บของผู้ใช้ (KI-4)
