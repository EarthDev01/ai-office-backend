// Package httpreq คุยกับ office-api-v10
//
// ██ อ่านอย่างเดียวเท่านั้น — client ตัวนี้ไม่มี method ที่ POST/PATCH/DELETE ได้
// ██ เป็นการบังคับที่ระดับ transport ไม่ใช่ที่ระดับ logic (GC-2)
package httpreq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

type BackofficeClient struct {
	http *http.Client
}

func NewBackofficeClient(timeout time.Duration) port.BackofficeAPI {
	return &BackofficeClient{http: &http.Client{Timeout: timeout}}
}

// employeeResponse สะท้อนรูปที่ office-api-v10 ตอบกลับ
// helper.Response(c, code, message, data, ...) → {"code":..,"message":..,"data":..}
type employeeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Role     struct {
			Name       string `json:"name"`
			Level      int32  `json:"level"`
			Permission []struct {
				Code string `json:"code"`
			} `json:"permission"`
			ListService []struct {
				Service    string `json:"service"`
				Permission bool   `json:"permission"`
			} `json:"list_service"`
		} `json:"role"`
	} `json:"data"`
}

func (c *BackofficeClient) EmployeeByID(ctx context.Context, baseURL string, cred domain.Credential) (domain.Caller, error) {
	if baseURL == "" {
		return domain.Caller{}, fmt.Errorf("office ยังไม่ได้ตั้ง backoffice_api_url")
	}
	if cred.Token == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	url := strings.TrimRight(baseURL, "/") + "/api/employees-byid"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.Caller{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cred.Token)

	// ██ O9 — office-api-v10 คำนวณ secret ของ JWT จาก User-Agent + IP ของ client
	// ██ จึงต้องส่งของเดิมไปด้วย ไม่งั้น verify ไม่ผ่าน (backoffice-api-survey.md §2.2)
	// ██ นี่เป็นสะพานชั่วคราว ทางที่ถูกคือขอ service token แยก (survey §2.4 ข้อ B)
	if cred.UserAgent != "" {
		req.Header.Set("User-Agent", cred.UserAgent)
	}
	if cred.ClientIP != "" {
		req.Header.Set("CF-Connecting-IP", cred.ClientIP)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return domain.Caller{}, domain.ErrUpstream
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		return domain.Caller{}, domain.ErrSessionExpired
	}
	if res.StatusCode != http.StatusOK {
		return domain.Caller{}, domain.ErrUpstream
	}

	var body employeeResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return domain.Caller{}, domain.ErrUpstream
	}
	// office-api ตอบ 200 เสมอแล้วบอกผลจริงใน code — 401/4011 คือ token หมดอายุ
	if body.Code == 401 || body.Code == 4011 {
		return domain.Caller{}, domain.ErrSessionExpired
	}
	if body.Data.ID == "" && body.Data.Username == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	caller := domain.Caller{
		AdminID:  body.Data.ID,
		Username: body.Data.Username,
		RoleName: body.Data.Role.Name,
		Level:    body.Data.Role.Level,
		Cred:     cred,
	}
	if caller.AdminID == "" {
		caller.AdminID = body.Data.Username
	}
	for _, p := range body.Data.Role.Permission {
		if p.Code != "" {
			caller.Permissions = append(caller.Permissions, p.Code)
		}
	}
	// เอาเฉพาะ service ที่ Permission == true — ตัวที่ false คือมีชื่อแต่ไม่ให้เข้า
	for _, s := range body.Data.Role.ListService {
		if s.Permission && s.Service != "" {
			caller.Services = append(caller.Services, s.Service)
		}
	}
	return caller, nil
}
