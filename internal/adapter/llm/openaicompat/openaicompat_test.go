package openaicompat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ai-office-backend/internal/core/port"
)

func server(t *testing.T, handle func(body map[string]any, w http.ResponseWriter)) (*httptest.Server, *map[string]any) {
	var last map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("auth %q", r.Header.Get("Authorization"))
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &last)
		handle(last, w)
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func TestComplete_ToolCallAndMapping(t *testing.T) {
	srv, last := server(t, func(_ map[string]any, w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"model":"m1","choices":[{"message":{"content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"withdraw_pending","arguments":"{\"date\":\"2026-09-24\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":100,"completion_tokens":7}}`))
	})
	c := New(srv.URL+"/v1", "k", "", map[string]any{"thinking": map[string]any{"type": "disabled"}})
	res, err := c.Complete(context.Background(), port.LLMRequest{
		Model: "local-model", System: "sys", ToolChoice: "any", MaxTokens: 50,
		Tools: []port.LLMTool{{Name: "withdraw_pending", Description: "d", Properties: map[string]any{"date": map[string]any{"type": "string"}}}},
		Messages: []port.LLMMessage{
			{Role: "user", Blocks: []port.LLMBlock{{Type: port.BlockText, Text: "ถอนค้าง"}}},
			{Role: "assistant", Blocks: []port.LLMBlock{{Type: port.BlockToolUse, ToolUseID: "c0", ToolName: "x", Input: map[string]any{"a": 1}}}},
			{Role: "user", Blocks: []port.LLMBlock{{Type: port.BlockToolResult, ToolUseID: "c0", Content: `{"ok":true}`}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Blocks) != 1 || res.Blocks[0].ToolName != "withdraw_pending" || res.Blocks[0].Input["date"] != "2026-09-24" || res.StopReason != "tool_use" {
		t.Fatalf("tool call: %+v", res)
	}
	if res.Usage.In != 100 || res.Usage.Out != 7 {
		t.Fatalf("usage: %+v", res.Usage)
	}
	b := *last
	if b["model"] != "local-model" || b["tool_choice"] != "required" || b["thinking"] == nil {
		t.Fatalf("body: %+v", b)
	}
	msgs := b["messages"].([]any)
	if msgs[0].(map[string]any)["role"] != "system" || msgs[2].(map[string]any)["tool_calls"] == nil || msgs[3].(map[string]any)["role"] != "tool" {
		t.Fatalf("messages: %+v", msgs)
	}
}

func TestStream_TextDeltas(t *testing.T) {
	srv, last := server(t, func(_ map[string]any, w http.ResponseWriter) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, s := range []string{`{"choices":[{"delta":{"content":"สวัส"}}]}`, `{"choices":[{"delta":{"content":"ดีครับ"},"finish_reason":"stop"}]}`, `{"choices":[],"usage":{"prompt_tokens":5,"completion_tokens":3}}`} {
			_, _ = w.Write([]byte("data: " + s + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	c := New(srv.URL+"/v1", "k", "forced-model", nil)
	var got strings.Builder
	res, err := c.Stream(context.Background(), port.LLMRequest{Model: "x", ToolChoice: "none"}, func(s string) { got.WriteString(s) })
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "สวัสดีครับ" || res.StopReason != "end_turn" || res.Usage.Out != 3 {
		t.Fatalf("stream: %q %+v", got.String(), res)
	}
	if (*last)["model"] != "forced-model" || (*last)["stream"] != true {
		t.Fatalf("body: %+v", *last)
	}
}

func TestErrorStatus(t *testing.T) {
	srv, _ := server(t, func(_ map[string]any, w http.ResponseWriter) { w.WriteHeader(500) })
	_, err := New(srv.URL+"/v1", "k", "", nil).Complete(context.Background(), port.LLMRequest{})
	if err == nil || !strings.Contains(err.Error(), "LLM_UNAVAILABLE") {
		t.Fatalf("err: %v", err)
	}
}
