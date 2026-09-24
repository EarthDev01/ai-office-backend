package domain

import "time"

// ---- ระบบตรวจคำตอบ (K3) ----

const (
	VerifyPending = "pending"
	VerifyCorrect = "correct"
	VerifyWrong   = "wrong"
)

// 5 ประเภทความผิดที่ต้องแยก (spec §7.5 verifications)
var VerificationErrorTypes = map[string]string{
	"wrong_number":   "ตัวเลขไม่ตรง",
	"wrong_question": "ตอบผิดคำถาม",
	"wrong_menu":     "บอกเมนูผิด",
	"should_refuse":  "ควรปฏิเสธแต่ตอบ",
	"should_answer":  "ควรตอบแต่ปฏิเสธ",
}

// Verification = ผลตรวจ 1 ข้อความ · ชุดทดสอบถาวร · ไม่ส่งกลับไปแก้ห้องเดิม
type Verification struct {
	ID             string    `json:"id"              bson:"_id"`
	OfficeID       string    `json:"office_id"       bson:"office_id"`
	ServiceID      string    `json:"service_id"      bson:"service_id"`
	MessageID      string    `json:"message_id"      bson:"message_id"`
	ConversationID string    `json:"conversation_id" bson:"conversation_id"`
	Status         string    `json:"status"          bson:"status"`
	ErrorType      string    `json:"error_type"      bson:"error_type"`
	CorrectAnswer  string    `json:"correct_answer"  bson:"correct_answer"`
	Note           string    `json:"note"            bson:"note"`
	VerifiedBy     string    `json:"verified_by"     bson:"verified_by"`
	VerifiedAt     time.Time `json:"verified_at"     bson:"verified_at"`
	CreatedAt      time.Time `json:"created_at"      bson:"created_at"`
}

// ---- บันทึกการเข้าถึงจากคอนโซล (P-13 · AC-11) ----

type AccessLog struct {
	ID             string    `json:"id"              bson:"_id"`
	Operator       string    `json:"operator"        bson:"operator"`
	Action         string    `json:"action"          bson:"action"` // read_conversation | read_message | read_old | search | delete
	OfficeID       string    `json:"office_id"       bson:"office_id"`
	ServiceID      string    `json:"service_id"      bson:"service_id"`
	ConversationID string    `json:"conversation_id" bson:"conversation_id"`
	MessageID      string    `json:"message_id"      bson:"message_id"`
	Detail         string    `json:"detail"          bson:"detail"`
	At             time.Time `json:"at"              bson:"at"`
	CreatedAt      time.Time `json:"created_at"      bson:"created_at"`
}

// ---- ลบตามคำขอ PDPA (K5) ----

type DeletionScope struct {
	OfficeID  string     `json:"office_id"  bson:"office_id"`
	ServiceID string     `json:"service_id" bson:"service_id"`
	UserID    string     `json:"user_id"    bson:"user_id"`
	From      *time.Time `json:"from,omitempty" bson:"from,omitempty"`
	To        *time.Time `json:"to,omitempty"   bson:"to,omitempty"`
}

type DeletionResult struct {
	Messages      int64 `json:"messages"      bson:"messages"`
	Conversations int64 `json:"conversations" bson:"conversations"`
	Verifications int64 `json:"verifications" bson:"verifications"`
}

type DeletionRequest struct {
	ID          string         `json:"id"           bson:"_id"`
	OfficeID    string         `json:"office_id"    bson:"office_id"`
	ServiceID   string         `json:"service_id"   bson:"service_id"`
	Scope       DeletionScope  `json:"scope"        bson:"scope"`
	Reason      string         `json:"reason"       bson:"reason"`
	RequestedBy string         `json:"requested_by" bson:"requested_by"`
	ApprovedBy  string         `json:"approved_by"  bson:"approved_by"`
	Status      string         `json:"status"       bson:"status"` // done | failed
	Result      DeletionResult `json:"result"       bson:"result"`
	Error       string         `json:"error,omitempty" bson:"error,omitempty"`
	CreatedAt   time.Time      `json:"created_at"   bson:"created_at"`
	CompletedAt *time.Time     `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// ---- โควตา (V9) · ตัวนับสดอยู่ Redis · ที่นี่คือบันทึกถาวรต่อรอบเดือน ----

type QuotaPeriod struct {
	ID            string     `json:"id"             bson:"_id"` // office|service|YYYY-MM
	OfficeID      string     `json:"office_id"      bson:"office_id"`
	ServiceID     string     `json:"service_id"     bson:"service_id"`
	Period        string     `json:"period"         bson:"period"`
	UsedTokens    int64      `json:"used_tokens"    bson:"used_tokens"`
	UsedQuestions int64      `json:"used_questions" bson:"used_questions"`
	CostAmount    float64    `json:"cost_amount"    bson:"cost_amount"`
	Alerted80     bool       `json:"alerted_80"     bson:"alerted_80"`
	Alerted95     bool       `json:"alerted_95"     bson:"alerted_95"`
	CutAt         *time.Time `json:"cut_at,omitempty" bson:"cut_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"     bson:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"     bson:"updated_at"`
}

func QuotaPeriodID(officeID, serviceID, period string) string {
	return officeID + "|" + serviceID + "|" + period
}

// QuotaStatus = มุมมองสำหรับคอนโซล/การตัดสิน
type QuotaStatus struct {
	OfficeID      string         `json:"office_id"`
	ServiceID     string         `json:"service_id"`
	ServiceLabel  string         `json:"service_label"`
	Period        string         `json:"period"`
	Limit         int64          `json:"limit"`
	UsedTokens    int64          `json:"used_tokens"`
	UsedQuestions int64          `json:"used_questions"`
	Percent       float64        `json:"percent"`
	CostAmount    float64        `json:"cost_amount"`
	Currency      string         `json:"currency"`
	Cut           bool           `json:"cut"`
	TempIncreases []TempIncrease `json:"temp_increases"`
}

// ---- สรุปรายวัน (ฐานของหน้าภาพรวมเฟส 2) ----

type DailyRollup struct {
	ID             string           `json:"id"             bson:"_id"` // office|service|YYYY-MM-DD
	OfficeID       string           `json:"office_id"      bson:"office_id"`
	ServiceID      string           `json:"service_id"     bson:"service_id"`
	Date           string           `json:"date"           bson:"date"`
	Conversations  int64            `json:"conversations"  bson:"conversations"`
	Questions      int64            `json:"questions"      bson:"questions"`
	TokensIn       int64            `json:"tokens_in"      bson:"tokens_in"`
	TokensOut      int64            `json:"tokens_out"     bson:"tokens_out"`
	CostAmount     float64          `json:"cost_amount"    bson:"cost_amount"`
	LatencySumMs   int64            `json:"latency_sum_ms" bson:"latency_sum_ms"`
	LatencyBuckets map[string]int64 `json:"latency_buckets" bson:"latency_buckets"`
	Refusals       int64            `json:"refusals"       bson:"refusals"`
	ToolErrors     int64            `json:"tool_errors"    bson:"tool_errors"`
	GuardHits      int64            `json:"guard_hits"     bson:"guard_hits"`
	Correct        int64            `json:"correct"        bson:"correct"`
	Wrong          int64            `json:"wrong"          bson:"wrong"`
	CreatedAt      time.Time        `json:"created_at"     bson:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"     bson:"updated_at"`
}

// RollupDelta = สิ่งที่จะบวกเพิ่มเข้า DailyRollup (upsert $inc)
type RollupDelta struct {
	Conversations int64
	Questions     int64
	TokensIn      int64
	TokensOut     int64
	CostAmount    float64
	LatencyMs     int64 // > 0 = นับเข้า bucket ด้วย
	Refusals      int64
	ToolErrors    int64
	GuardHits     int64
	Correct       int64
	Wrong         int64
}

// LatencyBucket แบ่งช่วงเวลาตอบไว้ประมาณ p50/p95 โดยไม่ต้องเก็บทุกค่า
func LatencyBucket(ms int64) string {
	switch {
	case ms <= 2000:
		return "le_2s"
	case ms <= 4000:
		return "le_4s"
	case ms <= 6000:
		return "le_6s"
	case ms <= 8000:
		return "le_8s"
	case ms <= 12000:
		return "le_12s"
	default:
		return "gt_12s"
	}
}
