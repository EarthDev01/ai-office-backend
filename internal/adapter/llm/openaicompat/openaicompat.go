// Package openaicompat เรียก LLM ที่ใช้ Chat Completions แบบ OpenAI (เช่น GLM ของ z.ai) แบบ streaming
//
// ██ SPIKE — ยังไม่มี tool / retry / นับ token
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

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model    string            `json:"model"`
	Stream   bool              `json:"stream"`
	Messages []message         `json:"messages"`
	Thinking map[string]string `json:"thinking,omitempty"`
}

type chunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			// reasoning_content = ความคิดของ model — ไม่ส่งให้ผู้ใช้
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Stream(ctx context.Context, system string, turns []port.ChatTurn, onDelta func(string) error) error {
	req := request{Model: c.model, Stream: true, Thinking: c.thinking, Messages: make([]message, 0, len(turns)+1)}
	if system != "" {
		req.Messages = append(req.Messages, message{Role: "system", Content: system})
	}
	for _, t := range turns {
		role := "user"
		if t.Role == "ai" {
			role = "assistant"
		}
		req.Messages = append(req.Messages, message{Role: role, Content: t.Text})
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+c.apiKey)

	res, err := c.http.Do(hreq)
	if err != nil {
		return fmt.Errorf("llm: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		var e chunk
		if json.Unmarshal(raw, &e) == nil && e.Error != nil {
			return fmt.Errorf("llm %d: %s %s", res.StatusCode, e.Error.Code, e.Error.Message)
		}
		return fmt.Errorf("llm %d", res.StatusCode)
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "[DONE]" {
			return nil
		}
		var ch chunk
		if err := json.Unmarshal([]byte(data), &ch); err != nil {
			continue
		}
		if ch.Error != nil {
			return fmt.Errorf("llm: %s %s", ch.Error.Code, ch.Error.Message)
		}
		for _, choice := range ch.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
