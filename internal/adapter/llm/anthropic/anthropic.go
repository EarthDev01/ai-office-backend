// Package anthropic = port.LLM ผ่าน Anthropic API (ชั่วคราวตาม spec · โมเดลมาจาก settings)
//
// ██ คีย์อ่านจาก env ANTHROPIC_API_KEY (K8s Secret) เท่านั้น — ห้ามอยู่ในหน้าเว็บ/ไฟล์ที่ git track (P-1)
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type Client struct {
	c sdk.Client
}

// New · apiKey ว่าง = ให้ SDK หาเอง (ANTHROPIC_API_KEY)
func New(apiKey string) *Client {
	opts := []option.RequestOption{option.WithMaxRetries(1)}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	return &Client{c: sdk.NewClient(opts...)}
}

func (c *Client) params(req port.LLMRequest) sdk.MessageNewParams {
	p := sdk.MessageNewParams{
		Model:     sdk.Model(req.Model),
		MaxTokens: int64(req.MaxTokens),
		// prompt cache: system + tools คงที่ต่อ kind/service → อ่านจาก cache ได้ทุกคำถาม
		System: []sdk.TextBlockParam{{Text: req.System, CacheControl: sdk.NewCacheControlEphemeralParam()}},
		// ปิด thinking: รอบเลือกเครื่องมือใช้ tool_choice แบบบังคับ และคำตอบสั้น — ลดเวลาตอบ (เป้า p95 ≤ 8 วิ)
		Thinking: sdk.ThinkingConfigParamUnion{OfDisabled: &sdk.ThinkingConfigDisabledParam{}},
	}
	if req.Effort != "" {
		p.OutputConfig = sdk.OutputConfigParam{Effort: sdk.OutputConfigEffort(req.Effort)}
	}
	for _, t := range req.Tools {
		tp := sdk.ToolParam{
			Name:        t.Name,
			Description: sdk.String(t.Description),
			InputSchema: sdk.ToolInputSchemaParam{Properties: t.Properties, Required: t.Required},
		}
		p.Tools = append(p.Tools, sdk.ToolUnionParam{OfTool: &tp})
	}
	switch req.ToolChoice {
	case "any":
		p.ToolChoice = sdk.ToolChoiceUnionParam{OfAny: &sdk.ToolChoiceAnyParam{}}
	case "none":
		if len(req.Tools) > 0 {
			p.ToolChoice = sdk.ToolChoiceUnionParam{OfNone: &sdk.ToolChoiceNoneParam{}}
		}
	case "auto":
		p.ToolChoice = sdk.ToolChoiceUnionParam{OfAuto: &sdk.ToolChoiceAutoParam{}}
	}
	for _, m := range req.Messages {
		var blocks []sdk.ContentBlockParamUnion
		for _, b := range m.Blocks {
			switch b.Type {
			case port.BlockText:
				if b.Text != "" {
					blocks = append(blocks, sdk.NewTextBlock(b.Text))
				}
			case port.BlockToolUse:
				in := b.Input
				if in == nil {
					in = map[string]any{}
				}
				blocks = append(blocks, sdk.NewToolUseBlock(b.ToolUseID, in, b.ToolName))
			case port.BlockToolResult:
				blocks = append(blocks, sdk.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
			}
		}
		if len(blocks) == 0 {
			continue
		}
		if m.Role == "assistant" {
			p.Messages = append(p.Messages, sdk.NewAssistantMessage(blocks...))
		} else {
			p.Messages = append(p.Messages, sdk.NewUserMessage(blocks...))
		}
	}
	return p
}

func convert(msg *sdk.Message) port.LLMResponse {
	out := port.LLMResponse{
		StopReason: string(msg.StopReason),
		Model:      string(msg.Model),
		Usage: domain.Usage{
			In:         msg.Usage.InputTokens,
			Out:        msg.Usage.OutputTokens,
			CacheRead:  msg.Usage.CacheReadInputTokens,
			CacheWrite: msg.Usage.CacheCreationInputTokens,
		},
	}
	for _, block := range msg.Content {
		switch v := block.AsAny().(type) {
		case sdk.TextBlock:
			out.Blocks = append(out.Blocks, port.LLMBlock{Type: port.BlockText, Text: v.Text})
		case sdk.ToolUseBlock:
			var in map[string]any
			if raw := v.JSON.Input.Raw(); raw != "" {
				_ = json.Unmarshal([]byte(raw), &in)
			} else if len(v.Input) > 0 {
				_ = json.Unmarshal(v.Input, &in)
			}
			if in == nil {
				in = map[string]any{}
			}
			out.Blocks = append(out.Blocks, port.LLMBlock{Type: port.BlockToolUse, ToolUseID: v.ID, ToolName: v.Name, Input: in})
		}
	}
	return out
}

func wrap(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		return fmt.Errorf("%w: status %d", port.ErrLLMUnavailable, apiErr.StatusCode)
	}
	return fmt.Errorf("%w: %v", port.ErrLLMUnavailable, err)
}

func (c *Client) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	msg, err := c.c.Messages.New(ctx, c.params(req))
	if err != nil {
		return port.LLMResponse{}, wrap(err)
	}
	return convert(msg), nil
}

func (c *Client) Stream(ctx context.Context, req port.LLMRequest, onText func(string)) (port.LLMResponse, error) {
	stream := c.c.Messages.NewStreaming(ctx, c.params(req))
	defer stream.Close()
	msg := sdk.Message{}
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return convert(&msg), wrap(err)
		}
		if d, ok := ev.AsAny().(sdk.ContentBlockDeltaEvent); ok {
			if t, ok := d.Delta.AsAny().(sdk.TextDelta); ok && t.Text != "" && onText != nil {
				onText(t.Text)
			}
		}
	}
	if err := stream.Err(); err != nil {
		return convert(&msg), wrap(err)
	}
	return convert(&msg), nil
}
