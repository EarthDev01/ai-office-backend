package domain

// Placement คือตำแหน่งปุ่มลอยของ widget — อยู่ระดับ office
// เพราะหลังบ้านแต่ละชุดมีปุ่มลอยเดิมอยู่คนละมุม
type Placement struct {
	Position string `json:"position" bson:"position"` // bottom-right | bottom-left
	OffsetX  int    `json:"offset_x" bson:"offset_x"`
	OffsetY  int    `json:"offset_y" bson:"offset_y"`
}

// UpdateOffice ใช้ pointer ทุก field เพื่อให้ PATCH แยกออกว่า
// "ไม่ได้ส่งมา" (nil) กับ "ส่งมาเป็นค่าว่าง/false" ต่างกัน
type UpdateOffice struct {
	Label            *string    `json:"label"`
	Kind             *string    `json:"kind"`
	AllowedOrigins   *[]string  `json:"allowed_origins"`
	BackofficeAPIURL *string    `json:"backoffice_api_url"`
	Enabled          *bool      `json:"enabled"`
	IsHidden         *bool      `json:"is_hidden"`
	Theme            *string    `json:"theme"`
	Placement        *Placement `json:"placement"`
}

type UpdateService struct {
	Label        *string   `json:"label"`
	Enabled      *bool     `json:"enabled"`
	AllowAll     *bool     `json:"allow_all"`
	Allowlist    *[]string `json:"allowlist"`
	DisplayName  *string   `json:"display_name"`
	Greeting     *string   `json:"greeting"`
	AvatarURL    *string   `json:"avatar_url"`
	MonthlyLimit *int64    `json:"monthly_limit"`
}

// เหตุผลที่ widget/ตั๋วถูกปฏิเสธ — ค่าคงที่ที่ widget ใช้อธิบายสาเหตุใน console
const (
	ReasonOfficeDisabled  = "office_disabled"
	ReasonServiceDisabled = "service_disabled"
	ReasonNoService       = "service_not_in_office"
	ReasonNotInAllowlist  = "not_in_allowlist"
	ReasonKindMismatch    = "kind_mismatch"
	ReasonKindNotSet      = "kind_not_set"
	ReasonNoSecret        = "no_secret"
	ReasonQuotaExceeded   = "quota_exceeded"
)

// Bootstrap คือสิ่งเดียวที่ widget ได้เห็นเรื่องหน้าตา
//
// ไม่มี allowlist / backoffice_api_url / รายชื่อ service อื่น อยู่ในนี้
// server ตัดสินให้เสร็จแล้วคืนมาแค่ enabled/reason
type Bootstrap struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
	// Notice = เปิดใช้ได้แต่ตอนนี้ถามไม่ได้ (เช่น quota_exceeded) — widget โชว์ข้อความแทนการซ่อนปุ่ม
	Notice       string    `json:"notice,omitempty"`
	OfficeID     string    `json:"office_id"`
	ServiceID    string    `json:"service_id"`
	ServiceLabel string    `json:"service_label"`
	Kind         string    `json:"kind"`
	IsHidden     bool      `json:"is_hidden"`
	AvatarURL    string    `json:"avatar_url"`
	DisplayName  string    `json:"display_name"`
	Greeting     string    `json:"greeting"`
	Theme        string    `json:"theme"`
	Placement    Placement `json:"placement"`
	// ให้ widget ต่ออายุตั๋วก่อนหมด (unix วินาที)
	TicketExpiresAt int64 `json:"ticket_expires_at"`
}
