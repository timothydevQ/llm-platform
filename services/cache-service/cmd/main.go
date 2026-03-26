package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ── Cache Entry ───────────────────────────────────────────────────────────────

type CacheEntry struct {
	Key       string          `json:"key"`
	Value     json.RawMessage `json:"value"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Hits      int64           `json:"hits"`
	SizeBytes int             `json:"size_bytes"`
}

func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// ── LRU Cache ─────────────────────────────────────────────────────────────────

type LRUNode struct {
	key   string
	entry *CacheEntry
	prev  *LRUNode
	next  *LRUNode
}

type LRUCache struct {
	mu       sync.RWMutex
	capacity int
	items    map[string]*LRUNode
	head     *LRUNode // most recently used
	tail     *LRUNode // least recently used
	ttl      time.Duration
	metrics  *CacheMetrics
}

func NewLRUCache(capacity int, ttl time.Duration) *LRUCache {
	head := &LRUNode{}
	tail := &LRUNode{}
	head.next = tail
	tail.prev = head
	c := &LRUCache{
		capacity: capacity,
		items:    make(map[string]*LRUNode),
		head:     head,
		tail:     tail,
		ttl:      ttl,
		metrics:  &CacheMetrics{},
	}
	go c.evictExpired()
	return c
}

func (c *LRUCache) Get(key string) (json.RawMessage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, ok := c.items[key]
	if !ok {
		atomic.AddInt64(&c.metrics.Misses, 1)
		return nil, false
	}
	if node.entry.IsExpired() {
		c.removeNode(node)
		delete(c.items, key)
		atomic.AddInt64(&c.metrics.Misses, 1)
		atomic.AddInt64(&c.metrics.Evictions, 1)
		return nil, false
	}

	// Move to front (most recently used)
	c.removeNode(node)
	c.insertFront(node)
	atomic.AddInt64(&node.entry.Hits, 1)
	atomic.AddInt64(&c.metrics.Hits, 1)
	return node.entry.Value, true
}

func (c *LRUCache) Set(key string, value json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if node, ok := c.items[key]; ok {
		node.entry.Value = value
		node.entry.ExpiresAt = time.Now().Add(c.ttl)
		c.removeNode(node)
		c.insertFront(node)
		return
	}

	if len(c.items) >= c.capacity {
		// Evict LRU
		lru := c.tail.prev
		if lru != c.head {
			c.removeNode(lru)
			delete(c.items, lru.key)
			atomic.AddInt64(&c.metrics.Evictions, 1)
		}
	}

	entry := &CacheEntry{
		Key:       key,
		Value:     value,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(c.ttl),
		SizeBytes: len(value),
	}
	node := &LRUNode{key: key, entry: entry}
	c.items[key] = node
	c.insertFront(node)
	atomic.AddInt64(&c.metrics.Sets, 1)
}

func (c *LRUCache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	node, ok := c.items[key]
	if !ok {
		return false
	}
	c.removeNode(node)
	delete(c.items, key)
	return true
}

func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

func (c *LRUCache) removeNode(node *LRUNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

func (c *LRUCache) insertFront(node *LRUNode) {
	node.next = c.head.next
	node.prev = c.head
	c.head.next.prev = node
	c.head.next = node
}

func (c *LRUCache) evictExpired() {
	for range time.NewTicker(30 * time.Second).C {
		c.mu.Lock()
		now := time.Now()
		for key, node := range c.items {
			if now.After(node.entry.ExpiresAt) {
				c.removeNode(node)
				delete(c.items, key)
				atomic.AddInt64(&c.metrics.Evictions, 1)
			}
		}
		c.mu.Unlock()
	}
}

// ── Cache Metrics ─────────────────────────────────────────────────────────────

type CacheMetrics struct {
	Hits      int64
	Misses    int64
	Sets      int64
	Evictions int64
}

func (m *CacheMetrics) HitRate() float64 {
	hits := atomic.LoadInt64(&m.Hits)
	misses := atomic.LoadInt64(&m.Misses)
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}

func (m *CacheMetrics) snapshot() map[string]any {
	return map[string]any{
		"hits":      atomic.LoadInt64(&m.Hits),
		"misses":    atomic.LoadInt64(&m.Misses),
		"sets":      atomic.LoadInt64(&m.Sets),
		"evictions": atomic.LoadInt64(&m.Evictions),
		"hit_rate":  m.HitRate(),
	}
}

// ── Cache Service ─────────────────────────────────────────────────────────────

type CacheService struct {
	promptCache   *LRUCache // exact prompt match
	responseCache *LRUCache // response for deterministic tasks
	embedCache    *LRUCache // embedding vectors
	metrics       *ServiceMetrics
}

type ServiceMetrics struct {
	PromptHits   int64
	ResponseHits int64
	EmbedHits    int64
	TotalSaved   float64 // estimated USD saved
	mu           sync.Mutex
}

func NewCacheService() *CacheService {
	return &CacheService{
		promptCache:   NewLRUCache(10000, 5*time.Minute),
		responseCache: NewLRUCache(5000, 30*time.Minute),
		embedCache:    NewLRUCache(50000, 24*time.Hour),
		metrics:       &ServiceMetrics{},
	}
}

func (s *CacheService) Get(key string) (json.RawMessage, bool) {
	// Try prompt cache first (shortest TTL)
	if val, ok := s.promptCache.Get(key); ok {
		atomic.AddInt64(&s.metrics.PromptHits, 1)
		return val, true
	}
	// Try response cache
	if val, ok := s.responseCache.Get(key); ok {
		atomic.AddInt64(&s.metrics.ResponseHits, 1)
		return val, true
	}
	// Try embed cache
	if val, ok := s.embedCache.Get(key); ok {
		atomic.AddInt64(&s.metrics.EmbedHits, 1)
		return val, true
	}
	return nil, false
}

func (s *CacheService) Set(key string, value json.RawMessage, cacheType string) {
	switch cacheType {
	case "embed":
		s.embedCache.Set(key, value)
	case "response":
		s.responseCache.Set(key, value)
	default:
		s.promptCache.Set(key, value)
	}
}

func (s *CacheService) Delete(key string) bool {
	deleted := s.promptCache.Delete(key)
	deleted = s.responseCache.Delete(key) || deleted
	deleted = s.embedCache.Delete(key) || deleted
	return deleted
}

func (s *CacheService) Stats() map[string]any {
	return map[string]any{
		"prompt_cache": map[string]any{
			"size":    s.promptCache.Len(),
			"metrics": s.promptCache.metrics.snapshot(),
		},
		"response_cache": map[string]any{
			"size":    s.responseCache.Len(),
			"metrics": s.responseCache.metrics.snapshot(),
		},
		"embed_cache": map[string]any{
			"size":    s.embedCache.Len(),
			"metrics": s.embedCache.metrics.snapshot(),
		},
		"service": map[string]any{
			"prompt_hits":   atomic.LoadInt64(&s.metrics.PromptHits),
			"response_hits": atomic.LoadInt64(&s.metrics.ResponseHits),
			"embed_hits":    atomic.LoadInt64(&s.metrics.EmbedHits),
		},
	}
}

// ── HTTP Handler ──────────────────────────────────────────────────────────────

type handler struct{ svc *CacheService }

func (h *handler) getCache(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key required"})
		return
	}
	val, ok := h.svc.Get(key)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cache miss"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(http.StatusOK)
	w.Write(val)
}

func (h *handler) setCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key       string          `json:"key"`
		Value     json.RawMessage `json:"value"`
		CacheType string          `json:"cache_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key and value required"})
		return
	}
	h.svc.Set(req.Key, req.Value, req.CacheType)
	slog.Info("Cache set", "key", req.Key[:min(len(req.Key), 32)], "type", req.CacheType)
	writeJSON(w, http.StatusOK, map[string]string{"status": "cached"})
}

func (h *handler) deleteCache(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "key required"})
		return
	}
	deleted := h.svc.Delete(key)
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": deleted})
}

func (h *handler) stats(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.Stats())
}

func (h *handler) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (h *handler) readiness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *handler) metricsHandler(w http.ResponseWriter, _ *http.Request) {
	stats := h.svc.Stats()
	pc := stats["prompt_cache"].(map[string]any)["metrics"].(map[string]any)
	fmt.Fprintf(w, "cache_prompt_hits %v\n", pc["hits"])
	fmt.Fprintf(w, "cache_prompt_misses %v\n", pc["misses"])
	fmt.Fprintf(w, "cache_prompt_hit_rate %v\n", pc["hit_rate"])
	fmt.Fprintf(w, "cache_prompt_size %v\n", stats["prompt_cache"].(map[string]any)["size"])
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b { return a }
	return b
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}

func methodHandler(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[strings.ToUpper(r.Method)]; ok {
			h(w, r)
			return
		}
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	svc := NewCacheService()
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/cache", methodHandler(map[string]http.HandlerFunc{
		"GET":    h.getCache,
		"POST":   h.setCache,
		"DELETE": h.deleteCache,
	}))
	mux.HandleFunc("/v1/stats", methodHandler(map[string]http.HandlerFunc{"GET": h.stats}))
	mux.HandleFunc("/healthz/live", h.liveness)
	mux.HandleFunc("/healthz/ready", h.readiness)
	mux.HandleFunc("/metrics", h.metricsHandler)

	port := getEnv("HTTP_PORT", "8084")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Cache Service started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
// cache entry
// is expired
// lru node
// lru cache
// lru get
// lru set
// lru evict
// lru delete
// lru helpers
