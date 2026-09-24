package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// โหมด browser — widget ในหน้าแอดมินยิง API เดิมของหลังบ้านแทน backend
//
//	backend ──SSE "fetch" {id, method, path, query, body}──▶ widget
//	widget ──ยิง API หลังบ้านด้วย token ของแอดมินเอง──▶ หลังบ้าน
//	widget ──POST …/chat/relay/:id {status, body}──▶ backend (เก็บใน cache ผูกกับตั๋ว)
//	backend (รออ่าน cache) ──▶ ตัด envelope / ตรวจ schema / ทำการ์ด ตามปกติ
//
// ██ widget ยิงได้เฉพาะคำขอที่ backend สร้างจาก connector (path ที่ประกาศไว้เท่านั้น)
// ██ ผลผูกกับ id ของตั๋ว — ตั๋วอื่นส่งผลแทรกเข้ามาไม่ได้

type RelayRequest struct {
	Method  string
	Path    string
	Query   map[string]string
	Body    any
	Timeout time.Duration
}

type RelayFunc func(ctx context.Context, req RelayRequest) (port.HostResponse, error)

type relayCtxKey struct{}

func WithRelay(ctx context.Context, fn RelayFunc) context.Context {
	return context.WithValue(ctx, relayCtxKey{}, fn)
}

func relayFrom(ctx context.Context) RelayFunc {
	fn, _ := ctx.Value(relayCtxKey{}).(RelayFunc)
	return fn
}

var (
	errRelayUnavailable = errors.New("relay unavailable")
	errRelayTimeout     = errors.New("relay timeout")
)

const (
	relayResultTTL = 90 * time.Second
	relayMaxBody   = 2 << 20 // 2 MB
	relayPoll      = 60 * time.Millisecond
)

func relayKey(ticketID, id string) string { return "ai:relay:" + ticketID + ":" + id }

type relayResult struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// NewRelay สร้างตัวส่งคำขอผ่าน browser สำหรับ 1 คำถาม (emit = ส่ง SSE event)
func NewRelay(cache port.Cache, ticketID string, emit func(ChatEvent), now port.Clock) RelayFunc {
	if now == nil {
		now = time.Now
	}
	return func(ctx context.Context, req RelayRequest) (port.HostResponse, error) {
		if cache == nil || emit == nil {
			return port.HostResponse{}, errRelayUnavailable
		}
		id := domain.NewID()
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = 8 * time.Second
		}
		started := now()
		emit(ChatEvent{Type: "fetch", Data: map[string]any{
			"id": id, "method": req.Method, "path": req.Path, "query": req.Query, "body": req.Body,
			"timeout_ms": timeout.Milliseconds(),
		}})
		// รอผลจาก widget (+ เผื่อเวลาเดินทาง) — ช้ากว่านี้ = ดึงไม่ได้ ไม่เดา
		deadline := time.NewTimer(timeout + 3*time.Second)
		defer deadline.Stop()
		tick := time.NewTicker(relayPoll)
		defer tick.Stop()
		for {
			if b, ok, err := cache.Get(ctx, relayKey(ticketID, id)); err == nil && ok {
				var r relayResult
				if err := json.Unmarshal(b, &r); err != nil {
					return port.HostResponse{}, err
				}
				return port.HostResponse{Status: r.Status, Body: []byte(r.Body), Ms: now().Sub(started).Milliseconds()}, nil
			}
			select {
			case <-ctx.Done():
				return port.HostResponse{}, ctx.Err()
			case <-deadline.C:
				return port.HostResponse{}, errRelayTimeout
			case <-tick.C:
			}
		}
	}
}

// DeliverRelay รับผลที่ widget ยิงหลังบ้านมาให้ (ผูกกับตั๋วที่ถืออยู่)
func (s *ChatService) DeliverRelay(ctx context.Context, t domain.AccessTicket, id string, status int, body []byte) error {
	if s.Runner == nil || s.Runner.Cache == nil {
		return errRelayUnavailable
	}
	if id == "" || len(id) > 64 || len(body) > relayMaxBody {
		return domain.ErrBadRequest
	}
	b, err := json.Marshal(relayResult{Status: status, Body: string(body)})
	if err != nil {
		return err
	}
	return s.Runner.Cache.Set(ctx, relayKey(t.ID, id), b, relayResultTTL)
}
