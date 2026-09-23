# ai-office-backend

API + widget bundle ของ AI ผู้ช่วยหลังบ้าน
spec อยู่ที่ `Claude/.workflow/AI Office/` — อ่าน `01-SPEC-SYSTEM.md` ก่อน

## โมเดล 2 ชั้น

```
Office  = หลังบ้าน 1 ชุด (1 การติดตั้ง = 1 snippet)   → public_key อยู่ใน snippet
  └ Service = แบรนด์/เว็บย่อยที่แอดมินสลับดู          → มาจาก session ไม่ใช่ snippet
```

1 office มีได้หลาย service และเพิ่ม/ลบ service ได้โดย **ไม่ต้องแก้ snippet** ที่ลูกค้าแปะไว้แล้ว

## ทำอะไรได้แล้ว (รอบนี้)

- `GET /widget/v1/:public_key/ai-office.js` — bundle + ETag · key ไม่มีจริง → 404
- `GET /api/ai/office/:public_key/service/:service_id/bootstrap` — ตัดสินที่ server ว่า widget ควรโผล่ไหม · ตรวจ `Origin` + สิทธิ์ใน service
- Office/Service CRUD จากคอนโซล + `rotate-key`
- CORS อ่าน `allowed_origins` จาก DB — เพิ่มลูกค้าใหม่ไม่ต้อง deploy
- ปฏิเสธ request ที่ client พยายามเลือก service เอง
- **ตรวจตัวตนกับ `office-api-v10` จริง** — `GET /api/employees-byid` ด้วย Bearer ของแอดมิน แล้วเอา `Role.ListService` มาตรวจว่าเข้า service นั้นได้ไหม (cache 60 วิ)
- **MongoDB** (`STORE_DRIVER=mongo`) หรือไฟล์ JSON (`file`) ไว้ dev เร็ว ๆ

**ยังไม่มีในรอบนี้:** LLM, tool, SSE chat, ประวัติแชท, โควตา, audit — อยู่ใน Phase 3+ ของ `05-PLAN.md`

service นี้ **ไม่เสิร์ฟหน้า HTML ใด ๆ** — มีแต่ API กับไฟล์ `ai-office.v1.js` ไฟล์เดียว
UI ของแชทถูกสร้างด้วย JS ในหน้า office เอง ไม่ได้ส่ง markup มาจากที่นี่

## รัน

```bash
cd widget && npm install && npm run build && cd ..
cp .env.example .env
go run ./_cmd                      # :6767
```

อยากดูของจริงให้รัน `Project/office-mock` ที่ :5174 (หลังบ้านจำลองของลูกค้า)
และ `Project/ai-office-report` ที่ :5173 (คอนโซล)

ครั้งแรก office `demo` ถูก seed มาให้แล้ว (key `pk_demo_local`, origin `http://localhost:5174`,
service `K11S` + `PG99` ที่ยังปิดอยู่) — เปิดใช้จากคอนโซล หรือสั่งตรง:

```bash
curl -X PATCH localhost:6767/api/ai/admin/offices/demo/services/K11S \
  -H "Authorization: Bearer dev-console-token" \
  -H "Content-Type: application/json" \
  -d '{"enabled":true,"allowlist":["adm_ploy"]}'
```

## test

```bash
go test ./test/ -v          # 43 test case
cd widget && npm test       # 19 test
```

## ของที่เป็น stub ในรอบนี้

| ส่วน | ตอนนี้ | ต้องเปลี่ยนเป็น |
|---|---|---|
| token ปลอม `dev:<admin>:<services>:<role>` | ใช้ได้เฉพาะ office ที่ยังไม่ตั้ง `backoffice_api_url` + `APP_MODE=dev` | ตั้ง `backoffice_api_url` แล้วระบบจะไปถาม office-api จริงเอง |
| forward `User-Agent`/`CF-Connecting-IP` | สะพานชั่วคราวของ `O9` | ขอ service token read-only จากทีม office-api (survey §2.4 ข้อ B) |
| `ConsoleAuth` | static token จาก env | JWT คนละ secret กับแอดมินเว็บ (`02-SPEC §6`) |
| `filestore` | ไฟล์ JSON | MongoDB — สลับที่ `_cmd/main.go` บรรทัดเดียว |
| `seedOffices()` | office `demo` hardcode | ไม่ต้องมี — สร้างจากคอนโซล |

ทุกจุดมี comment `██` กำกับไว้ในโค้ด

## ข้อจำกัดที่รู้ตัว

endpoint ที่ serve bundle **ตรวจ `Origin` ไม่ได้** เพราะ `<script src>` ไม่ส่ง header นี้มา
ด่านจริงอยู่ที่ `/bootstrap` ซึ่งเป็น `fetch` ข้าม origin จึงมี `Origin` เสมอ
ตัว bundle เป็น static ล้วน ไม่มีข้อมูลของใครอยู่ข้างใน
