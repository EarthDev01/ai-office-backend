package domain

import "time"

// UsagePeriod คือการใช้ token ของ 1 service ใน 1 เดือน (เวลาไทย) · _id = office|service|YYYY-MM
//
// ยังไม่มีลิมิตหรือราคา — เก็บไว้ดูปริมาณการใช้ก่อน
type UsagePeriod struct {
	ID           string    `json:"id"           bson:"_id"`
	OfficeID     string    `json:"office_id"    bson:"office_id"`
	ServiceID    string    `json:"service_id"   bson:"service_id"`
	Period       string    `json:"period"       bson:"period"` // YYYY-MM
	Questions    int64     `json:"questions"    bson:"questions"`
	InputTokens  int64     `json:"input_tokens" bson:"input_tokens"`
	OutputTokens int64     `json:"output_tokens" bson:"output_tokens"`
	CacheRead    int64     `json:"cache_read"   bson:"cache_read"`
	CacheWrite   int64     `json:"cache_write"  bson:"cache_write"`
	UpdatedAt    time.Time `json:"updated_at"   bson:"updated_at"`
}

// DailyRollup คือสรุปรายวันของ 1 service (เวลาไทย) · _id = office|service|YYYY-MM-DD
type DailyRollup struct {
	ID            string `json:"id"             bson:"_id"`
	OfficeID      string `json:"office_id"      bson:"office_id"`
	ServiceID     string `json:"service_id"     bson:"service_id"`
	Date          string `json:"date"           bson:"date"` // YYYY-MM-DD
	Conversations int64  `json:"conversations"  bson:"conversations"`
	Questions     int64  `json:"questions"      bson:"questions"`
	InputTokens   int64  `json:"input_tokens"   bson:"input_tokens"`
	OutputTokens  int64  `json:"output_tokens"  bson:"output_tokens"`
	CacheRead     int64  `json:"cache_read"     bson:"cache_read"`
	CacheWrite    int64  `json:"cache_write"    bson:"cache_write"`
	Refusals      int64  `json:"refusals"       bson:"refusals"`
	ToolErrors    int64  `json:"tool_errors"    bson:"tool_errors"`
	GuardHits     int64  `json:"guard_hits"     bson:"guard_hits"`
	Correct       int64  `json:"correct"        bson:"correct"`
	Wrong         int64  `json:"wrong"          bson:"wrong"`
}

// RollupDelta คือจำนวนที่จะบวกเข้า rollup ของวันนั้น (ติดลบได้ — ตอนตรวจคำตอบซ้ำ)
type RollupDelta struct {
	Conversations, Questions, InputTokens, OutputTokens, CacheRead, CacheWrite int64
	Refusals, ToolErrors, GuardHits, Correct, Wrong                            int64
}

// PeriodOf = เดือน YYYY-MM ตามเวลาไทย
func PeriodOf(t time.Time) string { return t.In(Bangkok()).Format("2006-01") }

// DateOf = วัน YYYY-MM-DD ตามเวลาไทย
func DateOf(t time.Time) string { return t.In(Bangkok()).Format("2006-01-02") }
