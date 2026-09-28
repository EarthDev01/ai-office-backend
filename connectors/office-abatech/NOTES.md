# office-abatech connector — บันทึกสำหรับ dev

ตรวจกับโค้ดจริง `office-abatech` (fb29edea) + `GOTOPOPOFFICE` (main) · 28/09/2026 · ภาพรวม endpoint: `Docs-workflow/w-17-ai-office/appendix-abatech-endpoints.md`

## 1. หลักที่ใช้
- โหมด browser: widget ยิง API เดิมด้วย `headertoken` ของแอดมิน · path สัมพัทธ์กับ `host_api_base` (= `.../api`) · เส้นต่อเว็บ `/<Endpoint>/{service}` (service ธรรมดา ไม่ base64)
- envelope หลัก `{code, msg, payload}` (`helper.Response`) · เส้นที่มี `count` ระดับบน / `code:200` / body ของ upstream ใช้ `envelope: raw`
- **backend แทบไม่ตรวจ permission code** (ส่วนใหญ่ `AuthRequiredDB(resource, "")` · บางเส้นไม่มี auth เลย) → `permissions.yaml` เป็นด่านเดียวที่ mirror หน้าจอ (เมนู + แท็บ `buttonMenu[].code`)
- สิทธิ์อ่านจาก `localStorage.token` (ใบที่มี `role.permission`) · `headertoken` ถูกล้าง permission ก่อนเซ็น
- ลิงก์บนการ์ดต่อ `?service={service_b64}` เพราะทุกหน้าต่อเว็บอ่าน `?service=` แบบ base64

## 2. endpoint ที่ตัดทิ้ง (ห้ามเพิ่มกลับโดยไม่ทบทวน)
| endpoint | เหตุผล |
|---|---|
| `GetMemberByUsername` · `GetMemberSearch` · `GetMemberAll` · `GetMemberExcell` | คืน `password` / `otp_code` / `token` / `session_id` ของสมาชิก |
| `GetBankAll` | คืน `citizen_id` / `birthdate` / `refresh_token` / `token_proxy_bank` / `api_auth` ของบัญชีบริษัท → BQ-10 ครอบแค่บัญชีฝาก |
| `CheckCredit` | เขียน working_log ในชื่อพนักงานทุกครั้ง (BQ-47 ตอบด้วย fact + guide) |
| `GetReportProfitLossQuery` · `GetDashboardSummaryALLQuery` | เขียนตาราง report (F-10) |
| `ReloadBalanceAgent` | ยิง gateway สด + upsert DB |
| `GetDashboardSummary` | ไม่มีหน้าจอเรียกแล้ว · last_* ไม่ใช่เมื่อวาน (KI-1) |
| `GetBillPaymentByID` | IDOR + มีเลขบัญชีรับเงิน |
| `GetLogLogin` · `GetWorkingLogByUser` | ต้องรู้ id พนักงาน / อ่านข้ามทุก service |
| `GetWalletStatement` · `GetSummaryWalletStatement` | กระเป๋าของระบบ ไม่ใช่เครดิตสมาชิก |

## 3. เรื่องที่ยังไม่ยืนยัน / ควรรู้
1. **timezone ของ pod** — backend ตัดวันด้วย `time.Local` (goment) · Dockerfile ไม่ได้ตั้ง `TZ` → ถ้า pod เป็น UTC "วันนี้" ของเส้นรายการฝาก/ถอน/สมาชิกใหม่จะเริ่ม 07:00 ไทย (dashboard ใช้ offset ไทยเองจึงไม่โดน)
2. ฝาก/ถอน **ทีละวัน** เท่านั้น (filter date บังคับวันเดียว) · list ถอนดึงสูงสุด 2000 แถว/วัน (การ์ดบอกเมื่อดึงได้บางส่วน) · count ตันที่ 5000
3. รายการฝาก/ถอน/รอตรวจบัญชีคืน `phone_number` / เลขบัญชี / ชื่อบัญชีลูกค้าผ่าน relay — ไม่ขึ้นการ์ดและไม่ถึงโมเดล (values เลือกเฉพาะ field ที่ประกาศ) แต่ผ่าน backend AI ในหน่วยความจำ
4. `GetBillPayment` ได้บิลของ **ทุก service ใน deployment** และรูปร่าง response มาจาก UI (ไม่มี struct ใน repo)
5. `bank_list` / `bank_config.status` ความหมาย 0/1 อนุมานจากหน้าจอ
6. เมนูที่ deployment ปิดผ่าน `GetConfigByKey?key=submenu` ยังไม่ได้ mirror → `lookup_menu` อาจแนะนำเมนูที่เว็บนั้นซ่อนอยู่
7. โปรโมชั่น: กรองแค่ `status == 1` (expression เทียบวันเวลาไม่ได้) — การ์ดแสดงช่วงวันให้ผู้ใช้ตรวจเอง
8. นอกขอบเขต: `components/Dashboard/Layout/TopNavbar.vue` `SOSFirebase` มี email/password ของ Firebase `topup-group` hardcode ในโค้ด frontend
