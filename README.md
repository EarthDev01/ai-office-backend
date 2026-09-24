# ai-office-backend

ระบบ AI กลาง + กล่องแชท (widget) ของ "AI ผู้ช่วยหลังบ้าน" (w-17)
เอกสารสั่งงาน: `Docs-workflow/w-17-ai-office/spec-v2.md` + `plan-v2.md` · สัญญาฝั่ง host: `contract-host-ai.md`

## ภาพรวม

```
office  = หลังบ้าน 1 ชุด (1 snippet · public_key) · kind = ชนิดหลังบ้าน (connectors/<kind>/)
  └ service = เว็บย่อยที่แอดมินสลับดู · มี secret_key ของตัวเอง (host ถือ) · allow_all/allowlist · โควตา
```

ลำดับ 1 คำถาม: widget → **host** `/api/ai/session/:service` (host ตรวจผู้ใช้เอง) → host → `/session` ที่นี่ด้วย `X-AI-Secret` → **ตั๋ว** → widget ถามด้วยตั๋ว (SSE) → ที่นี่ขอกุญแจดอกเล็กจาก host ด้วย grant → อ่าน `/api/ai/read/...` ของ host → การ์ด (ค่าจากระบบตรง ๆ) + คำตอบ (โมเดลไม่เห็นตัวเลข)

| โฟลเดอร์ | อะไร |
|---|---|
| `_cmd/` | entrypoint · `_cmd/connector-check` ตรวจ connector ทีละ kind |
| `internal/core/` | domain · port · service (chat, tool runner, guard, quota, admin) · connector (DSL engine) — **ห้ามมีของเฉพาะ kind** |
| `internal/adapter/` | gin handler · Mongo/file store · Redis · Anthropic · host client · metrics · Telegram |
| `connectors/` | ปลั๊กต่อ kind — ดู [`connectors/README.md`](connectors/README.md) |
| `widget/` | TS Web Component (closed Shadow DOM) → `static/widget/ai-office.v1.js` |

## รัน (dev)

```bash
docker compose -f docker-compose.dev.yml up -d      # Mongo + Redis (หรือ STORE_DRIVER=file ไม่ต้องมี)
cp .env.example .env                                  # แล้วใส่ค่าเอง (ANTHROPIC_API_KEY ฯลฯ)
cd widget && npm ci && npm run build && cd ..
go run ./_cmd                                         # :6767
```

## ทดสอบ

```bash
go test ./... -count=1                                # รวม plugin lint + contract test ของ connector จริง
go run ./_cmd/connector-check connectors/office-v10x connectors/office-abatech
cd widget && npm run type-check && npm test
```

## deploy

`Dockerfile` (widget + Go static + tzdata · `APP_MODE=production`) · `.github/workflows/build.yaml` (test → Kaniko → JFrog → webhook · develop=dev · main=uat · tag=prd)
production บังคับ: `TICKET_SECRET` / `CONSOLE_JWT_SECRET` ≥ 32 ตัว · `STORE_DRIVER=mongo` · `REDIS_URL` · ingress ห้ามบัฟเฟอร์ SSE + timeout ≥ 60 วิ
