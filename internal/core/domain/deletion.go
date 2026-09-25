package domain

import "time"

// DeletionScope คือขอบเขตการลบ — office บังคับ และต้องมีอย่างน้อย 1 อย่างใน service / ผู้ใช้ / ช่วงเวลา
type DeletionScope struct {
	OfficeID  string     `json:"office_id"            bson:"office_id"`
	ServiceID string     `json:"service_id,omitempty" bson:"service_id,omitempty"`
	User      string     `json:"user,omitempty"       bson:"user,omitempty"` // admin_id หรือ username ตามที่ผู้ขอพิมพ์
	From      *time.Time `json:"from,omitempty"       bson:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"         bson:"to,omitempty"`
}

// DeletionResult คือจำนวนที่ลบจริง
type DeletionResult struct {
	Messages      int64 `json:"messages"      bson:"messages"`
	Conversations int64 `json:"conversations" bson:"conversations"`
	Verifications int64 `json:"verifications" bson:"verifications"`
}

// DeletionRequest คือคำขอลบ 1 ครั้ง — เก็บถาวรเป็นใบรับรองการลบ
//
// Status: running → done | failed
type DeletionRequest struct {
	ID          string         `json:"id"                     bson:"_id"`
	Scope       DeletionScope  `json:"scope"                  bson:"scope"`
	Reason      string         `json:"reason"                 bson:"reason"`
	RequestedBy string         `json:"requested_by"           bson:"requested_by"`
	Status      string         `json:"status"                 bson:"status"`
	Result      DeletionResult `json:"result"                 bson:"result"`
	Error       string         `json:"error,omitempty"        bson:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"             bson:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}
