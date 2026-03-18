// Package auth provides API key validation and tenant context resolution.
// Keys are stored in SQLite and cached in memory with a short TTL to avoid
// a DB round-trip on every request.
package auth

import (
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ─── Types ────────────────────────────────────────────────────────────────────

// Principal is the resolved identity for an authenticated request.
type Principal struct {
	KeyID        string
	TenantID     string
	RoutingMode  string // "latency_optimized"|"cost_optimized"|"balanced"
	RateLimitRPS int
	BurstLimit   int
}

// ─── Store ────────────────────────────────────────────────────────────────────

type Store struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]*cacheEntry // key_hash → principal
}

type cacheEntry struct {
	principal *Principal
	expiresAt time.Time
}

const cacheTTL = 60 * time.Second

func NewStore(db *sql.DB) *Store {
	s := &Store{db: db, cache: make(map[string]*cacheEntry)}
	go s.cleanup()
	return s
}

// Validate resolves an API key to a Principal. Returns ErrUnauthorized if the
// key is unknown or disabled.
func (s *Store) Validate(keyHash string) (*Principal, error) {
	// Cache lookup
	s.mu.RLock()
	entry, ok := s.cache[keyHash]
	s.mu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
		return entry.principal, nil
	}

	// DB lookup
	row := s.db.QueryRow(`
		SELECT k.key_id, k.tenant_id, t.routing_mode, t.rate_limit_rps, t.burst_limit
		FROM api_keys k
		JOIN tenants t ON t.tenant_id = k.tenant_id
		WHERE k.key_hash = ? AND k.enabled = 1 AND t.enabled = 1`, keyHash)

	p := &Principal{}
	err := row.Scan(&p.KeyID, &p.TenantID, &p.RoutingMode, &p.RateLimitRPS, &p.BurstLimit)
	if err == sql.ErrNoRows {
		return nil, ErrUnauthorized
	}
	if err != nil {
		return nil, fmt.Errorf("auth store: %w", err)
	}

	// Update last_used
	go s.db.Exec(`UPDATE api_keys SET last_used = datetime('now') WHERE key_hash = ?`, keyHash)

	// Cache
	s.mu.Lock()
	s.cache[keyHash] = &cacheEntry{principal: p, expiresAt: time.Now().Add(cacheTTL)}
	s.mu.Unlock()

	return p, nil
}

func (s *Store) cleanup() {
	for range time.NewTicker(5 * time.Minute).C {
		now := time.Now()
		s.mu.Lock()
		for k, e := range s.cache {
			if now.After(e.expiresAt) {
				delete(s.cache, k)
			}
		}
		s.mu.Unlock()
	}
}

// Invalidate removes a key from the cache (call after rotation).
func (s *Store) Invalidate(keyHash string) {
	s.mu.Lock()
	delete(s.cache, keyHash)
	s.mu.Unlock()
}

var ErrUnauthorized = fmt.Errorf("unauthorized: invalid or disabled API key")
// gw1_37
// gw1_38
// gw1_39
// gw1_40
// gw1_41
// gw1_42
// gw1_43
// gw1_44
// gw1_45
// gw1_46
// gw1_47
// gw1_48
// gw1_49
// gw1_50
// gw1_51
// gw1_52
// gw1_53
// gw1_54
// gw1_55
// gw1_56
// gw1_57
