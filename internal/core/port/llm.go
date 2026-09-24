package port

import (
	"context"
	"errors"

	"ai-office-backend/internal/core/domain"
)

// LLM แยก core ออกจาก SDK ของผู้ให้บริการ — สลับไป local model ได้โดยไม่แตะ core (OQ-5)

type LLMTool struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

const (
	BlockText       = "text"
	BlockToolUse    = "tool_use"
	BlockToolResult = "tool_result"
)

type LLMBlock struct {
	Type      string
	Text      string
	ToolUseID string
	ToolName  string
	Input     map[string]any
	Content   string // tool_result (JSON string)
	IsError   bool
}

type LLMMessage struct {
	Role   string // user | assistant
	Blocks []LLMBlock
}

type LLMRequest struct {
	Model      string
	System     string
	Messages   []LLMMessage
	Tools      []LLMTool
	ToolChoice string // auto | any | none
	MaxTokens  int
	Effort     string
}

type LLMResponse struct {
	Blocks     []LLMBlock
	StopReason string
	Usage      domain.Usage
	Model      string
}

var (
	ErrLLMUnavailable = errors.New("LLM_UNAVAILABLE")
	ErrLLMRefused     = errors.New("LLM_REFUSED")
)

type LLM interface {
	// Complete = รอบตัดสินใจ (เลือก tool) ไม่ต้อง stream
	Complete(ctx context.Context, req LLMRequest) (LLMResponse, error)
	// Stream = รอบเขียนคำตอบ · onText ได้ข้อความทีละชิ้น
	Stream(ctx context.Context, req LLMRequest, onText func(string)) (LLMResponse, error)
}
