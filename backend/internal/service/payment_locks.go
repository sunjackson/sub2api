package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
)

var paymentScopedKeyLocks = newPaymentScopedKeyLockRegistry()

type paymentScopedKeyLockRegistry struct {
	mu    sync.Mutex
	locks map[string]*paymentScopedKeyLockEntry
}

type paymentScopedKeyLockEntry struct {
	mu   sync.Mutex
	refs int
}

func newPaymentScopedKeyLockRegistry() *paymentScopedKeyLockRegistry {
	return &paymentScopedKeyLockRegistry{locks: make(map[string]*paymentScopedKeyLockEntry)}
}

func (r *paymentScopedKeyLockRegistry) lock(keys ...string) func() {
	normalized := normalizePaymentLockKeys(keys...)
	if len(normalized) == 0 {
		return func() {}
	}

	entries := make([]*paymentScopedKeyLockEntry, 0, len(normalized))
	r.mu.Lock()
	for _, key := range normalized {
		entry := r.locks[key]
		if entry == nil {
			entry = &paymentScopedKeyLockEntry{}
			r.locks[key] = entry
		}
		entry.refs++
		entries = append(entries, entry)
	}
	r.mu.Unlock()

	for _, entry := range entries {
		entry.mu.Lock()
	}

	return func() {
		for i := len(entries) - 1; i >= 0; i-- {
			entries[i].mu.Unlock()
		}

		r.mu.Lock()
		defer r.mu.Unlock()
		for idx, key := range normalized {
			entry := entries[idx]
			entry.refs--
			if entry.refs == 0 {
				delete(r.locks, key)
			}
		}
	}
}

func normalizePaymentLockKeys(keys ...string) []string {
	if len(keys) == 0 {
		return nil
	}
	deduped := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		deduped[trimmed] = struct{}{}
	}
	if len(deduped) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(deduped))
	for key := range deduped {
		normalized = append(normalized, key)
	}
	sort.Strings(normalized)
	return normalized
}

func paymentAdvisoryLockHash(key string) int64 {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(key))
	return int64(hasher.Sum64())
}

func lockPaymentScopedKeys(ctx context.Context, client *dbent.Client, keys ...string) (func(), error) {
	release := paymentScopedKeyLocks.lock(keys...)
	normalized := normalizePaymentLockKeys(keys...)
	if len(normalized) == 0 || client == nil || client.Driver().Dialect() != dialect.Postgres {
		return release, nil
	}

	for _, key := range normalized {
		var rows entsql.Rows
		if err := client.Driver().Query(ctx, "SELECT pg_advisory_xact_lock($1)", []any{paymentAdvisoryLockHash(key)}, &rows); err != nil {
			release()
			return nil, err
		}
		_ = rows.Close()
	}
	return release, nil
}

func paymentOrderUserLockKey(userID int64) string {
	return fmt.Sprintf("payment_order:create:user:%d", userID)
}
