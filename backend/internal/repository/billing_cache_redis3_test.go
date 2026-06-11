//go:build unit

package repository

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type redisCommandCaptureHook struct {
	mu       sync.Mutex
	commands []string
}

func (h *redisCommandCaptureHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *redisCommandCaptureHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		h.record(cmd)
		return next(ctx, cmd)
	}
}

func (h *redisCommandCaptureHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		for _, cmd := range cmds {
			h.record(cmd)
		}
		return next(ctx, cmds)
	}
}

func (h *redisCommandCaptureHook) record(cmd redis.Cmder) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.commands = append(h.commands, strings.ToLower(cmd.Name()))
}

func (h *redisCommandCaptureHook) hasCommand(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	name = strings.ToLower(name)
	for _, cmd := range h.commands {
		if cmd == name {
			return true
		}
	}
	return false
}

func newCapturedMiniRedisCache(t *testing.T) (*billingCache, *redisCommandCaptureHook) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	hook := &redisCommandCaptureHook{}
	rdb.AddHook(hook)
	return &billingCache{rdb: rdb}, hook
}

func TestBillingCacheRedis3CompatibleHashSetCommands(t *testing.T) {
	tests := []struct {
		name string
		act  func(ctx context.Context, c *billingCache) error
	}{
		{
			name: "subscription cache",
			act: func(ctx context.Context, c *billingCache) error {
				return c.SetSubscriptionCache(ctx, 1, 2, &service.SubscriptionCacheData{
					Status:       "active",
					ExpiresAt:    time.Unix(1714521600, 0),
					DailyUsage:   1,
					WeeklyUsage:  2,
					MonthlyUsage: 3,
					Version:      4,
				})
			},
		},
		{
			name: "api key rate limit cache",
			act: func(ctx context.Context, c *billingCache) error {
				return c.SetAPIKeyRateLimit(ctx, 3, &service.APIKeyRateLimitCacheData{
					Usage5h:  1,
					Usage1d:  2,
					Usage7d:  3,
					Window5h: 4,
					Window1d: 5,
					Window7d: 6,
				})
			},
		},
		{
			name: "user platform quota cache",
			act: func(ctx context.Context, c *billingCache) error {
				limit := 10.0
				window := time.Unix(1714521600, 0).UTC()
				return c.SetUserPlatformQuotaCache(ctx, 4, "openai", &service.UserPlatformQuotaCacheEntry{
					DailyUsageUSD:    1,
					WeeklyUsageUSD:   2,
					MonthlyUsageUSD:  3,
					Version:          4,
					SchemaVersion:    service.UserPlatformQuotaCacheSchemaV1,
					DailyLimitUSD:    &limit,
					DailyWindowStart: &window,
				}, time.Minute)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache, hook := newCapturedMiniRedisCache(t)
			if err := tt.act(context.Background(), cache); err != nil {
				t.Fatalf("act: %v", err)
			}
			if !hook.hasCommand("hmset") {
				t.Fatalf("expected HMSET for Redis 3 compatibility, got commands %v", hook.commands)
			}
			if hook.hasCommand("hset") {
				t.Fatalf("multi-field HSET is not Redis 3 compatible, got commands %v", hook.commands)
			}
		})
	}
}
