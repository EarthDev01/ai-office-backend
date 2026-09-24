package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Office คือ "การติดตั้ง 1 ชุด" — หลังบ้าน 1 ระบบที่เอา script ไปแปะ
//
// 1 office มีได้หลาย service (แบรนด์/เว็บย่อย) ที่แอดมินสลับไปมาในหน้าเดิม
// จึงต้องแยก 2 ชั้น:
//
//	public_key  อยู่ใน snippet  → บอกว่าหน้านี้เป็นของ office ไหน (ไม่ใช่ความลับ)
//	service     มาจากหน้าเว็บ   → host เป็นผู้ยืนยันตอนออกตั๋ว (ดู contract-host-ai.md)
//
// Kind = ชนิดหลังบ้าน = ปลั๊ก connectors/<kind>/ ที่ใช้ · office ที่ยังไม่ตั้ง kind ใช้ widget ไม่ได้
type Office struct {
	ID               string    `json:"id"                 bson:"_id"`
	Label            string    `json:"label"              bson:"label"`
	Kind             string    `json:"kind"               bson:"kind"`
	PublicKey        string    `json:"public_key"         bson:"public_key"`
	AllowedOrigins   []string  `json:"allowed_origins"    bson:"allowed_origins"`
	BackofficeAPIURL string    `json:"backoffice_api_url" bson:"backoffice_api_url"`
	Enabled          bool      `json:"enabled"            bson:"enabled"`
	IsHidden         bool      `json:"is_hidden"          bson:"is_hidden"` // โหลด widget แต่ไม่โชว์ปุ่มลอย ให้ office เรียกเปิดเอง
	Theme            string    `json:"theme"              bson:"theme"`
	Placement        Placement `json:"placement"          bson:"placement"`
	Services         []Service `json:"services"           bson:"services"`
	CreatedAt        time.Time `json:"created_at"         bson:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"         bson:"updated_at"`
	UpdatedBy        string    `json:"updated_by"         bson:"updated_by"`
}

// Service คือแบรนด์/เว็บย่อยใต้ office หนึ่ง
//
// ค่าที่อยู่ตรงนี้คือค่าที่ "ต่างกันได้ระหว่างแบรนด์"
// ส่วน theme/placement อยู่ที่ระดับ office เพราะเป็นเรื่องหน้าตาของหลังบ้านชุดเดียวกัน
// ถ้าให้ต่างกันรายแบรนด์ ปุ่มจะเด้งไปมาตอนแอดมินสลับ service
//
// ██ secret_key เก็บแค่ hash (P-8) · field hash ถูกซ่อนจาก JSON (json:"-") — คอนโซลเห็นแค่สถานะ
type Service struct {
	ID          string   `json:"id"           bson:"id"`
	Label       string   `json:"label"        bson:"label"`
	Enabled     bool     `json:"enabled"      bson:"enabled"`
	AllowAll    bool     `json:"allow_all"    bson:"allow_all"` // true = ทุกคนที่ host ยืนยันแล้ว · false = เฉพาะ allowlist (ว่าง = ไม่มีใคร)
	Allowlist   []string `json:"allowlist"    bson:"allowlist"`
	DisplayName string   `json:"display_name" bson:"display_name"`
	Greeting    string   `json:"greeting"     bson:"greeting"`
	AvatarURL   string   `json:"avatar_url"   bson:"avatar_url"`

	Quota ServiceQuota `json:"quota" bson:"quota"`

	SecretKeyHash     string     `json:"-"                             bson:"secret_key_hash,omitempty"`
	SecretKeyPrevHash string     `json:"-"                             bson:"secret_key_prev_hash,omitempty"`
	SecretCreatedAt   *time.Time `json:"secret_created_at,omitempty"   bson:"secret_created_at,omitempty"`
	SecretRotatedAt   *time.Time `json:"secret_rotated_at,omitempty"   bson:"secret_rotated_at,omitempty"`
	SecretLastUsedAt  *time.Time `json:"secret_last_used_at,omitempty" bson:"secret_last_used_at,omitempty"`

	// สถานะที่คำนวณก่อนตอบคอนโซล (ไม่เก็บลง DB)
	HasSecret     bool `json:"has_secret"      bson:"-"`
	HasPrevSecret bool `json:"has_prev_secret" bson:"-"`
}

// ServiceQuota = เพดานการใช้ต่อเดือน (หน่วย token: input+output) · ≤0 = ใช้ค่าเริ่มต้นจาก settings
type ServiceQuota struct {
	MonthlyLimit  int64          `json:"monthly_limit"  bson:"monthly_limit"`
	TempIncreases []TempIncrease `json:"temp_increases" bson:"temp_increases"`
}

// TempIncrease = เพิ่มเพดานชั่วคราวเฉพาะรอบเดือน Period (YYYY-MM · Bangkok) — บันทึกผู้ทำเสมอ
type TempIncrease struct {
	Period string    `json:"period" bson:"period"`
	Amount int64     `json:"amount" bson:"amount"`
	By     string    `json:"by"     bson:"by"`
	Reason string    `json:"reason" bson:"reason"`
	At     time.Time `json:"at"     bson:"at"`
}

// EffectiveLimit คือเพดานของรอบ period — ไม่มีทาง "ไม่จำกัด" (service ใหม่มีเพดานเสมอ)
func (q ServiceQuota) EffectiveLimit(period string, defaultLimit int64) int64 {
	limit := q.MonthlyLimit
	if limit <= 0 {
		limit = defaultLimit
	}
	for _, t := range q.TempIncreases {
		if t.Period == period {
			limit += t.Amount
		}
	}
	return limit
}

func (s *Service) HasActiveSecret() bool { return s.SecretKeyHash != "" }

// Redacted คืนสำเนาที่เติมสถานะ secret ให้คอนโซล (hash ถูกซ่อนด้วย json:"-" อยู่แล้ว)
func (o Office) Redacted() Office {
	cp := o
	cp.Services = make([]Service, len(o.Services))
	for i, s := range o.Services {
		s.HasSecret = s.SecretKeyHash != ""
		s.HasPrevSecret = s.SecretKeyPrevHash != ""
		cp.Services[i] = s
	}
	return cp
}

func (o *Office) FindService(id string) (Service, bool) {
	for _, s := range o.Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

func (o *Office) ServiceIndex(id string) int {
	for i := range o.Services {
		if o.Services[i].ID == id {
			return i
		}
	}
	return -1
}

// AllowsOrigin — Origin ว่างถือว่าเป็น same-origin (เบราว์เซอร์ไม่ส่ง header นี้มา)
// AllowsOrigin เทียบหลังทำให้เป็นรูปแบบเดียวกันแล้ว — Origin ว่างไม่ผ่าน (fetch ข้ามโดเมนจากเบราว์เซอร์ส่ง Origin เสมอ)
func (o *Office) AllowsOrigin(origin string) bool {
	want, err := NormalizeOrigin(origin)
	if err != nil {
		return false
	}
	for _, a := range o.AllowedOrigins {
		if got, err := NormalizeOrigin(a); err == nil && got == want {
			return true
		}
	}
	return false
}

// NormalizeOrigin ทำ origin ให้เป็นรูปแบบเดียว (scheme://host[:port] ตัวพิมพ์เล็ก ไม่มี / ท้าย
// ตัดพอร์ตมาตรฐานทิ้ง) — ต้องใช้ทั้งตอนบันทึกและตอนค้น ไม่งั้น "https://A.com/" กับ
// "https://a.com" จะกลายเป็นคนละลูกค้า
func NormalizeOrigin(raw string) (string, error) {
	v := strings.TrimRight(strings.TrimSpace(raw), "/")
	if v == "" {
		return "", fmt.Errorf("origin ว่าง")
	}
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("origin ไม่ถูกต้อง — เจอ %q", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("origin ต้องขึ้นต้นด้วย http:// หรือ https:// — เจอ %q", raw)
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("origin ต้องมีแค่ scheme กับ host ห้ามมี path — เจอ %q", raw)
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func NewPublicKey() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "pk_" + hex.EncodeToString(b)
}

func DefaultService(id, label string) Service {
	return Service{
		ID:          id,
		Label:       label,
		Enabled:     false,
		AllowAll:    true, // D-74: ค่าเริ่มต้น true ตามโค้ดเดิม · เว็บจริงต้องปิดแล้วใส่ allowlist
		Allowlist:   []string{},
		DisplayName: "ผู้ช่วยหลังบ้าน",
		Greeting:    "สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",
		Quota:       ServiceQuota{TempIncreases: []TempIncrease{}},
	}
}

func NewOffice(id, label string) Office {
	now := time.Now()
	return Office{
		ID:             id,
		Label:          label,
		PublicKey:      NewPublicKey(),
		AllowedOrigins: []string{},
		Enabled:        true,
		Theme:          "auto",
		Placement:      Placement{Position: "bottom-right", OffsetX: 12, OffsetY: 12},
		Services:       []Service{},
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
