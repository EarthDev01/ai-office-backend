package filestore

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// chatRepo เก็บห้องแชทและข้อความเป็น JSON Lines (ใช้ตอน dev เท่านั้น)
//
// ห้องแชทและข้อความเขียนต่อท้ายทุกครั้งที่เปลี่ยน — ตอนโหลดบรรทัดหลังสุดของ id เดียวกันชนะ
type chatRepo struct {
	mu       sync.RWMutex
	convPath string
	msgPath  string
	convs    map[string]domain.Conversation
	msgs     []domain.ChatMessage
	msgIdx   map[string]int // id → ตำแหน่งใน msgs
}

func NewChatRepository(convPath, msgPath string) (port.ChatRepository, error) {
	r := &chatRepo{convPath: convPath, msgPath: msgPath, convs: map[string]domain.Conversation{}, msgIdx: map[string]int{}}
	if err := readLines(convPath, func(line []byte) {
		var c domain.Conversation
		if json.Unmarshal(line, &c) == nil {
			r.convs[c.ID] = c
		}
	}); err != nil {
		return nil, err
	}
	if err := readLines(msgPath, func(line []byte) {
		var m domain.ChatMessage
		if json.Unmarshal(line, &m) == nil {
			r.putMsg(m)
		}
	}); err != nil {
		return nil, err
	}
	return r, nil
}

func readLines(path string, fn func([]byte)) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			fn([]byte(line))
		}
	}
	return sc.Err()
}

func appendLine(path string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

func (r *chatRepo) CreateConversation(_ context.Context, c domain.Conversation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.convs[c.ID] = c
	return appendLine(r.convPath, c)
}

func (r *chatRepo) GetConversation(_ context.Context, id string) (domain.Conversation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.convs[id]
	if !ok {
		return c, domain.ErrNotFound
	}
	return c, nil
}

func (r *chatRepo) putMsg(m domain.ChatMessage) {
	if i, ok := r.msgIdx[m.ID]; ok {
		r.msgs[i] = m
		return
	}
	r.msgIdx[m.ID] = len(r.msgs)
	r.msgs = append(r.msgs, m)
}

func (r *chatRepo) TouchConversation(_ context.Context, id string, at time.Time, addMessages int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.UpdatedAt = at
	c.MessageCount += addMessages
	r.convs[id] = c
	return appendLine(r.convPath, c)
}

func (r *chatRepo) AppendMessage(_ context.Context, m domain.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.putMsg(m)
	return appendLine(r.msgPath, m)
}

func (r *chatRepo) RecentMessages(_ context.Context, conversationID string, limit int) ([]domain.ChatMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []domain.ChatMessage
	for _, m := range r.msgs {
		if m.ConversationID == conversationID {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

func inRange(t time.Time, from, to *time.Time) bool {
	return (from == nil || !t.Before(*from)) && (to == nil || !t.After(*to))
}

func page[T any](list []T, limit, offset int) []T {
	limit, offset = port.NormalizePage(limit, offset)
	if offset >= len(list) {
		return []T{}
	}
	end := offset + limit
	if end > len(list) {
		end = len(list)
	}
	return list[offset:end]
}

func (r *chatRepo) SearchConversations(_ context.Context, f port.ConversationFilter) ([]domain.Conversation, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var ids map[string]bool
	if f.IDs != nil {
		ids = map[string]bool{}
		for _, id := range f.IDs {
			ids[id] = true
		}
	}
	text := strings.ToLower(f.Text)
	out := []domain.Conversation{}
	for _, c := range r.convs {
		switch {
		case f.OfficeID != "" && c.OfficeID != f.OfficeID,
			f.ServiceID != "" && c.ServiceID != f.ServiceID,
			f.User != "" && c.AdminID != f.User && c.Username != f.User,
			text != "" && !strings.Contains(strings.ToLower(c.Title), text),
			ids != nil && !ids[c.ID],
			!inRange(c.CreatedAt, f.From, f.To):
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return page(out, f.Limit, f.Offset), int64(len(out)), nil
}

func (r *chatRepo) GetMessage(_ context.Context, id string) (domain.ChatMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.msgIdx[id]
	if !ok {
		return domain.ChatMessage{}, domain.ErrNotFound
	}
	return r.msgs[i], nil
}

func (r *chatRepo) SearchMessages(_ context.Context, f port.MessageFilter) ([]domain.ChatMessage, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []domain.ChatMessage{}
	for _, m := range r.msgs {
		switch {
		case f.OfficeID != "" && m.OfficeID != f.OfficeID,
			f.ServiceID != "" && m.ServiceID != f.ServiceID,
			f.Role != "" && m.Role != f.Role,
			f.Verification != "" && m.VerificationStatus != f.Verification,
			!inRange(m.CreatedAt, f.From, f.To):
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
	return page(out, f.Limit, f.Offset), int64(len(out)), nil
}

func (r *chatRepo) SetVerification(_ context.Context, messageID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	i, ok := r.msgIdx[messageID]
	if !ok {
		return domain.ErrNotFound
	}
	r.msgs[i].VerificationStatus = status
	return appendLine(r.msgPath, r.msgs[i])
}

func (r *chatRepo) CountByVerification(_ context.Context, officeID, serviceID string) (map[string]int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[string]int64{}
	for _, m := range r.msgs {
		if m.Role != "assistant" || m.VerificationStatus == "" ||
			(officeID != "" && m.OfficeID != officeID) || (serviceID != "" && m.ServiceID != serviceID) {
			continue
		}
		out[m.VerificationStatus]++
	}
	return out, nil
}

// ---- verifications / access_log (JSON Lines · บรรทัดหลังสุดของ message เดียวกันชนะ) ----

type verificationRepo struct {
	mu    sync.RWMutex
	path  string
	items map[string]domain.Verification // message_id → ผลล่าสุด
}

func NewVerificationRepository(path string) (port.VerificationRepository, error) {
	r := &verificationRepo{path: path, items: map[string]domain.Verification{}}
	err := readLines(path, func(line []byte) {
		var v domain.Verification
		if json.Unmarshal(line, &v) == nil {
			r.items[v.MessageID] = v
		}
	})
	return r, err
}

func (r *verificationRepo) Upsert(_ context.Context, v domain.Verification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if old, ok := r.items[v.MessageID]; ok {
		v.ID, v.CreatedAt = old.ID, old.CreatedAt
	}
	r.items[v.MessageID] = v
	return appendLine(r.path, v)
}

func (r *verificationRepo) GetByMessage(_ context.Context, messageID string) (domain.Verification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.items[messageID]
	if !ok {
		return v, domain.ErrNotFound
	}
	return v, nil
}

type accessLogRepo struct {
	mu   sync.Mutex
	path string
}

func NewAccessLogRepository(path string) port.AccessLogRepository { return &accessLogRepo{path: path} }

func (r *accessLogRepo) Insert(_ context.Context, e domain.AccessLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return appendLine(r.path, e)
}

// ---- ลบตามคำขอ (เขียนไฟล์ใหม่ทั้งไฟล์หลังลบ) ----

func (r *chatRepo) userConvs(s domain.DeletionScope) map[string]bool {
	ids := map[string]bool{}
	for _, c := range r.convs {
		if c.OfficeID == s.OfficeID && (s.ServiceID == "" || c.ServiceID == s.ServiceID) &&
			(s.User == "" || c.AdminID == s.User || c.Username == s.User) {
			ids[c.ID] = true
		}
	}
	return ids
}

func (r *chatRepo) msgInScope(m domain.ChatMessage, s domain.DeletionScope, convs map[string]bool) bool {
	return m.OfficeID == s.OfficeID && (s.ServiceID == "" || m.ServiceID == s.ServiceID) &&
		(s.User == "" || convs[m.ConversationID]) && inRange(m.CreatedAt, s.From, s.To)
}

func (r *chatRepo) MessageIDsInScope(_ context.Context, s domain.DeletionScope) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	convs := r.userConvs(s)
	var out []string
	for _, m := range r.msgs {
		if r.msgInScope(m, s, convs) {
			out = append(out, m.ID)
		}
	}
	return out, nil
}

func rewriteLines[T any](path string, items []T) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	for _, it := range items {
		raw, err := json.Marshal(it)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := f.Write(append(raw, '\n')); err != nil {
			f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (r *chatRepo) DeleteMessagesInScope(_ context.Context, s domain.DeletionScope) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	convs := r.userConvs(s)
	kept := r.msgs[:0:0]
	var n int64
	for _, m := range r.msgs {
		if r.msgInScope(m, s, convs) {
			n++
			continue
		}
		kept = append(kept, m)
	}
	r.msgs = kept
	r.msgIdx = map[string]int{}
	for i, m := range r.msgs {
		r.msgIdx[m.ID] = i
	}
	return n, rewriteLines(r.msgPath, r.msgs)
}

func (r *chatRepo) DeleteEmptyConversations(_ context.Context, s domain.DeletionScope) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	used := map[string]bool{}
	for _, m := range r.msgs {
		used[m.ConversationID] = true
	}
	var n int64
	for id := range r.userConvs(s) {
		if !used[id] {
			delete(r.convs, id)
			n++
		}
	}
	all := make([]domain.Conversation, 0, len(r.convs))
	for _, c := range r.convs {
		all = append(all, c)
	}
	return n, rewriteLines(r.convPath, all)
}

func (r *verificationRepo) DeleteByMessages(_ context.Context, messageIDs []string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var n int64
	for _, id := range messageIDs {
		if _, ok := r.items[id]; ok {
			delete(r.items, id)
			n++
		}
	}
	all := make([]domain.Verification, 0, len(r.items))
	for _, v := range r.items {
		all = append(all, v)
	}
	return n, rewriteLines(r.path, all)
}

func (r *accessLogRepo) List(_ context.Context, f port.AccessLogFilter) ([]domain.AccessLog, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var all []domain.AccessLog
	err := readLines(r.path, func(line []byte) {
		var e domain.AccessLog
		if json.Unmarshal(line, &e) == nil &&
			(f.OfficeID == "" || e.OfficeID == f.OfficeID) && (f.ServiceID == "" || e.ServiceID == f.ServiceID) &&
			(f.Operator == "" || e.Operator == f.Operator) {
			all = append(all, e)
		}
	})
	sort.Slice(all, func(i, j int) bool { return all[i].At.After(all[j].At) })
	return page(all, f.Limit, f.Offset), int64(len(all)), err
}
