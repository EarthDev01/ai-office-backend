// Package redis = ตัวนับ/ลิมิต/แคชที่ทุก replica เห็นตรงกัน (P-14 · spec §7.5 Redis)
//
//	ai:conc:<office>:<service>                     semaphore (sorted set · สมาชิกหมดอายุเองถ้าโปรเซสตาย)
//	ai:quota:<office>:<service>:<YYYY-MM>          hash {tokens, questions}
//	ai:cache:<office>:<service>:<tool>:<call>:<h>  ผล tool ≤ 60 วิ
package redis

import (
	"context"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"ai-office-backend/internal/core/domain"
)

type Store struct {
	rdb *goredis.Client
}

func New(url string) (*Store, error) {
	opt, err := goredis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	rdb := goredis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &Store{rdb: rdb}, nil
}

// NewFromClient ใช้ใน test (miniredis)
func NewFromClient(c *goredis.Client) *Store { return &Store{rdb: c} }

func (s *Store) Close() error { return s.rdb.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.rdb.Ping(ctx).Err() }

// ---- semaphore ----

var acquireScript = goredis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) < tonumber(ARGV[3]) then
  redis.call('ZADD', KEYS[1], ARGV[2], ARGV[4])
  redis.call('PEXPIRE', KEYS[1], ARGV[5])
  return 1
end
return 0
`)

func (s *Store) Acquire(ctx context.Context, key string, limit int, ttl time.Duration) (func(), bool, error) {
	now := time.Now()
	member := domain.NewID()
	res, err := acquireScript.Run(ctx, s.rdb, []string{key},
		now.UnixMilli(), now.Add(ttl).UnixMilli(), limit, member, ttl.Milliseconds()).Int()
	if err != nil {
		return nil, false, err
	}
	if res != 1 {
		return func() {}, false, nil
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		s.rdb.ZRem(ctx, key, member)
	}, true, nil
}

// ---- quota counter ----

func (s *Store) Get(ctx context.Context, key string) (int64, int64, bool, error) {
	vals, err := s.rdb.HMGet(ctx, key, "tokens", "questions").Result()
	if err != nil {
		return 0, 0, false, err
	}
	if vals[0] == nil && vals[1] == nil {
		return 0, 0, false, nil
	}
	return toInt(vals[0]), toInt(vals[1]), true, nil
}

func (s *Store) Init(ctx context.Context, key string, tokens, questions int64, ttl time.Duration) error {
	pipe := s.rdb.TxPipeline()
	pipe.HSetNX(ctx, key, "tokens", tokens)
	pipe.HSetNX(ctx, key, "questions", questions)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Add(ctx context.Context, key string, tokens, questions int64, ttl time.Duration) (int64, int64, error) {
	pipe := s.rdb.TxPipeline()
	t := pipe.HIncrBy(ctx, key, "tokens", tokens)
	q := pipe.HIncrBy(ctx, key, "questions", questions)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, err
	}
	return t.Val(), q.Val(), nil
}

func toInt(v any) int64 {
	switch x := v.(type) {
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	case int64:
		return x
	}
	return 0
}

// ---- cache ----

// CacheView แยก method ของแคช (Get/Set ชื่อชนกับ quota counter)
type CacheView struct{ s *Store }

func (s *Store) Cache() *CacheView { return &CacheView{s: s} }

func (c *CacheView) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.s.rdb.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (c *CacheView) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	return c.s.rdb.Set(ctx, key, val, ttl).Err()
}
