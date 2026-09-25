// Package openaicompat เรียก LLM ที่ใช้ Chat Completions แบบ OpenAI (เช่น GLM ของ z.ai)
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ai-office-backend/internal/core/port"
)

type Client struct {
	baseURL string // เช่น https://api.z.ai/api/coding/paas/v4
	apiKey  string // ██ secret — ห้าม log
	model   string
	// thinking ส่งเป็น field "thinking" ตามรูปของ z.ai — nil = ไม่ส่ง (provider อื่นที่ไม่รู้จัก field นี้)
	thinking map[string]string
	http     *http.Client
}

func New(baseURL, apiKey, model string, thinking map[string]string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, thinking: thinking,
		http: &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 30 * time.Second}},
	}
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type request struct {
	Model         string            `json:"model"`
	Stream        bool              `json:"stream"`
	Messages      []message         `json:"messages"`
	Tools         []tool            `json:"tools,omitempty"`
	ToolChoice    string            `json:"tool_choice,omitempty"`
	MaxTokens     int               `json:"max_tokens,omitempty"`
	Thinking      map[string]string `json:"thinking,omitempty"`
	StreamOptions map[string]bool   `json:"stream_options,omitempty"`
}

type usage struct {
	PromptTokens        int `json:"prompt_tokens"`
	CompletionTokens    int `json:"completion_tokens"`
	PromptTokensDetails struct {
		CachedTokens int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

// toUsage — prompt_tokens ของ API แบบ OpenAI รวมส่วนที่มาจาก cache ไว้แล้ว จึงแยกออก
func (u usage) toUsage() port.LLMUsage {
	cached := u.PromptTokensDetails.CachedTokens
	return port.LLMUsage{InputTokens: u.PromptTokens - cached, OutputTokens: u.CompletionTokens, CacheRead: cached}
}

type apiError struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
}

func (c *Client) build(req port.LLMRequest, stream bool) request {
	r := request{Model: c.model, Stream: stream, Thinking: c.thinking, MaxTokens: req.MaxTokens}
	if req.System != "" {
		r.Messages = append(r.Messages, message{Role: "system", Content: req.System})
	}
	for _, m := range req.Messages {
		if len(m.ToolResults) > 0 {
			// ผล tool ของ OpenAI เป็นข้อความ role=tool แยกทีละตัว
			for _, tr := range m.ToolResults {
				r.Messages = append(r.Messages, message{Role: "tool", ToolCallID: tr.ToolUseID, Content: tr.Content})
			}
			if m.Text != "" {
				r.Messages = append(r.Messages, message{Role: "user", Content: m.Text})
			}
			continue
		}
		msg := message{Role: m.Role, Content: m.Text}
		for _, u := range m.ToolUses {
			args, _ := json.Marshal(u.Input)
			tc := toolCall{ID: u.ID, Type: "function"}
			tc.Function.Name = u.Name
			tc.Function.Arguments = string(args)
			msg.ToolCalls = append(msg.ToolCalls, tc)
		}
		r.Messages = append(r.Messages, msg)
	}
	for _, t := range req.Tools {
		var tl tool
		tl.Type = "function"
		tl.Function.Name = t.Name
		tl.Function.Description = t.Description
		tl.Function.Parameters = t.InputSchema
		r.Tools = append(r.Tools, tl)
	}
	if len(r.Tools) > 0 {
		r.ToolChoice = req.ToolChoice
		if r.ToolChoice == "" {
			r.ToolChoice = port.ToolChoiceAuto
		}
	}
	if stream {
		r.StreamOptions = map[string]bool{"include_usage": true}
	}
	return r
}

func (c *Client) post(ctx context.Context, r request) (*http.Response, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+c.apiKey)
	res, err := c.http.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 300)) // body บอกเหตุผลจริง (เช่น 429 เพราะผิด endpoint)
		return nil, fmt.Errorf("llm %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return res, nil
}

func (c *Client) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	res, err := c.post(ctx, c.build(req, false))
	if err != nil {
		return port.LLMResponse{}, err
	}
	defer res.Body.Close()
	var out struct {
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
		Usage usage     `json:"usage"`
		Error *apiError `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return port.LLMResponse{}, fmt.Errorf("llm: decode: %w", err)
	}
	if out.Error != nil {
		return port.LLMResponse{}, fmt.Errorf("llm: %v %s", out.Error.Code, out.Error.Message)
	}
	r := port.LLMResponse{Usage: out.Usage.toUsage()}
	if len(out.Choices) > 0 {
		m := out.Choices[0].Message
		r.Text = m.Content
		for _, tc := range m.ToolCalls {
			in := map[string]any{}
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &in)
			r.ToolUses = append(r.ToolUses, port.ToolUse{ID: tc.ID, Name: tc.Function.Name, Input: in})
		}
	}
	return r, nil
}

func (c *Client) Stream(ctx context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	var u port.LLMUsage
	res, err := c.post(ctx, c.build(req, true))
	if err != nil {
		return u, err
	}
	defer res.Body.Close()

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "[DONE]" {
			return u, nil
		}
		var ch struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
					// reasoning_content = ความคิดของ model — ไม่ส่งให้ผู้ใช้
				} `json:"delta"`
			} `json:"choices"`
			Usage *usage    `json:"usage"`
			Error *apiError `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			continue
		}
		if ch.Error != nil {
			return u, fmt.Errorf("llm: %v %s", ch.Error.Code, ch.Error.Message)
		}
		if ch.Usage != nil {
			u = ch.Usage.toUsage()
		}
		for _, choice := range ch.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return u, err
			}
		}
	}
	return u, sc.Err()
}
