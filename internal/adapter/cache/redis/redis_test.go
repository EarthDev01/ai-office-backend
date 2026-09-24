package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTest(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	return NewFromClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()})), mr
}

// ลิมิตพร้อมกันต้องคงที่ข้าม replica (ใช้ Redis ตัวเดียว) — AC-16
func TestSemaphore_SharedAcrossReplicas(t *testing.T) {
	a, mr := newTest(t)
	b := NewFromClient(goredis.NewClient(&goredis.Options{Addr: mr.Addr()})) // replica ที่ 2
	ctx := context.Background()
	var releases []func()
	for i, s := range []*Store{a, b, a} {
		rel, ok, err := s.Acquire(ctx, "ai:conc:demo:K11S", 3, time.Minute)
		if err != nil || !ok {
			t.Fatalf("slot %d ต้องได้: %v", i, err)
		}
		releases = append(releases, rel)
	}
	if _, ok, _ := b.Acquire(ctx, "ai:conc:demo:K11S", 3, time.Minute); ok {
		t.Fatal("slot ที่ 4 ต้องไม่ได้ แม้มาจาก replica อื่น")
	}
	releases[0]()
	if _, ok, _ := b.Acquire(ctx, "ai:conc:demo:K11S", 3, time.Minute); !ok {
		t.Fatal("คืน slot แล้วต้องได้")
	}
}

// โปรเซสตายไม่คืน slot → หมดอายุเอง
func TestSemaphore_ExpiresIfNotReleased(t *testing.T) {
	s, _ := newTest(t)
	ctx := context.Background()
	if _, ok, _ := s.Acquire(ctx, "k", 1, 50*time.Millisecond); !ok {
		t.Fatal("ต้องได้")
	}
	time.Sleep(80 * time.Millisecond)
	if _, ok, _ := s.Acquire(ctx, "k", 1, time.Minute); !ok {
		t.Fatal("slot ที่หมดอายุต้องถูกคืนเอง")
	}
}

func TestQuotaCounter(t *testing.T) {
	s, _ := newTest(t)
	ctx := context.Background()
	if _, _, found, _ := s.Get(ctx, "q"); found {
		t.Fatal("ยังไม่มีต้อง found=false")
	}
	_ = s.Init(ctx, "q", 100, 2, time.Hour)
	_ = s.Init(ctx, "q", 999, 9, time.Hour) // init ซ้ำต้องไม่ทับ
	tok, q, err := s.Add(ctx, "q", 50, 1, time.Hour)
	if err != nil || tok != 150 || q != 3 {
		t.Fatalf("add ผิด: %d %d %v", tok, q, err)
	}
}

func TestCache(t *testing.T) {
	s, mr := newTest(t)
	ctx := context.Background()
	c := s.Cache()
	_ = c.Set(ctx, "ai:cache:x", []byte("v"), 60*time.Second)
	if b, ok, _ := c.Get(ctx, "ai:cache:x"); !ok || string(b) != "v" {
		t.Fatal("cache get ผิด")
	}
	mr.FastForward(61 * time.Second)
	if _, ok, _ := c.Get(ctx, "ai:cache:x"); ok {
		t.Fatal("เกิน 60 วิต้องหมดอายุ")
	}
}
