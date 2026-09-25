package port

import "context"

// LLMMessage คือข้อความ 1 ชิ้นในบทสนทนาที่ส่งให้ LLM
//
// Role เป็น "user" หรือ "assistant" · assistant ที่เรียก tool มี ToolUses ·
// user ที่ตอบผล tool มี ToolResults (อยู่ข้อความเดียวกันทั้งหมด)
type LLMMessage struct {
	Role        string
	Text        string
	ToolUses    []ToolUse
	ToolResults []ToolResult
}

// ToolDef คือ tool ที่ประกาศให้ LLM เลือก — InputSchema เป็น JSON Schema (type: object)
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolUse คือคำขอเรียก tool จาก LLM
//
// Signature เป็นค่าทึบที่บาง provider (Gemini) ต้องให้ส่งกลับไปพร้อม tool call เดิมในรอบถัดไป
type ToolUse struct {
	ID        string
	Name      string
	Input     map[string]any
	Signature string
}

type ToolResult struct {
	ToolUseID string
	Name      string
	Content   string
}

// ToolChoice — auto = LLM เลือกเองว่าจะเรียก tool ไหม · none = ห้ามเรียก (ตอบเป็นข้อความเท่านั้น)
//
// ไม่มีแบบบังคับเรียก (any) เพราะหลาย provider ไม่รองรับหรือใช้คู่กับ thinking ไม่ได้ —
// ChatService ถือว่า "ไม่เรียก tool" = answer_directly แทน
const (
	ToolChoiceAuto = "auto"
	ToolChoiceNone = "none"
)

type LLMRequest struct {
	System     string
	Messages   []LLMMessage
	Tools      []ToolDef
	ToolChoice string
	MaxTokens  int
}

// LLMUsage — InputTokens ไม่รวมส่วนที่อ่าน/เขียน prompt cache (แยกไว้ใน CacheRead/CacheWrite)
type LLMUsage struct {
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
}

type LLMResponse struct {
	Text     string
	ToolUses []ToolUse
	Usage    LLMUsage
}

// LLMClient คุยกับ model
//
// Complete ใช้รอบเลือก tool (ไม่ต้อง stream) · Stream ใช้รอบเขียนคำตอบ
// onDelta คืน error เมื่อฝั่งผู้รับไปต่อไม่ได้ (client ปิดการเชื่อมต่อ) — Stream ต้องหยุดทันที
type LLMClient interface {
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
	Stream(ctx context.Context, req LLMRequest, onDelta func(text string) error) (LLMUsage, error)
}
