package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Office คือ record ของ officeลูกค้า 1 เจ้า (1 โดเมนหรือหลายโดเมนของลูกค้าเจ้าเดียว)
//
// officeลูกค้า อย่าง office-v10x เป็นโค้ดชุดเดียวที่ deploy ไปหลายโดเมน snippet จึงเหมือนกันทุกโดเมน
// เราแยกลูกค้าแต่ละเจ้าด้วย "โดเมนที่เรียกเข้ามา" (Origin) แทน key ใน snippet:
//
//	allowed_origins  → บอกว่าเป็นลูกค้าเจ้าไหน · 1 โดเมนอยู่ได้แค่ office เดียว
//	service          → มาจาก localStorage["web-service"] ของหน้านั้น (เปลี่ยนได้ตลอดโดยไม่ต้องแก้ snippet)
//
// public_key เลิกใช้ระบุ office แล้ว — เก็บไว้เพราะ Mongo มี unique index อยู่ และ snippet เก่ายังพกมา
type Office struct {
	ID             string   `json:"id"                 bson:"_id"`
	Label          string   `json:"label"              bson:"label"`
	PublicKey      string   `json:"public_key"         bson:"public_key"`
	AllowedOrigins []string `json:"allowed_origins"    bson:"allowed_origins"`
	// HostAPIBase — URL API หลังบ้านที่ widget ของ office นี้ยิง · ว่าง = {origin}/api ตาม connector
	// ใช้กับ office ที่หน้าเว็บกับ API อยู่คนละโดเมน (เช่น หน้า dev ที่ localhost) — ทุกโดเมนของ office นี้ยิงไปที่เดียวกัน
	HostAPIBase string    `json:"host_api_base" bson:"host_api_base,omitempty"`
	Enabled     bool      `json:"enabled"            bson:"enabled"`
	IsHidden    bool      `json:"is_hidden"          bson:"is_hidden"` // โหลด widget แต่ไม่โชว์ปุ่มลอย ให้ office เรียกเปิดเอง
	Theme       string    `json:"theme"              bson:"theme"`
	Placement   Placement `json:"placement"          bson:"placement"`
	Services    []Service `json:"services"           bson:"services"`
	CreatedAt   time.Time `json:"created_at"         bson:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"         bson:"updated_at"`
	UpdatedBy   string    `json:"updated_by"         bson:"updated_by"`
}

// Service คือแบรนด์/เว็บย่อยใต้ office หนึ่ง
//
// ค่าที่อยู่ตรงนี้คือค่าที่ "ต่างกันได้ระหว่างแบรนด์"
// ส่วน theme/placement อยู่ที่ระดับ office เพราะเป็นเรื่องหน้าตาของหลังบ้านชุดเดียวกัน
// ถ้าให้ต่างกันรายแบรนด์ ปุ่มจะเด้งไปมาตอนแอดมินสลับ service
type Service struct {
	ID          string   `json:"id"           bson:"id"`
	Label       string   `json:"label"        bson:"label"`
	Enabled     bool     `json:"enabled"      bson:"enabled"`
	Allowlist   []string `json:"allowlist"    bson:"allowlist"`
	DisplayName string   `json:"display_name" bson:"display_name"`
	Greeting    string   `json:"greeting"     bson:"greeting"`
	AvatarURL   string   `json:"avatar_url"   bson:"avatar_url"`
}

func (o *Office) FindService(id string) (Service, bool) {
	for _, s := range o.Services {
		if s.ID == id {
			return s, true
		}
	}
	return Service{}, false
}

// AllowsOrigin เทียบหลังทำให้เป็นรูปแบบเดียวกันแล้ว — Origin ว่างไม่ผ่าน เพราะใช้ระบุ office ไม่ได้
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

// NormalizeHostAPIBase ตรวจ URL API หลังบ้าน: http(s)://host[:port][/path] ห้ามมี query, fragment, user:pass
// คืนค่าที่ตัด / ท้ายแล้ว และ scheme/host เป็นตัวพิมพ์เล็ก
func NormalizeHostAPIBase(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	u, err := url.Parse(v)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("URL API ไม่ถูกต้อง — เจอ %q", raw)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("URL API ต้องขึ้นต้นด้วย http:// หรือ https:// — เจอ %q", raw)
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("URL API ห้ามมี query, # หรือ user:pass — เจอ %q", raw)
	}
	return scheme + "://" + strings.ToLower(u.Host) + strings.TrimRight(u.EscapedPath(), "/"), nil
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
		Allowlist:   []string{},
		DisplayName: "ผู้ช่วยหลังบ้าน",
		Greeting:    "สวัสดีครับ ผมเป็นผู้ช่วยหลังบ้าน เป็นระบบอัตโนมัติไม่ใช่คนนะครับ",
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
