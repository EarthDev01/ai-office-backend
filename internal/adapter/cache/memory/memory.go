// Package memory = ตัวนับ/แคชในหน่วยความจำ — ใช้ใน test และ dev ที่ไม่มี Redis เท่านั้น
//
// ██ production ห้ามใช้ (หลาย replica จะนับแยกกัน — P-14) · main บังคับ REDIS_URL เมื่อ APP_MODE=production
package memory

import (
	"context"
	"sync"
	"time"
)

type Store struct {
	mu    sync.Mutex
	sems  map[string]map[int64]time.Time
	seq   int64
	quota map[string][2]int64
	cache map[string]cacheItem
	now   func() time.Time
}

type cacheItem struct {
	val []byte
	exp time.Time
}

func New() *Store {
	return &Store{sems: map[string]map[int64]time.Time{}, quota: map[string][2]int64{}, cache: map[string]cacheItem{}, now: time.Now}
}

func (s *Store) Acquire(_ context.Context, key string, limit int, ttl time.Duration) (func(), bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	set := s.sems[key]
	if set == nil {
		set = map[int64]time.Time{}
		s.sems[key] = set
	}
	for id, exp := range set {
		if now.After(exp) {
			delete(set, id)
		}
	}
	if len(set) >= limit {
		return func() {}, false, nil
	}
	s.seq++
	id := s.seq
	set[id] = now.Add(ttl)
	return func() {
		s.mu.Lock()
		delete(set, id)
		s.mu.Unlock()
	}, true, nil
}

func (s *Store) Get(_ context.Context, key string) (int64, int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.quota[key]
	return v[0], v[1], ok, nil
}

func (s *Store) Init(_ context.Context, key string, tokens, questions int64, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.quota[key]; !ok {
		s.quota[key] = [2]int64{tokens, questions}
	}
	return nil
}

func (s *Store) Add(_ context.Context, key string, tokens, questions int64, _ time.Duration) (int64, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.quota[key]
	v[0] += tokens
	v[1] += questions
	s.quota[key] = v
	return v[0], v[1], nil
}

type CacheView struct{ s *Store }

func (s *Store) Cache() *CacheView { return &CacheView{s: s} }

func (c *CacheView) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	it, ok := c.s.cache[key]
	if !ok || c.s.now().After(it.exp) {
		return nil, false, nil
	}
	return it.val, true, nil
}

func (c *CacheView) Set(_ context.Context, key string, val []byte, ttl time.Duration) error {
	c.s.mu.Lock()
	defer c.s.mu.Unlock()
	c.s.cache[key] = cacheItem{val: val, exp: c.s.now().Add(ttl)}
	return nil
}
