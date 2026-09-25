// Package anthropic เรียก Claude ผ่าน Messages API (SDK ทางการ) — ตัวที่ใช้บน production
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"ai-office-backend/internal/core/port"
)

type Client struct {
	c      sdk.Client
	model  string
	effort string // low | medium | high … — แชทหลังบ้านต้องตอบไว ค่าเริ่มต้น low
}

// New — apiKey ██ secret ห้าม log
func New(apiKey, model, effort string) *Client {
	if effort == "" {
		effort = "low"
	}
	return &Client{c: sdk.NewClient(option.WithAPIKey(apiKey)), model: model, effort: effort}
}

func (c *Client) params(req port.LLMRequest) sdk.MessageNewParams {
	p := sdk.MessageNewParams{
		Model:        sdk.Model(c.model),
		MaxTokens:    int64(req.MaxTokens),
		Messages:     toMessages(req.Messages),
		OutputConfig: sdk.OutputConfigParam{Effort: sdk.OutputConfigEffort(c.effort)},
	}
	if req.System != "" {
		p.System = []sdk.TextBlockParam{{Text: req.System}}
	}
	// ถ้าในประวัติมี tool_use/tool_result ต้องประกาศ tools ด้วยเสมอ แม้รอบนี้ห้ามเรียก
	for _, t := range req.Tools {
		tp := sdk.ToolParam{
			Name:        t.Name,
			Description: sdk.String(t.Description),
			InputSchema: toSchema(t.InputSchema),
		}
		p.Tools = append(p.Tools, sdk.ToolUnionParam{OfTool: &tp})
	}
	if len(p.Tools) > 0 {
		if req.ToolChoice == port.ToolChoiceNone {
			p.ToolChoice = sdk.ToolChoiceUnionParam{OfNone: &sdk.ToolChoiceNoneParam{}}
		} else {
			p.ToolChoice = sdk.ToolChoiceUnionParam{OfAuto: &sdk.ToolChoiceAutoParam{}}
		}
	}
	return p
}

func toSchema(s map[string]any) sdk.ToolInputSchemaParam {
	out := sdk.ToolInputSchemaParam{Properties: s["properties"], ExtraFields: map[string]any{}}
	if req, ok := s["required"].([]string); ok {
		out.Required = req
	}
	for k, v := range s {
		if k != "properties" && k != "required" && k != "type" {
			out.ExtraFields[k] = v
		}
	}
	return out
}

func toMessages(in []port.LLMMessage) []sdk.MessageParam {
	out := make([]sdk.MessageParam, 0, len(in))
	for _, m := range in {
		var blocks []sdk.ContentBlockParamUnion
		if m.Text != "" {
			blocks = append(blocks, sdk.NewTextBlock(m.Text))
		}
		for _, u := range m.ToolUses {
			blocks = append(blocks, sdk.NewToolUseBlock(u.ID, u.Input, u.Name))
		}
		for _, r := range m.ToolResults {
			blocks = append(blocks, sdk.NewToolResultBlock(r.ToolUseID, r.Content, false))
		}
		if m.Role == "assistant" {
			out = append(out, sdk.NewAssistantMessage(blocks...))
		} else {
			out = append(out, sdk.NewUserMessage(blocks...))
		}
	}
	return out
}

func (c *Client) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	msg, err := c.c.Messages.New(ctx, c.params(req))
	if err != nil {
		return port.LLMResponse{}, wrap(err)
	}
	var res port.LLMResponse
	for _, b := range msg.Content {
		switch v := b.AsAny().(type) {
		case sdk.TextBlock:
			res.Text += v.Text
		case sdk.ToolUseBlock:
			in := map[string]any{}
			_ = json.Unmarshal([]byte(v.JSON.Input.Raw()), &in)
			res.ToolUses = append(res.ToolUses, port.ToolUse{ID: v.ID, Name: v.Name, Input: in})
		}
	}
	res.Usage = port.LLMUsage{InputTokens: int(msg.Usage.InputTokens), OutputTokens: int(msg.Usage.OutputTokens)}
	return res, nil
}

func (c *Client) Stream(ctx context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	stream := c.c.Messages.NewStreaming(ctx, c.params(req))
	defer stream.Close()
	var usage port.LLMUsage
	for stream.Next() {
		switch ev := stream.Current().AsAny().(type) {
		case sdk.MessageStartEvent:
			usage.InputTokens = int(ev.Message.Usage.InputTokens)
		case sdk.MessageDeltaEvent:
			usage.OutputTokens = int(ev.Usage.OutputTokens)
		case sdk.ContentBlockDeltaEvent:
			if d, ok := ev.Delta.AsAny().(sdk.TextDelta); ok && d.Text != "" {
				if err := onDelta(d.Text); err != nil {
					return usage, err
				}
			}
		}
	}
	if err := stream.Err(); err != nil {
		return usage, wrap(err)
	}
	return usage, nil
}

// wrap ใส่ status + body (ตัด 300 ตัวอักษร) ให้เห็นสาเหตุใน log — ไม่มี key อยู่ใน body
func wrap(err error) error {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		raw := apiErr.RawJSON()
		if len(raw) > 300 {
			raw = raw[:300]
		}
		return fmt.Errorf("anthropic %d: %s", apiErr.StatusCode, raw)
	}
	return fmt.Errorf("anthropic: %w", err)
}
