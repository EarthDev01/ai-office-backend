// Package gemini เรียก Gemini API (generativelanguage.googleapis.com) แบบ streaming
//
// ██ SPIKE — ยังไม่มี tool / retry / นับ token · key ส่งทาง header ไม่ใส่ใน URL กันหลุดลง log
package gemini

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

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models/"

type Client struct {
	apiKey string // ██ secret — ห้าม log
	model  string
	http   *http.Client
}

func New(apiKey, model string) *Client {
	// ไม่ตั้ง Timeout ทั้ง request เพราะ stream ยาวได้ — ใช้ ctx ของ request คุมแทน
	return &Client{apiKey: apiKey, model: model, http: &http.Client{Transport: &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	}}}
}

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type request struct {
	SystemInstruction *content  `json:"systemInstruction,omitempty"`
	Contents          []content `json:"contents"`
	GenerationConfig  genConfig `json:"generationConfig"`
}

type genConfig struct {
	ThinkingConfig struct {
		// minimal = ไม่คิดก่อนตอบ — แชทตอบไวขึ้นเกือบเท่าตัว (วัดได้ ~3.3s เทียบ ~7.5s ของ low)
		ThinkingLevel string `json:"thinkingLevel"`
	} `json:"thinkingConfig"`
}

type chunk struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Stream(ctx context.Context, system string, turns []port.ChatTurn, onDelta func(string) error) error {
	req := request{Contents: make([]content, 0, len(turns))}
	if system != "" {
		req.SystemInstruction = &content{Parts: []part{{Text: system}}}
	}
	for _, t := range turns {
		role := "user"
		if t.Role == "ai" {
			role = "model"
		}
		req.Contents = append(req.Contents, content{Role: role, Parts: []part{{Text: t.Text}}})
	}
	req.GenerationConfig.ThinkingConfig.ThinkingLevel = "minimal"

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	url := baseURL + c.model + ":streamGenerateContent?alt=sse"
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("x-goog-api-key", c.apiKey)

	res, err := c.http.Do(hreq)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		var e chunk
		if json.Unmarshal(raw, &e) == nil && e.Error != nil {
			return fmt.Errorf("gemini %d: %s", res.StatusCode, e.Error.Message)
		}
		return fmt.Errorf("gemini %d", res.StatusCode)
	}

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // thoughtSignature ยาวได้หลาย KB ต่อบรรทัด
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var ch chunk
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &ch); err != nil {
			continue
		}
		if ch.Error != nil {
			return fmt.Errorf("gemini: %s", ch.Error.Message)
		}
		for _, cand := range ch.Candidates {
			for _, p := range cand.Content.Parts {
				if p.Thought || p.Text == "" {
					continue
				}
				if err := onDelta(p.Text); err != nil {
					return err
				}
			}
		}
	}
	return sc.Err()
}
