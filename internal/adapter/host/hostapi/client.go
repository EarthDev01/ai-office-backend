// Package hostapi = HTTP client กลางที่ backend ใช้คุยกับหลังบ้าน (host) ทุก kind แบบ server-to-server
//
// ██ ไม่มีทางแนบ token ของแอดมิน — header ที่ส่งมาจาก ToolRunner เท่านั้น (กุญแจดอกเล็ก) (P-9 · AC-32)
// ██ ไม่ follow redirect (กันถูกพาไปที่อื่น) · จำกัดขนาด response
// ██ พฤติกรรมเฉพาะของแต่ละ kind อยู่ใน connectors/<kind>/host.yaml ไม่ใช่ที่นี่
package hostapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ai-office-backend/internal/core/port"
)

const maxBody = 4 << 20 // 4 MB

type Client struct {
	http *http.Client
	ua   string
}

func New() *Client {
	return &Client{
		http: &http.Client{
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			Transport: &http.Transport{
				Proxy:               http.ProxyFromEnvironment,
				MaxIdleConnsPerHost: 16,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 5 * time.Second,
			},
		},
		ua: "ai-office-backend/1",
	}
}

// forbiddenHeaders = header ที่ ToolRunner ห้ามส่งต่อ (ป้องกันพลาด) — ตัวตนผู้ใช้/ IP ของแอดมิน
var forbiddenHeaders = map[string]bool{
	"authorization": true, "cookie": true, "cf-connecting-ip": true, "x-forwarded-for": true, "headertoken": true,
}

func (c *Client) Do(ctx context.Context, r port.HostRequest) (port.HostResponse, error) {
	u, err := url.Parse(r.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return port.HostResponse{}, fmt.Errorf("url ของ host ผิด")
	}
	if len(r.Query) > 0 {
		q := u.Query()
		for k, v := range r.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
	}
	var body io.Reader
	if r.Body != nil {
		b, err := json.Marshal(r.Body)
		if err != nil {
			return port.HostResponse{}, err
		}
		body = bytes.NewReader(b)
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 6 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(r.Method), u.String(), body)
	if err != nil {
		return port.HostResponse{}, err
	}
	req.Header.Set("User-Agent", c.ua)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range r.Header {
		if forbiddenHeaders[strings.ToLower(k)] {
			return port.HostResponse{}, fmt.Errorf("header %s ห้ามส่งไป host", k)
		}
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := c.http.Do(req)
	ms := time.Since(start).Milliseconds()
	if err != nil {
		return port.HostResponse{Ms: ms}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return port.HostResponse{Status: resp.StatusCode, Ms: ms}, err
	}
	if len(b) > maxBody {
		return port.HostResponse{Status: resp.StatusCode, Ms: ms}, fmt.Errorf("response ใหญ่เกิน %d bytes", maxBody)
	}
	return port.HostResponse{Status: resp.StatusCode, Body: b, Ms: time.Since(start).Milliseconds()}, nil
}
