package domain

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
	Label            *string    `json:"label"`
	AllowedOrigins   *[]string  `json:"allowed_origins"`
	BackofficeAPIURL *string    `json:"backoffice_api_url"`
	Enabled          *bool      `json:"enabled"`
	IsHidden         *bool      `json:"is_hidden"`
	Theme            *string    `json:"theme"`
	Placement        *Placement `json:"placement"`
}

type UpdateService struct {
	Label       *string   `json:"label"`
	Enabled     *bool     `json:"enabled"`
	Allowlist   *[]string `json:"allowlist"`
	DisplayName *string   `json:"display_name"`
	Greeting    *string   `json:"greeting"`
	AvatarURL   *string   `json:"avatar_url"`
}

// Bootstrap คือสิ่งเดียวที่ widget ได้เห็น
//
// ไม่มี allowlist / backoffice_api_url / รายชื่อ service อื่น อยู่ในนี้
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
	Theme        string    `json:"theme"`
	Placement    Placement `json:"placement"`
}
