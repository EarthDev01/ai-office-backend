package filestore

import (
	"context"
	"sort"
	"sync"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// llmCredentialRepo — ไฟล์ JSON (สิทธิ์ 0600) เก็บแต่ ciphertext เหมือนฝั่ง Mongo
type llmCredentialRepo struct {
	mu   sync.Mutex
	path string
}

func NewLLMCredentialRepository(path string) port.LLMCredentialRepository {
	return &llmCredentialRepo{path: path}
}

func (r *llmCredentialRepo) load() (map[string]domain.LLMCredential, error) {
	m := map[string]domain.LLMCredential{}
	return m, readJSON(r.path, &m)
}

func (r *llmCredentialRepo) List(_ context.Context) ([]domain.LLMCredential, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load()
	if err != nil {
		return nil, err
	}
	out := make([]domain.LLMCredential, 0, len(m))
	for _, c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

func (r *llmCredentialRepo) Save(_ context.Context, c domain.LLMCredential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load()
	if err != nil {
		return err
	}
	m[c.Provider] = c
	return writeJSON(r.path, m)
}

func (r *llmCredentialRepo) Delete(_ context.Context, provider string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, err := r.load()
	if err != nil {
		return err
	}
	delete(m, provider)
	return writeJSON(r.path, m)
}
