# ai-office-backend — API + widget bundle (spec-v2 §5 · plan-v2 D1)
#
# stage 1: build widget (static/widget/ai-office.v1.js — build ใหม่ทุกครั้ง ไม่เชื่อไฟล์ที่ commit ไว้)
FROM node:22-alpine AS widget
WORKDIR /src/widget
COPY widget/package.json widget/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY widget/ ./
RUN mkdir -p ../static/widget && npm run build

# stage 2: build Go (static binary) · docs/ ต้องมีเพราะคู่มือถูก embed เข้า binary
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY _cmd ./_cmd
COPY internal ./internal
COPY docs ./docs
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ai-office ./_cmd

# stage 3: runtime — binary อ่าน ./connectors และ ./static/widget จาก WORKDIR (hardcode ใน _cmd/main.go)
FROM alpine:3.20
# tzdata: ตัดวันตาม Asia/Bangkok (B-6) · ca-certificates: เรียก LLM ผ่าน https
RUN apk add --no-cache tzdata ca-certificates && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /out/ai-office ./ai-office
COPY --from=widget /src/static/widget ./static/widget
COPY connectors ./connectors
# production ต้องตั้งผ่าน Secret: DB_URI · CONSOLE_JWT_SECRET · CHAT_TICKET_SECRET · LLM_KEY_SECRET (ไม่ครบ = ไม่ boot)
ENV APP_MODE=production \
    STORE_DRIVER=mongo \
    HTTP_PORT=6767 \
    TZ=Asia/Bangkok
USER app
EXPOSE 6767
# probe ใช้ /healthz · รัน 1 replica เท่านั้น (ตั๋ว/คิว relay/ตัวนับอยู่ในหน่วยความจำ) · ingress ต้องไม่บัฟเฟอร์ SSE และ timeout ≥ 60 วิ
ENTRYPOINT ["./ai-office"]
