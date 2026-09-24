// Package memory = repository ในหน่วยความจำของ collection ประวัติ/บันทึก (dev STORE_DRIVER=file และ test)
//
// ██ ข้อมูลหายเมื่อรีสตาร์ต — production ใช้ MongoDB เท่านั้น (P-16)
// พฤติกรรมต้องเหมือน mongodb/repository: ทุก query ที่อ่านของผู้ใช้กรอง office+service เสมอ
package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// ---------- settings ----------

type Settings struct {
	mu sync.Mutex
	v  *domain.Settings
}

func (r *Settings) Get(context.Context) (domain.Settings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.v == nil {
		return domain.Settings{}, domain.ErrNotFound
	}
	return *r.v, nil
}

func (r *Settings) Save(_ context.Context, s domain.Settings) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.v = &s
	return nil
}

// ---------- conversations ----------

type Conversations struct {
	mu sync.Mutex
	m  map[string]domain.Conversation
}

func NewConversations() *Conversations { return &Conversations{m: map[string]domain.Conversation{}} }

func (r *Conversations) Create(_ context.Context, c domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[c.ID] = c
	return nil
}

func (r *Conversations) Get(_ context.Context, officeID, serviceID, id string) (domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.m[id]
	if !ok || c.OfficeID != officeID || c.ServiceID != serviceID {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *Conversations) GetAny(_ context.Context, id string) (domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.m[id]
	if !ok {
		return domain.Conversation{}, domain.ErrNotFound
	}
	return c, nil
}

func (r *Conversations) Touch(_ context.Context, officeID, serviceID, id string, at time.Time, title string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.m[id]
	if !ok || c.OfficeID != officeID || c.ServiceID != serviceID {
		return domain.ErrNotFound
	}
	c.LastMessageAt = at
	c.MessageCount += 2
	if title != "" {
		c.Title = title
	}
	r.m[id] = c
	return nil
}

func (r *Conversations) Close(_ context.Context, officeID, serviceID, id string, at time.Time, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.m[id]
	if !ok || c.OfficeID != officeID || c.ServiceID != serviceID {
		return domain.ErrNotFound
	}
	if c.ClosedAt == nil {
		t := at
		c.ClosedAt = &t
		c.ClosedReason = reason
		r.m[id] = c
	}
	return nil
}

func (r *Conversations) ListForUser(_ context.Context, officeID, serviceID, userID string, since time.Time, limit int) ([]domain.Conversation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Conversation
	for _, c := range r.m {
		if c.OfficeID == officeID && c.ServiceID == serviceID && c.UserID == userID && !c.LastMessageAt.Before(since) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastMessageAt.After(out[j].LastMessageAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *Conversations) Search(_ context.Context, f port.ConversationFilter) ([]domain.Conversation, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Conversation
	for _, c := range r.m {
		if f.OfficeID != "" && c.OfficeID != f.OfficeID {
			continue
		}
		if f.ServiceID != "" && c.ServiceID != f.ServiceID {
			continue
		}
		if f.UserID != "" && c.UserID != f.UserID && c.UserName != f.UserID {
			continue
		}
		if f.From != nil && c.OpenedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && c.OpenedAt.After(*f.To) {
			continue
		}
		if f.Text != "" && !strings.Contains(strings.ToLower(c.Title), strings.ToLower(f.Text)) {
			continue
		}
		if f.IDs != nil && !inList(f.IDs, c.ID) {
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastMessageAt.After(out[j].LastMessageAt) })
	total := int64(len(out))
	return page(out, f.Offset, f.Limit), total, nil
}

func (r *Conversations) DeleteScope(_ context.Context, s domain.DeletionScope) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for id, c := range r.m {
		if matchScope(s, c.OfficeID, c.ServiceID, c.UserID, c.OpenedAt) {
			delete(r.m, id)
			n++
		}
	}
	return n, nil
}

// ---------- messages ----------

type Messages struct {
	mu sync.Mutex
	m  []domain.Message
}

func NewMessages() *Messages { return &Messages{} }

func (r *Messages) Insert(_ context.Context, m domain.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = append(r.m, m)
	return nil
}

func (r *Messages) ListByConversation(_ context.Context, officeID, serviceID, conversationID string, limit int) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Message
	for _, m := range r.m {
		if m.OfficeID == officeID && m.ServiceID == serviceID && m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func (r *Messages) Get(_ context.Context, id string) (domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range r.m {
		if m.ID == id {
			return m, nil
		}
	}
	return domain.Message{}, domain.ErrNotFound
}

func (r *Messages) Search(_ context.Context, f port.MessageFilter) ([]domain.Message, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Message
	for _, m := range r.m {
		if f.OfficeID != "" && m.OfficeID != f.OfficeID {
			continue
		}
		if f.ServiceID != "" && m.ServiceID != f.ServiceID {
			continue
		}
		if f.ConversationID != "" && m.ConversationID != f.ConversationID {
			continue
		}
		if f.UserID != "" && m.UserID != f.UserID && m.UserName != f.UserID {
			continue
		}
		if f.Role != "" && m.Role != f.Role {
			continue
		}
		if f.Verification != "" && m.VerificationStatus != f.Verification {
			continue
		}
		if f.From != nil && m.CreatedAt.Before(*f.From) {
			continue
		}
		if f.To != nil && m.CreatedAt.After(*f.To) {
			continue
		}
		if f.Text != "" && !strings.Contains(strings.ToLower(m.Text), strings.ToLower(f.Text)) {
			continue
		}
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if f.Ascending {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	total := int64(len(out))
	return page(out, f.Offset, f.Limit), total, nil
}

func (r *Messages) SetVerification(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.m {
		if r.m[i].ID == id {
			r.m[i].VerificationStatus = status
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *Messages) CountByVerification(_ context.Context, officeID, serviceID string) (map[string]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]int64{}
	for _, m := range r.m {
		if m.Role != domain.RoleAssistant {
			continue
		}
		if officeID != "" && m.OfficeID != officeID {
			continue
		}
		if serviceID != "" && m.ServiceID != serviceID {
			continue
		}
		out[m.VerificationStatus]++
	}
	return out, nil
}

func (r *Messages) DeleteScope(_ context.Context, s domain.DeletionScope) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	keep := r.m[:0]
	var n int64
	for _, m := range r.m {
		if matchScope(s, m.OfficeID, m.ServiceID, m.UserID, m.CreatedAt) {
			n++
			continue
		}
		keep = append(keep, m)
	}
	r.m = keep
	return n, nil
}

func (r *Messages) MessageIDsInScope(_ context.Context, s domain.DeletionScope) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, m := range r.m {
		if matchScope(s, m.OfficeID, m.ServiceID, m.UserID, m.CreatedAt) {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

func (r *Messages) Estimate(context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return int64(len(r.m)), nil
}

// ---------- verifications ----------

type Verifications struct {
	mu sync.Mutex
	m  map[string]domain.Verification // by message id
}

func NewVerifications() *Verifications { return &Verifications{m: map[string]domain.Verification{}} }

func (r *Verifications) Upsert(_ context.Context, v domain.Verification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.m[v.MessageID]; ok {
		v.ID, v.CreatedAt = old.ID, old.CreatedAt
	}
	r.m[v.MessageID] = v
	return nil
}

func (r *Verifications) GetByMessage(_ context.Context, messageID string) (domain.Verification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.m[messageID]
	if !ok {
		return domain.Verification{}, domain.ErrNotFound
	}
	return v, nil
}

func (r *Verifications) List(_ context.Context, officeID, serviceID, status string, limit, offset int) ([]domain.Verification, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.Verification
	for _, v := range r.m {
		if (officeID == "" || v.OfficeID == officeID) && (serviceID == "" || v.ServiceID == serviceID) && (status == "" || v.Status == status) {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].VerifiedAt.After(out[j].VerifiedAt) })
	return page(out, offset, limit), int64(len(out)), nil
}

func (r *Verifications) DeleteScope(_ context.Context, s domain.DeletionScope, messageIDs []string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, id := range messageIDs {
		if _, ok := r.m[id]; ok {
			delete(r.m, id)
			n++
		}
	}
	return n, nil
}

// ---------- access log ----------

type AccessLogs struct {
	mu sync.Mutex
	m  []domain.AccessLog
}

func (r *AccessLogs) Insert(_ context.Context, l domain.AccessLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = append(r.m, l)
	return nil
}

func (r *AccessLogs) List(_ context.Context, officeID, serviceID, operator string, limit, offset int) ([]domain.AccessLog, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.AccessLog
	for i := len(r.m) - 1; i >= 0; i-- {
		l := r.m[i]
		if (officeID == "" || l.OfficeID == officeID) && (serviceID == "" || l.ServiceID == serviceID) && (operator == "" || l.Operator == operator) {
			out = append(out, l)
		}
	}
	return page(out, offset, limit), int64(len(out)), nil
}

// ---------- deletion requests ----------

type Deletions struct {
	mu sync.Mutex
	m  []domain.DeletionRequest
}

func (r *Deletions) Insert(_ context.Context, d domain.DeletionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m = append(r.m, d)
	return nil
}

func (r *Deletions) Update(_ context.Context, d domain.DeletionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.m {
		if r.m[i].ID == d.ID {
			r.m[i] = d
			return nil
		}
	}
	return domain.ErrNotFound
}

func (r *Deletions) List(_ context.Context, limit, offset int) ([]domain.DeletionRequest, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.DeletionRequest, 0, len(r.m))
	for i := len(r.m) - 1; i >= 0; i-- {
		out = append(out, r.m[i])
	}
	return page(out, offset, limit), int64(len(out)), nil
}

// ---------- quota periods ----------

type Quotas struct {
	mu sync.Mutex
	m  map[string]domain.QuotaPeriod
}

func NewQuotas() *Quotas { return &Quotas{m: map[string]domain.QuotaPeriod{}} }

func (r *Quotas) Get(_ context.Context, officeID, serviceID, period string) (domain.QuotaPeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.m[domain.QuotaPeriodID(officeID, serviceID, period)]
	if !ok {
		return domain.QuotaPeriod{}, domain.ErrNotFound
	}
	return p, nil
}

func (r *Quotas) AddUsage(_ context.Context, officeID, serviceID, period string, tokens, questions int64, cost float64) (domain.QuotaPeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := domain.QuotaPeriodID(officeID, serviceID, period)
	p, ok := r.m[id]
	if !ok {
		p = domain.QuotaPeriod{ID: id, OfficeID: officeID, ServiceID: serviceID, Period: period, CreatedAt: time.Now()}
	}
	p.UsedTokens += tokens
	p.UsedQuestions += questions
	p.CostAmount += cost
	p.UpdatedAt = time.Now()
	r.m[id] = p
	return p, nil
}

func (r *Quotas) MarkAlert(_ context.Context, officeID, serviceID, period, flag string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := domain.QuotaPeriodID(officeID, serviceID, period)
	p := r.m[id]
	p.ID, p.OfficeID, p.ServiceID, p.Period = id, officeID, serviceID, period
	switch flag {
	case "alerted_80":
		if p.Alerted80 {
			return false, nil
		}
		p.Alerted80 = true
	case "alerted_95":
		if p.Alerted95 {
			return false, nil
		}
		p.Alerted95 = true
	case "cut":
		if p.CutAt != nil {
			return false, nil
		}
		t := time.Now()
		p.CutAt = &t
	default:
		return false, nil
	}
	r.m[id] = p
	return true, nil
}

func (r *Quotas) List(_ context.Context, period string) ([]domain.QuotaPeriod, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.QuotaPeriod
	for _, p := range r.m {
		if period == "" || p.Period == period {
			out = append(out, p)
		}
	}
	return out, nil
}

// ---------- rollups ----------

type Rollups struct {
	mu sync.Mutex
	m  map[string]domain.DailyRollup
}

func NewRollups() *Rollups { return &Rollups{m: map[string]domain.DailyRollup{}} }

func (r *Rollups) Add(_ context.Context, officeID, serviceID, date string, d domain.RollupDelta) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := officeID + "|" + serviceID + "|" + date
	x, ok := r.m[id]
	if !ok {
		x = domain.DailyRollup{ID: id, OfficeID: officeID, ServiceID: serviceID, Date: date, LatencyBuckets: map[string]int64{}, CreatedAt: time.Now()}
	}
	x.Conversations += d.Conversations
	x.Questions += d.Questions
	x.TokensIn += d.TokensIn
	x.TokensOut += d.TokensOut
	x.CostAmount += d.CostAmount
	x.Refusals += d.Refusals
	x.ToolErrors += d.ToolErrors
	x.GuardHits += d.GuardHits
	x.Correct += d.Correct
	x.Wrong += d.Wrong
	if d.LatencyMs > 0 {
		x.LatencySumMs += d.LatencyMs
		x.LatencyBuckets[domain.LatencyBucket(d.LatencyMs)]++
	}
	x.UpdatedAt = time.Now()
	r.m[id] = x
	return nil
}

func (r *Rollups) List(_ context.Context, officeID, serviceID, from, to string) ([]domain.DailyRollup, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []domain.DailyRollup
	for _, x := range r.m {
		if (officeID == "" || x.OfficeID == officeID) && (serviceID == "" || x.ServiceID == serviceID) &&
			(from == "" || x.Date >= from) && (to == "" || x.Date <= to) {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// ---------- helpers ----------

func page[T any](in []T, offset, limit int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []T{}
	}
	in = in[offset:]
	if limit > 0 && len(in) > limit {
		in = in[:limit]
	}
	return in
}

func matchScope(s domain.DeletionScope, officeID, serviceID, userID string, at time.Time) bool {
	if s.OfficeID == "" || officeID != s.OfficeID {
		return false
	}
	if s.ServiceID != "" && serviceID != s.ServiceID {
		return false
	}
	if s.UserID != "" && userID != s.UserID {
		return false
	}
	if s.From != nil && at.Before(*s.From) {
		return false
	}
	if s.To != nil && at.After(*s.To) {
		return false
	}
	return true
}

func inList(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
