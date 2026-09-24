// Package openaicompat = port.LLM ผ่าน API แบบ OpenAI Chat Completions
//
// ใช้กับผู้ให้บริการ/โมเดลใดก็ได้ที่พูด API รูปแบบนี้ (local model: Ollama · vLLM · LM Studio · llama.cpp server
// และบริการอย่าง GLM / Gemini แบบ OpenAI-compatible) — core ไม่ผูกค่ายใด (port.LLM)
//
// ██ คีย์มาจาก env (LLM_API_KEY) เท่านั้น · local model ส่วนใหญ่ไม่ต้องใช้คีย์
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type Client struct {
	baseURL string
	apiKey  string
	model   string
	// extra = field เพิ่มใน request ตามผู้ให้บริการ (เช่น ปิด thinking) — มาจาก LLM_EXTRA_BODY (JSON)
	extra map[string]any
	http  *http.Client
}

// New · baseURL เช่น http://localhost:11434/v1 (Ollama) · model ว่าง = ตาม settings
func New(baseURL, apiKey, model string, extra map[string]any) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey, model: model, extra: extra,
		http: &http.Client{Timeout: 120 * time.Second},
	}
}

// ---------- request ----------

type fnCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolCall struct {
	Index    *int   `json:"index,omitempty"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function fnCall `json:"function"`
}

type message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

func str(s string) *string { return &s }

func (c *Client) body(req port.LLMRequest, stream bool) map[string]any {
	model := req.Model
	if c.model != "" {
		model = c.model
	}
	msgs := []message{}
	if req.System != "" {
		msgs = append(msgs, message{Role: "system", Content: str(req.System)})
	}
	for _, m := range req.Messages {
		if m.Role == "assistant" {
			am := message{Role: "assistant"}
			var text strings.Builder
			for _, b := range m.Blocks {
				switch b.Type {
				case port.BlockText:
					text.WriteString(b.Text)
				case port.BlockToolUse:
					in := b.Input
					if in == nil {
						in = map[string]any{}
					}
					args, _ := json.Marshal(in)
					am.ToolCalls = append(am.ToolCalls, toolCall{ID: b.ToolUseID, Type: "function", Function: fnCall{Name: b.ToolName, Arguments: string(args)}})
				}
			}
			if text.Len() > 0 || len(am.ToolCalls) == 0 {
				am.Content = str(text.String())
			}
			msgs = append(msgs, am)
			continue
		}
		// user: ข้อความ + ผลของ tool (role "tool" ต่อ 1 call)
		var text strings.Builder
		for _, b := range m.Blocks {
			switch b.Type {
			case port.BlockToolResult:
				content := b.Content
				if b.IsError && !strings.Contains(content, "error") {
					content = `{"error":` + jsonString(content) + `}`
				}
				msgs = append(msgs, message{Role: "tool", ToolCallID: b.ToolUseID, Content: str(content)})
			case port.BlockText:
				text.WriteString(b.Text)
			}
		}
		if text.Len() > 0 {
			msgs = append(msgs, message{Role: "user", Content: str(text.String())})
		}
	}
	body := map[string]any{"model": model, "messages": msgs}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, t := range req.Tools {
			props := t.Properties
			if props == nil {
				props = map[string]any{}
			}
			params := map[string]any{"type": "object", "properties": props}
			if len(t.Required) > 0 {
				params["required"] = t.Required
			}
			tools = append(tools, map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": params,
			}})
		}
		body["tools"] = tools
		switch req.ToolChoice {
		case "any":
			body["tool_choice"] = "required"
		case "none":
			body["tool_choice"] = "none"
		case "auto":
			body["tool_choice"] = "auto"
		}
	}
	if stream {
		body["stream"] = true
		body["stream_options"] = map[string]any{"include_usage": true}
	}
	for k, v := range c.extra {
		body[k] = v
	}
	return body
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func (c *Client) post(ctx context.Context, body map[string]any) (*http.Response, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		r.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(r)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", port.ErrLLMUnavailable, err)
	}
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		return nil, fmt.Errorf("%w: status %d", port.ErrLLMUnavailable, resp.StatusCode)
	}
	return resp, nil
}

// ---------- response ----------

type usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	Details          *struct {
		CachedTokens int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
}

func (u *usage) domain() domain.Usage {
	if u == nil {
		return domain.Usage{}
	}
	out := domain.Usage{In: u.PromptTokens, Out: u.CompletionTokens}
	if u.Details != nil && u.Details.CachedTokens > 0 {
		out.CacheRead = u.Details.CachedTokens
		out.In -= u.Details.CachedTokens
	}
	return out
}

type completion struct {
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Content   *string    `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

func toolUse(tc toolCall) port.LLMBlock {
	in := map[string]any{}
	if strings.TrimSpace(tc.Function.Arguments) != "" {
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &in)
	}
	id := tc.ID
	if id == "" {
		id = "call_" + domain.NewID()
	}
	return port.LLMBlock{Type: port.BlockToolUse, ToolUseID: id, ToolName: tc.Function.Name, Input: in}
}

func stopReason(finish string, hasTools bool) string {
	switch {
	case hasTools || finish == "tool_calls":
		return "tool_use"
	case finish == "length":
		return "max_tokens"
	}
	return "end_turn"
}

func (c *Client) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	resp, err := c.post(ctx, c.body(req, false))
	if err != nil {
		return port.LLMResponse{}, err
	}
	defer resp.Body.Close()
	var cm completion
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&cm); err != nil {
		return port.LLMResponse{}, fmt.Errorf("%w: %v", port.ErrLLMUnavailable, err)
	}
	out := port.LLMResponse{Model: cm.Model, Usage: cm.Usage.domain()}
	if len(cm.Choices) == 0 {
		return out, fmt.Errorf("%w: no choices", port.ErrLLMUnavailable)
	}
	ch := cm.Choices[0]
	if ch.Message.Content != nil && *ch.Message.Content != "" {
		out.Blocks = append(out.Blocks, port.LLMBlock{Type: port.BlockText, Text: *ch.Message.Content})
	}
	for _, tc := range ch.Message.ToolCalls {
		out.Blocks = append(out.Blocks, toolUse(tc))
	}
	out.StopReason = stopReason(ch.FinishReason, len(ch.Message.ToolCalls) > 0)
	return out, nil
}

type chunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content   *string    `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *usage `json:"usage"`
}

func (c *Client) Stream(ctx context.Context, req port.LLMRequest, onText func(string)) (port.LLMResponse, error) {
	resp, err := c.post(ctx, c.body(req, true))
	if err != nil {
		return port.LLMResponse{}, err
	}
	defer resp.Body.Close()
	out := port.LLMResponse{}
	var text strings.Builder
	calls := map[int]*toolCall{}
	order := []int{}
	finish := ""
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var ck chunk
		if json.Unmarshal([]byte(data), &ck) != nil {
			continue
		}
		if ck.Model != "" {
			out.Model = ck.Model
		}
		if ck.Usage != nil {
			out.Usage = ck.Usage.domain()
		}
		for _, ch := range ck.Choices {
			if ch.Delta.Content != nil && *ch.Delta.Content != "" {
				text.WriteString(*ch.Delta.Content)
				if onText != nil {
					onText(*ch.Delta.Content)
				}
			}
			for _, tc := range ch.Delta.ToolCalls {
				i := 0
				if tc.Index != nil {
					i = *tc.Index
				}
				cur, ok := calls[i]
				if !ok {
					cur = &toolCall{}
					calls[i] = cur
					order = append(order, i)
				}
				if tc.ID != "" {
					cur.ID = tc.ID
				}
				if tc.Function.Name != "" {
					cur.Function.Name = tc.Function.Name
				}
				cur.Function.Arguments += tc.Function.Arguments
			}
			if ch.FinishReason != nil {
				finish = *ch.FinishReason
			}
		}
	}
	if err := sc.Err(); err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return out, ctx.Err()
		}
		return out, fmt.Errorf("%w: %v", port.ErrLLMUnavailable, err)
	}
	if text.Len() > 0 {
		out.Blocks = append(out.Blocks, port.LLMBlock{Type: port.BlockText, Text: text.String()})
	}
	for _, i := range order {
		out.Blocks = append(out.Blocks, toolUse(*calls[i]))
	}
	out.StopReason = stopReason(finish, len(order) > 0)
	return out, nil
}
