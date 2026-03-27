package main

import (
	"encoding/json"
	"testing"
	"time"
)

func jsonVal(s string) json.RawMessage { return json.RawMessage(`"` + s + `"`) }

// ── LRU Cache Tests ───────────────────────────────────────────────────────────

func TestLRU_SetAndGet(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	c.Set("k1", jsonVal("v1"))
	val, ok := c.Get("k1")
	if !ok { t.Fatal("expected cache hit") }
	if string(val) != `"v1"` { t.Errorf("wrong value: %s", val) }
}

func TestLRU_MissReturnsNotFound(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	_, ok := c.Get("missing")
	if ok { t.Error("expected cache miss") }
}

func TestLRU_ExpiredEntryMisses(t *testing.T) {
	c := NewLRUCache(10, 10*time.Millisecond)
	c.Set("k1", jsonVal("v1"))
	time.Sleep(20 * time.Millisecond)
	_, ok := c.Get("k1")
	if ok { t.Error("expected expired entry to miss") }
}

func TestLRU_EvictsLRU(t *testing.T) {
	c := NewLRUCache(3, time.Minute)
	c.Set("k1", jsonVal("v1"))
	c.Set("k2", jsonVal("v2"))
	c.Set("k3", jsonVal("v3"))
	c.Get("k1") // access k1 to make k2 LRU
	c.Get("k3")
	c.Set("k4", jsonVal("v4")) // should evict k2
	_, ok := c.Get("k2")
	if ok { t.Error("expected k2 to be evicted (LRU)") }
}

func TestLRU_UpdateMovesToFront(t *testing.T) {
	c := NewLRUCache(3, time.Minute)
	c.Set("k1", jsonVal("v1"))
	c.Set("k2", jsonVal("v2"))
	c.Set("k3", jsonVal("v3"))
	c.Set("k1", jsonVal("v1-updated")) // update k1 moves to front
	c.Set("k4", jsonVal("v4"))         // evicts LRU (k2)
	_, ok := c.Get("k1")
	if !ok { t.Error("expected k1 to still be present after update") }
}

func TestLRU_Delete(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	c.Set("k1", jsonVal("v1"))
	deleted := c.Delete("k1")
	if !deleted { t.Error("expected delete to return true") }
	_, ok := c.Get("k1")
	if ok { t.Error("expected deleted key to miss") }
}

func TestLRU_DeleteNonExistent(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	deleted := c.Delete("nonexistent")
	if deleted { t.Error("expected delete of nonexistent to return false") }
}

func TestLRU_Len(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	if c.Len() != 0 { t.Error("expected empty cache") }
	c.Set("k1", jsonVal("v1"))
	c.Set("k2", jsonVal("v2"))
	if c.Len() != 2 { t.Errorf("expected 2, got %d", c.Len()) }
}

func TestLRU_HitRateMetrics(t *testing.T) {
	c := NewLRUCache(10, time.Minute)
	c.Set("k1", jsonVal("v1"))
	c.Get("k1") // hit
	c.Get("k2") // miss
	rate := c.metrics.HitRate()
	if rate != 0.5 { t.Errorf("expected 0.5 hit rate, got %f", rate) }
}

func TestLRU_CapacityZeroDoesNotPanic(t *testing.T) {
	c := NewLRUCache(1, time.Minute)
	c.Set("k1", jsonVal("v1"))
	c.Set("k2", jsonVal("v2")) // evicts k1
	if c.Len() != 1 { t.Errorf("expected 1, got %d", c.Len()) }
}

func TestLRU_ConcurrentAccess(t *testing.T) {
	c := NewLRUCache(100, time.Minute)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := string(rune('a' + n))
			c.Set(key, jsonVal("v"))
			c.Get(key)
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ { <-done }
}

// ── Cache Metrics Tests ───────────────────────────────────────────────────────

func TestCacheMetrics_HitRate_NoRequests(t *testing.T) {
	m := &CacheMetrics{}
	if m.HitRate() != 0 { t.Error("expected 0 with no requests") }
}

func TestCacheMetrics_HitRate_AllHits(t *testing.T) {
	m := &CacheMetrics{Hits: 10, Misses: 0}
	if m.HitRate() != 1.0 { t.Errorf("expected 1.0, got %f", m.HitRate()) }
}

func TestCacheMetrics_HitRate_AllMisses(t *testing.T) {
	m := &CacheMetrics{Hits: 0, Misses: 10}
	if m.HitRate() != 0.0 { t.Errorf("expected 0.0, got %f", m.HitRate()) }
}

func TestCacheMetrics_Snapshot(t *testing.T) {
	m := &CacheMetrics{Hits: 5, Misses: 3, Sets: 8}
	snap := m.snapshot()
	if snap["hits"].(int64) != 5 { t.Errorf("expected 5 hits") }
	if snap["misses"].(int64) != 3 { t.Errorf("expected 3 misses") }
}

// ── Cache Service Tests ───────────────────────────────────────────────────────

func TestCacheService_SetAndGet(t *testing.T) {
	svc := NewCacheService()
	svc.Set("key1", jsonVal("response"), "prompt")
	val, ok := svc.Get("key1")
	if !ok { t.Error("expected hit") }
	if string(val) != `"response"` { t.Errorf("wrong value: %s", val) }
}

func TestCacheService_MissMisses(t *testing.T) {
	svc := NewCacheService()
	_, ok := svc.Get("nonexistent")
	if ok { t.Error("expected miss") }
}

func TestCacheService_EmbedCache(t *testing.T) {
	svc := NewCacheService()
	svc.Set("emb:query", jsonVal("[0.1,0.2]"), "embed")
	val, ok := svc.Get("emb:query")
	if !ok { t.Error("expected embed cache hit") }
	_ = val
}

func TestCacheService_ResponseCache(t *testing.T) {
	svc := NewCacheService()
	svc.Set("resp:key", jsonVal("resp"), "response")
	_, ok := svc.Get("resp:key")
	if !ok { t.Error("expected response cache hit") }
}

func TestCacheService_Delete(t *testing.T) {
	svc := NewCacheService()
	svc.Set("del:key", jsonVal("v"), "prompt")
	svc.Delete("del:key")
	_, ok := svc.Get("del:key")
	if ok { t.Error("expected miss after delete") }
}

func TestCacheService_Stats(t *testing.T) {
	svc := NewCacheService()
	svc.Set("k1", jsonVal("v"), "prompt")
	svc.Get("k1")
	stats := svc.Stats()
	if stats == nil { t.Error("expected non-nil stats") }
	if _, ok := stats["prompt_cache"]; !ok { t.Error("expected prompt_cache in stats") }
}

func TestCacheService_PromptHitsTracked(t *testing.T) {
	svc := NewCacheService()
	svc.Set("k1", jsonVal("v"), "prompt")
	svc.Get("k1")
	svc.Get("k1")
	stats := svc.Stats()
	svcStats := stats["service"].(map[string]any)
	if svcStats["prompt_hits"].(int64) != 2 {
		t.Errorf("expected 2 prompt hits, got %v", svcStats["prompt_hits"])
	}
}

func TestMin_Helper(t *testing.T) {
	if min(3, 5) != 3 { t.Error("expected 3") }
	if min(5, 3) != 3 { t.Error("expected 3") }
	if min(4, 4) != 4 { t.Error("expected 4") }
}

func TestGetEnv_Cache(t *testing.T) {
	t.Setenv("TEST_CACHE_KEY", "val")
	if getEnv("TEST_CACHE_KEY", "fb") != "val" { t.Error("expected val") }
}
// set get
// miss
// expired
// evicts lru
// update front
// delete
// delete nonexistent
// len
// hit rate
// capacity one
// concurrent
