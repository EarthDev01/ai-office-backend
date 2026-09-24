# ai-office-backend — API + widget bundle (spec-v2 §5 · plan-v2 R/V/W)
#
# stage 1: build widget (ไฟล์เดียว static/widget/ai-office.v1.js — build ใหม่ทุกครั้ง ไม่เชื่อไฟล์ที่ commit ไว้)
FROM node:22-alpine AS widget
WORKDIR /src/widget
COPY widget/package.json widget/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY widget/ ./
RUN mkdir -p ../static/widget && npm run build

# stage 2: build Go (static binary)
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY _cmd ./_cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ai-office ./_cmd

# stage 3: runtime
FROM alpine:3.20
# tzdata: ตัดวันตาม Asia/Bangkok (B-6) · ca-certificates: เรียก Anthropic / host ผ่าน https
RUN apk add --no-cache tzdata ca-certificates && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /out/ai-office ./ai-office
COPY --from=widget /src/static/widget ./static/widget
COPY connectors ./connectors
ENV APP_MODE=production \
    HTTP_PORT=6767 \
    CONNECTORS_DIR=/app/connectors \
    TZ=Asia/Bangkok
USER app
EXPOSE 6767
# K8s ใช้ /healthz (liveness) + /readyz (readiness) · ingress ต้องไม่บัฟเฟอร์ SSE และ timeout ≥ 60 วิ (PRE-6)
ENTRYPOINT ["./ai-office"]
