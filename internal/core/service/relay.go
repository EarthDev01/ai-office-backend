package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// ขีดจำกัดของ relay — ผลที่ widget ส่งกลับต้องไม่ใหญ่เกินและ id ต้องสั้น
const (
	RelayMaxBody  = 2 << 20
	RelayMaxIDLen = 64
	relayTTL      = 90 * time.Second
	relayPoll     = 60 * time.Millisecond
	relayGrace    = 3 * time.Second // widget ต้องมีเวลาส่งผลกลับหลังหลังบ้านตอบ
)

var (
	ErrRelayTimeout = errors.New("timeout")
	ErrRelayInvalid = errors.New("relay: id หรือ body ไม่ถูกต้อง")
)

// FetchCommand คือ SSE event "fetch" ที่สั่งให้ widget ยิง API หลังบ้านแทน backend
type FetchCommand struct {
	ID        string            `json:"id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Query     map[string]string `json:"query,omitempty"`
	Body      any               `json:"body"`
	TimeoutMs int               `json:"timeout_ms"`
}

// RelayRequest คือคำขอที่ render จาก template ของ connector แล้ว (ยังไม่มี host — host มาจาก host_api_base ฝั่ง widget)
type RelayRequest struct {
	Method string
	Path   string
	Query  map[string]string
	Body   any
}

// RelayResult คือผลที่ widget ส่งกลับ — Status 0 = network error ฝั่ง browser
type RelayResult struct {
	Status int
	Body   string
}

// Relay รับผลจาก widget (POST …/chat/relay/:id) มาส่งให้ฝั่งที่รออยู่ใน ToolRunner
//
// key ผูกกับ ticket ID — ตั๋วใบอื่นส่งผลแทรกเข้ามาไม่ได้แม้จะเดา id ถูก
// เก็บใน memory เพราะ backend รันเครื่องเดียว
type Relay struct {
	mu      sync.Mutex
	results map[string]relayEntry
}

type relayEntry struct {
	res   RelayResult
	until time.Time
}

func NewRelay() *Relay { return &Relay{results: map[string]relayEntry{}} }

func relayKey(ticketID, id string) string { return "ai:relay:" + ticketID + ":" + id }

// Deliver เก็บผลที่ widget ส่งกลับ
func (r *Relay) Deliver(ticketID, id string, res RelayResult) error {
	if id == "" || len(id) > RelayMaxIDLen || len(res.Body) > RelayMaxBody {
		return ErrRelayInvalid
	}
	now := time.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, e := range r.results { // เก็บกวาดของที่ไม่มีใครมารับ
		if now.After(e.until) {
			delete(r.results, k)
		}
	}
	r.results[relayKey(ticketID, id)] = relayEntry{res: res, until: now.Add(relayTTL)}
	return nil
}

func (r *Relay) take(key string) (RelayResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.results[key]
	if ok {
		delete(r.results, key)
	}
	return e.res, ok
}

// Fetch ส่งคำสั่ง fetch ให้ widget ผ่าน emit แล้วรอผลได้ timeout + 3 วิ
func (r *Relay) Fetch(ctx context.Context, ticketID string, req RelayRequest, timeout time.Duration, emit Emitter) (RelayResult, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	cmd := FetchCommand{ID: id, Method: req.Method, Path: req.Path, Query: req.Query, Body: req.Body, TimeoutMs: int(timeout / time.Millisecond)}
	if err := emit("fetch", cmd); err != nil {
		return RelayResult{}, err
	}

	key := relayKey(ticketID, id)
	deadline := time.NewTimer(timeout + relayGrace)
	defer deadline.Stop()
	tick := time.NewTicker(relayPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return RelayResult{}, ctx.Err()
		case <-deadline.C:
			return RelayResult{}, ErrRelayTimeout
		case <-tick.C:
			if res, ok := r.take(key); ok {
				return res, nil
			}
		}
	}
}

// ttlCache คือ cache ผลของ tool ใน memory (key ผูกกับตั๋วเสมอ)
type ttlCache struct {
	mu    sync.Mutex
	items map[string]cacheEntry
}

type cacheEntry struct {
	v     any
	until time.Time
}

func newTTLCache() *ttlCache { return &ttlCache{items: map[string]cacheEntry{}} }

func (c *ttlCache) get(k string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.items[k]
	if !ok || time.Now().After(e.until) {
		delete(c.items, k)
		return nil, false
	}
	return e.v, true
}

func (c *ttlCache) put(k string, v any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) > 5000 { // กัน map โตไม่จำกัด
		c.items = map[string]cacheEntry{}
	}
	c.items[k] = cacheEntry{v: v, until: time.Now().Add(ttl)}
}
