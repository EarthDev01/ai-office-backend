// Package metrics = /metrics ของ Prometheus (V3 · spec §7.2 ops)
//
//	ai_messages_count{office,service,category,result}   คำถามที่ตอบ (ทั้งสำเร็จ/ล้ม)
//	ai_chat_latency_seconds{office,service}             เวลาตอบรวม (เป้า p95 ≤ 8 วิ — AC-2)
//	ai_llm_tokens_total{office,service,direction}       token เข้า/ออก (ต้นทุน)
//	ai_guard_hits_total{office,service}                 ตัวเลขที่โมเดลพิมพ์เองแล้วถูกตัด (R-17)
//	ai_tool_calls_total{tool,result,cached}             การยิง host
//	ai_tool_latency_seconds{tool}
//	ai_messages_rows                                    จำนวนแถว messages (R-9: ~3 ล้าน = สัญญาณเฟส 2)
package metrics

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ai-office-backend/internal/core/domain"
)

type Metrics struct {
	reg       *prometheus.Registry
	messages  *prometheus.CounterVec
	latency   *prometheus.HistogramVec
	tokens    *prometheus.CounterVec
	guard     *prometheus.CounterVec
	toolCalls *prometheus.CounterVec
	toolLat   *prometheus.HistogramVec
	rows      prometheus.Gauge
	sessions  *prometheus.CounterVec
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		messages: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_messages_count", Help: "คำถามที่ AI ตอบ"},
			[]string{"office", "service", "category", "result"}),
		latency: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ai_chat_latency_seconds", Help: "เวลาตอบทั้งหมดต่อคำถาม",
			Buckets: []float64{1, 2, 3, 4, 5, 6, 8, 10, 15, 25}}, []string{"office", "service"}),
		tokens: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_llm_tokens_total", Help: "token ของโมเดล"},
			[]string{"office", "service", "direction"}),
		guard: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_guard_hits_total", Help: "ค่าที่ output guard ตัดออก"},
			[]string{"office", "service"}),
		toolCalls: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_tool_calls_total", Help: "การยิง API ของ host"},
			[]string{"tool", "result", "cached"}),
		toolLat: prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ai_tool_latency_seconds", Help: "เวลาที่ host ตอบ",
			Buckets: []float64{.1, .25, .5, 1, 2, 3, 5, 8}}, []string{"tool"}),
		rows: prometheus.NewGauge(prometheus.GaugeOpts{Name: "ai_messages_rows", Help: "จำนวนแถวใน messages (ประมาณ)"}),
		sessions: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ai_sessions_total", Help: "การขอตั๋วจาก host"},
			[]string{"result"}),
	}
	reg.MustRegister(m.messages, m.latency, m.tokens, m.guard, m.toolCalls, m.toolLat, m.rows, m.sessions,
		prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}

func (m *Metrics) Message(officeID, serviceID, category string, latency time.Duration, u domain.Usage, guardHits int, failed bool) {
	result := "ok"
	if failed {
		result = "error"
	}
	m.messages.WithLabelValues(officeID, serviceID, category, result).Inc()
	m.latency.WithLabelValues(officeID, serviceID).Observe(latency.Seconds())
	m.tokens.WithLabelValues(officeID, serviceID, "in").Add(float64(u.In + u.CacheRead + u.CacheWrite))
	m.tokens.WithLabelValues(officeID, serviceID, "out").Add(float64(u.Out))
	if guardHits > 0 {
		m.guard.WithLabelValues(officeID, serviceID).Add(float64(guardHits))
	}
}

func (m *Metrics) ToolCall(tool string, ok bool, cached bool, ms int64) {
	result := "ok"
	if !ok {
		result = "error"
	}
	m.toolCalls.WithLabelValues(tool, result, strconv.FormatBool(cached)).Inc()
	if !cached {
		m.toolLat.WithLabelValues(tool).Observe(float64(ms) / 1000)
	}
}

func (m *Metrics) Session(result string) { m.sessions.WithLabelValues(result).Inc() }

// WatchRows อัปเดตจำนวนแถว messages ทุก 5 นาที (ใช้ estimatedDocumentCount — ไม่หนัก)
func (m *Metrics) WatchRows(ctx context.Context, count func(context.Context) (int64, error)) {
	tick := func() {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if n, err := count(c); err == nil {
			m.rows.Set(float64(n))
		}
	}
	tick()
	t := time.NewTicker(5 * time.Minute)
	go func() {
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				tick()
			}
		}
	}()
}
