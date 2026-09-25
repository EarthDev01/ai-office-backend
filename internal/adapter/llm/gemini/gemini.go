// Package gemini เรียก Gemini API (generativelanguage.googleapis.com)
//
// ใช้ทดสอบเท่านั้น — production ใช้ Claude · key ส่งทาง header ไม่ใส่ใน URL กันหลุดลง log
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

type functionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type functionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type functionDecl struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parametersJsonSchema"`
}

type request struct {
	SystemInstruction *content  `json:"systemInstruction,omitempty"`
	Contents          []content `json:"contents"`
	Tools             []struct {
		FunctionDeclarations []functionDecl `json:"functionDeclarations"`
	} `json:"tools,omitempty"`
	ToolConfig *struct {
		FunctionCallingConfig struct {
			Mode string `json:"mode"`
		} `json:"functionCallingConfig"`
	} `json:"toolConfig,omitempty"`
	GenerationConfig genConfig `json:"generationConfig"`
}

type genConfig struct {
	MaxOutputTokens int `json:"maxOutputTokens,omitempty"`
	ThinkingConfig  struct {
		// minimal = ไม่คิดก่อนตอบ — แชทตอบไวขึ้นเกือบเท่าตัว (วัดได้ ~3.3s เทียบ ~7.5s ของ low)
		ThinkingLevel string `json:"thinkingLevel"`
	} `json:"thinkingConfig"`
}

type usageMeta struct {
	PromptTokenCount        int `json:"promptTokenCount"`
	CandidatesTokenCount    int `json:"candidatesTokenCount"`
	CachedContentTokenCount int `json:"cachedContentTokenCount"`
}

// toUsage — promptTokenCount รวมส่วนที่มาจาก cache ไว้แล้ว จึงแยกออก
func (u usageMeta) toUsage() port.LLMUsage {
	return port.LLMUsage{InputTokens: u.PromptTokenCount - u.CachedContentTokenCount, OutputTokens: u.CandidatesTokenCount, CacheRead: u.CachedContentTokenCount}
}

type response struct {
	Candidates []struct {
		Content content `json:"content"`
	} `json:"candidates"`
	UsageMetadata usageMeta `json:"usageMetadata"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) build(req port.LLMRequest) request {
	r := request{Contents: make([]content, 0, len(req.Messages))}
	if req.System != "" {
		r.SystemInstruction = &content{Parts: []part{{Text: req.System}}}
	}
	for _, m := range req.Messages {
		role := "user"
		if m.Role == "assistant" {
			role = "model"
		}
		var parts []part
		if m.Text != "" {
			parts = append(parts, part{Text: m.Text})
		}
		for _, u := range m.ToolUses {
			parts = append(parts, part{
				FunctionCall:     &functionCall{ID: u.ID, Name: u.Name, Args: u.Input},
				ThoughtSignature: u.Signature, // Gemini 3 ต้องได้ signature เดิมกลับไป ไม่งั้นตอบ 400
			})
		}
		for _, tr := range m.ToolResults {
			parts = append(parts, part{FunctionResponse: &functionResponse{
				ID: tr.ToolUseID, Name: tr.Name, Response: map[string]any{"result": tr.Content},
			}})
		}
		r.Contents = append(r.Contents, content{Role: role, Parts: parts})
	}
	if len(req.Tools) > 0 {
		decls := make([]functionDecl, 0, len(req.Tools))
		for _, t := range req.Tools {
			decls = append(decls, functionDecl{Name: t.Name, Description: t.Description, Parameters: t.InputSchema})
		}
		r.Tools = append(r.Tools, struct {
			FunctionDeclarations []functionDecl `json:"functionDeclarations"`
		}{decls})
		r.ToolConfig = &struct {
			FunctionCallingConfig struct {
				Mode string `json:"mode"`
			} `json:"functionCallingConfig"`
		}{}
		r.ToolConfig.FunctionCallingConfig.Mode = "AUTO"
		if req.ToolChoice == port.ToolChoiceNone {
			r.ToolConfig.FunctionCallingConfig.Mode = "NONE"
		}
	}
	r.GenerationConfig.MaxOutputTokens = req.MaxTokens
	r.GenerationConfig.ThinkingConfig.ThinkingLevel = "minimal"
	return r
}

func (c *Client) post(ctx context.Context, method string, r request) (*http.Response, error) {
	body, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+c.model+method, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("x-goog-api-key", c.apiKey)
	res, err := c.http.Do(hreq)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 300))
		return nil, fmt.Errorf("gemini %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	return res, nil
}

func (c *Client) Complete(ctx context.Context, req port.LLMRequest) (port.LLMResponse, error) {
	res, err := c.post(ctx, ":generateContent", c.build(req))
	if err != nil {
		return port.LLMResponse{}, err
	}
	defer res.Body.Close()
	var out response
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return port.LLMResponse{}, fmt.Errorf("gemini: decode: %w", err)
	}
	if out.Error != nil {
		return port.LLMResponse{}, fmt.Errorf("gemini: %s", out.Error.Message)
	}
	r := port.LLMResponse{Usage: out.UsageMetadata.toUsage()}
	if len(out.Candidates) > 0 { // ใช้ candidate แรกเท่านั้น
		for i, p := range out.Candidates[0].Content.Parts {
			switch {
			case p.FunctionCall != nil:
				id := p.FunctionCall.ID
				if id == "" {
					id = fmt.Sprintf("call_%d", i)
				}
				r.ToolUses = append(r.ToolUses, port.ToolUse{
					ID: id, Name: p.FunctionCall.Name, Input: p.FunctionCall.Args, Signature: p.ThoughtSignature,
				})
			case !p.Thought && p.Text != "":
				r.Text += p.Text
			}
		}
	}
	return r, nil
}

func (c *Client) Stream(ctx context.Context, req port.LLMRequest, onDelta func(string) error) (port.LLMUsage, error) {
	var u port.LLMUsage
	res, err := c.post(ctx, ":streamGenerateContent?alt=sse", c.build(req))
	if err != nil {
		return u, err
	}
	defer res.Body.Close()

	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 64*1024), 1024*1024) // thoughtSignature ยาวได้หลาย KB ต่อบรรทัด
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var ch response
		if err := json.Unmarshal([]byte(strings.TrimSpace(line[5:])), &ch); err != nil {
			continue
		}
		if ch.Error != nil {
			return u, fmt.Errorf("gemini: %s", ch.Error.Message)
		}
		if ch.UsageMetadata.PromptTokenCount > 0 {
			u = ch.UsageMetadata.toUsage()
		}
		for _, cand := range ch.Candidates {
			for _, p := range cand.Content.Parts {
				if p.Thought || p.Text == "" {
					continue
				}
				if err := onDelta(p.Text); err != nil {
					return u, err
				}
			}
		}
	}
	return u, sc.Err()
}
