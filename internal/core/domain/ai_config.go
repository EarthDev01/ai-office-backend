package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Placement คือตำแหน่งปุ่มลอยของ widget — อยู่ระดับ office
// เพราะหลังบ้านแต่ละชุดมีปุ่มลอยเดิมอยู่คนละมุม (02-SPEC-BACKEND.md §8)
type Placement struct {
	Position string `json:"position" bson:"position"` // bottom-right | bottom-left
	OffsetX  int    `json:"offset_x" bson:"offset_x"`
	OffsetY  int    `json:"offset_y" bson:"offset_y"`
}

// UpdateOffice ใช้ pointer ทุก field เพื่อให้ PATCH แยกออกว่า
// "ไม่ได้ส่งมา" (nil) กับ "ส่งมาเป็นค่าว่าง/false" ต่างกัน
type UpdateOffice struct {
	Label          *string    `json:"label"`
	AllowedOrigins *[]string  `json:"allowed_origins"`
	HostAPIBase    *string    `json:"host_api_base"`
	Kind           *string    `json:"kind"`
	GroupID        *string    `json:"group_id"`
	Enabled        *bool      `json:"enabled"`
	IsHidden       *bool      `json:"is_hidden"`
	Theme          *string    `json:"theme"`
	AccentColor    *string    `json:"accent_color"`
	AccentColors   *[]string  `json:"accent_colors"`
	ColorSource    *string    `json:"color_source"`
	Placement      *Placement `json:"placement"`
	// CardStyles — ส่งมาทั้งชุด (แทนของเดิม) · ค่าว่าง = กลับไปใช้แบบตั้งต้นของ connector
	CardStyles *map[string]string `json:"card_styles"`
}

type UpdateService struct {
	Label        *string   `json:"label"`
	Enabled      *bool     `json:"enabled"`
	Allowlist    *[]string `json:"allowlist"`
	DisplayName  *string   `json:"display_name"`
	Greeting     *string   `json:"greeting"`
	AvatarURL    *string   `json:"avatar_url"`
	Tagline      *string   `json:"tagline"`
	Background   *string   `json:"background"`
	LauncherIcon *string   `json:"launcher_icon"`
}

// Bootstrap คือสิ่งเดียวที่ widget ได้เห็น
//
// ไม่มี allowlist / allowed_origins / รายชื่อ service อื่น อยู่ในนี้
// server ตัดสินให้เสร็จแล้วคืนมาแค่ enabled/reason
type Bootstrap struct {
	Enabled      bool      `json:"enabled"`
	Reason       string    `json:"reason,omitempty"` // office_disabled | service_disabled | not_in_allowlist | wrong_office
	OfficeID     string    `json:"office_id"`
	ServiceID    string    `json:"service_id"`
	ServiceLabel string    `json:"service_label"`
	IsHidden     bool      `json:"is_hidden"`
	AvatarURL    string    `json:"avatar_url"`
	DisplayName  string    `json:"display_name"`
	Greeting     string    `json:"greeting"`
	Tagline      string    `json:"tagline,omitempty"`
	LauncherIcon string    `json:"launcher_icon,omitempty"`
	Background   string    `json:"background,omitempty"`
	Theme        string    `json:"theme"`
	AccentColor  string    `json:"accent_color,omitempty"`
	AccentColors []string  `json:"accent_colors,omitempty"`
	ColorSource  string    `json:"color_source,omitempty"`
	Placement    Placement `json:"placement"`
}

// ---- หน้าตา widget ที่เลือกจากคอนโซล (รูปผู้ช่วย · พื้นหลัง) ----

// WidgetPatterns = ลายพื้นหลังที่ให้เลือกในคอนโซล (widget วาดเองตามสีหลัก — ไม่ต้องมีไฟล์รูป)
var WidgetPatterns = []string{"glow"}

// legacyPatterns = ลายที่เคยให้เลือก (06/10) — ยังรับค่าเดิมได้ widget ยังวาดได้ แต่ไม่โชว์ในคอนโซลแล้ว
var legacyPatterns = []string{"dots", "grid", "diagonal"}

// LookRandom = ให้ระบบสุ่มจากคลังรูปทุกครั้งที่ widget โหลด (bootstrap แปลงเป็นค่าจริงก่อนส่ง)
const LookRandom = "random"

var (
	assetNameRe = regexp.MustCompile(`^[a-z0-9_-]+/[A-Za-z0-9._-]+\.(png|jpe?g|webp|gif|svg)$`)
	lookURLRe   = regexp.MustCompile(`^(https://|http://localhost(:[0-9]+)?/)[A-Za-z0-9._~:/?#\[\]@!$&*+,;=%-]+$`)
)

// ValidateLookValue ตรวจค่ารูปผู้ช่วย/พื้นหลัง — ค่านี้ลงไปเป็น CSS/รูปในหน้าเว็บลูกค้า จึงรับเฉพาะรูปแบบที่ปลอดภัย
//
//	"" · pattern:<id> (เฉพาะพื้นหลัง) · asset:<โฟลเดอร์>/<ไฟล์รูป> · https://… (http://localhost ได้ตอน dev) · ห้ามมีเครื่องหมายคำพูด/วงเล็บ/ช่องว่าง
func ValidateLookValue(v string, allowPattern bool) error {
	switch {
	case v == "" || v == LookRandom:
		return nil
	case len(v) > 500:
		return fmt.Errorf("ยาวเกิน 500 ตัวอักษร")
	case strings.HasPrefix(v, "pattern:"):
		if allowPattern {
			for _, p := range append(append([]string{}, WidgetPatterns...), legacyPatterns...) {
				if v == "pattern:"+p {
					return nil
				}
			}
		}
		return fmt.Errorf("ไม่รู้จักลาย %q", v)
	case strings.HasPrefix(v, "asset:"):
		if assetNameRe.MatchString(strings.TrimPrefix(v, "asset:")) {
			return nil
		}
		return fmt.Errorf("ชื่อไฟล์ในคลังรูปไม่ถูกต้อง")
	case lookURLRe.MatchString(v):
		return nil
	}
	return fmt.Errorf("ต้องเป็นลิงก์รูปแบบ https:// หรือรูปจากคลังรูป")
}
