package redisx

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("redis key not found")

// Client is the cache surface used by account sessions / online presence.
type Client interface {
	Enabled() bool
	Ping(ctx context.Context) error
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
	Expire(ctx context.Context, key string, ttl time.Duration) error
	SAdd(ctx context.Context, key string, members ...string) error
	SRem(ctx context.Context, key string, members ...string) error
	SMembers(ctx context.Context, key string) ([]string, error)
	Close() error
}

type RedisClient struct {
	rdb *redis.Client
}

func Open(ctx context.Context, addr, password string, db int) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, err
	}
	return &RedisClient{rdb: rdb}, nil
}

func (c *RedisClient) Enabled() bool { return c != nil && c.rdb != nil }

func (c *RedisClient) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *RedisClient) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, raw, ttl).Err()
}

func (c *RedisClient) GetJSON(ctx context.Context, key string, dest any) error {
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dest)
}

func (c *RedisClient) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

func (c *RedisClient) SAdd(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	vals := make([]any, len(members))
	for i, m := range members {
		vals[i] = m
	}
	return c.rdb.SAdd(ctx, key, vals...).Err()
}

func (c *RedisClient) SRem(ctx context.Context, key string, members ...string) error {
	if len(members) == 0 {
		return nil
	}
	vals := make([]any, len(members))
	for i, m := range members {
		vals[i] = m
	}
	return c.rdb.SRem(ctx, key, vals...).Err()
}

func (c *RedisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *RedisClient) Close() error {
	return c.rdb.Close()
}

// MemoryClient is an in-process Redis stand-in for tests and REDIS_ENABLED=false.
type MemoryClient struct {
	mu      sync.Mutex
	values  map[string]memValue
	sets    map[string]map[string]struct{}
	enabled bool
}

type memValue struct {
	raw       []byte
	expiresAt time.Time
}

func NewMemoryClient() *MemoryClient {
	return &MemoryClient{
		values:  map[string]memValue{},
		sets:    map[string]map[string]struct{}{},
		enabled: true,
	}
}

func (m *MemoryClient) Enabled() bool { return m != nil && m.enabled }

func (m *MemoryClient) Ping(ctx context.Context) error { return nil }

func (m *MemoryClient) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	m.values[key] = memValue{raw: raw, expiresAt: exp}
	return nil
}

func (m *MemoryClient) GetJSON(ctx context.Context, key string, dest any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok || (!v.expiresAt.IsZero() && time.Now().After(v.expiresAt)) {
		delete(m.values, key)
		return ErrNotFound
	}
	return json.Unmarshal(v.raw, dest)
}

func (m *MemoryClient) Delete(ctx context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.values, key)
		delete(m.sets, key)
	}
	return nil
}

func (m *MemoryClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok {
		return nil
	}
	if ttl > 0 {
		v.expiresAt = time.Now().Add(ttl)
	} else {
		v.expiresAt = time.Time{}
	}
	m.values[key] = v
	return nil
}

func (m *MemoryClient) SAdd(ctx context.Context, key string, members ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set, ok := m.sets[key]
	if !ok {
		set = map[string]struct{}{}
		m.sets[key] = set
	}
	for _, member := range members {
		set[member] = struct{}{}
	}
	return nil
}

func (m *MemoryClient) SRem(ctx context.Context, key string, members ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	set, ok := m.sets[key]
	if !ok {
		return nil
	}
	for _, member := range members {
		delete(set, member)
	}
	return nil
}

func (m *MemoryClient) SMembers(ctx context.Context, key string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	set := m.sets[key]
	out := make([]string, 0, len(set))
	for member := range set {
		out = append(out, member)
	}
	return out, nil
}

func (m *MemoryClient) Close() error { return nil }

// NopClient reports Enabled=false; all ops are no-ops / not found.
type NopClient struct{}

func (NopClient) Enabled() bool                  { return false }
func (NopClient) Ping(ctx context.Context) error { return nil }
func (NopClient) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	return nil
}
func (NopClient) GetJSON(ctx context.Context, key string, dest any) error { return ErrNotFound }
func (NopClient) Delete(ctx context.Context, keys ...string) error        { return nil }
func (NopClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}
func (NopClient) SAdd(ctx context.Context, key string, members ...string) error { return nil }
func (NopClient) SRem(ctx context.Context, key string, members ...string) error { return nil }
func (NopClient) SMembers(ctx context.Context, key string) ([]string, error)    { return nil, nil }
func (NopClient) Close() error                                                  { return nil }
