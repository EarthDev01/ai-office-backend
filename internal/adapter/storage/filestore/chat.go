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
// ห้องแชทเขียนต่อท้ายทุกครั้งที่เปลี่ยน — ตอนโหลดบรรทัดหลังสุดของ id เดียวกันชนะ
type chatRepo struct {
	mu       sync.RWMutex
	convPath string
	msgPath  string
	convs    map[string]domain.Conversation
	msgs     []domain.ChatMessage
}

func NewChatRepository(convPath, msgPath string) (port.ChatRepository, error) {
	r := &chatRepo{convPath: convPath, msgPath: msgPath, convs: map[string]domain.Conversation{}}
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
			r.msgs = append(r.msgs, m)
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

func (r *chatRepo) TouchConversation(_ context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.convs[id]
	if !ok {
		return domain.ErrNotFound
	}
	c.UpdatedAt = at
	r.convs[id] = c
	return appendLine(r.convPath, c)
}

func (r *chatRepo) AppendMessage(_ context.Context, m domain.ChatMessage) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, m)
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

func (r *chatRepo) CountAnswersSince(_ context.Context, officeID, serviceID string, since time.Time) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var n int64
	for _, m := range r.msgs {
		if m.OfficeID == officeID && m.ServiceID == serviceID && m.Role == "assistant" && m.Status == "ok" && !m.CreatedAt.Before(since) {
			n++
		}
	}
	return n, nil
}
