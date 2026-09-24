package port

import "context"

// ChatTurn คือข้อความ 1 ชิ้นในบทสนทนาที่ส่งให้ LLM — Role เป็น "user" หรือ "ai"
type ChatTurn struct {
	Role string
	Text string
}

// LLMClient ส่งบทสนทนาให้ model แล้วคืนคำตอบทีละส่วนผ่าน onDelta
//
// onDelta คืน error เมื่อฝั่งผู้รับไปต่อไม่ได้ (client ปิดการเชื่อมต่อ) — Stream ต้องหยุดทันที
type LLMClient interface {
	Stream(ctx context.Context, system string, turns []ChatTurn, onDelta func(text string) error) error
}
