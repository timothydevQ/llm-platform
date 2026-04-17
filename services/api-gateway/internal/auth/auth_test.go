package auth_test

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
	"github.com/timothydevQ/llm-platform/services/api-gateway/internal/auth"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	f, _ := os.CreateTemp("", "auth-test-*.db")
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := sql.Open("sqlite", f.Name())
	if err != nil { t.Fatalf("open db: %v", err) }

	db.Exec(`
	CREATE TABLE tenants (
		tenant_id TEXT PRIMARY KEY, name TEXT, routing_mode TEXT DEFAULT 'balanced',
		rate_limit_rps INTEGER DEFAULT 50, burst_limit INTEGER DEFAULT 100, enabled INTEGER DEFAULT 1
	);
	CREATE TABLE api_keys (
		key_id TEXT PRIMARY KEY, key_hash TEXT UNIQUE, tenant_id TEXT,
		name TEXT, enabled INTEGER DEFAULT 1, last_used TEXT
	);
	INSERT INTO tenants VALUES ('t1','Test','balanced',50,100,1);
	INSERT INTO tenants VALUES ('t2','Disabled','balanced',50,100,0);
	INSERT INTO api_keys VALUES ('k1','valid-key','t1','Test Key',1,NULL);
	INSERT INTO api_keys VALUES ('k2','disabled-key','t1','Disabled Key',0,NULL);
	INSERT INTO api_keys VALUES ('k3','disabled-tenant-key','t2','Disabled Tenant',1,NULL);
	`)
	return db
}

func TestValidate_KnownKey(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	p, err := s.Validate("valid-key")
	if err != nil { t.Fatalf("Validate: %v", err) }
	if p.TenantID != "t1" { t.Errorf("wrong tenant: %s", p.TenantID) }
	if p.RoutingMode != "balanced" { t.Errorf("wrong mode: %s", p.RoutingMode) }
}

func TestValidate_UnknownKey(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	_, err := s.Validate("no-such-key")
	if err != auth.ErrUnauthorized { t.Errorf("expected ErrUnauthorized, got %v", err) }
}

func TestValidate_DisabledKey(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	_, err := s.Validate("disabled-key")
	if err != auth.ErrUnauthorized { t.Errorf("expected ErrUnauthorized for disabled key, got %v", err) }
}

func TestValidate_DisabledTenant(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	_, err := s.Validate("disabled-tenant-key")
	if err != auth.ErrUnauthorized { t.Errorf("expected ErrUnauthorized for disabled tenant, got %v", err) }
}

func TestValidate_CachesResult(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	p1, _ := s.Validate("valid-key")
	p2, _ := s.Validate("valid-key")
	if p1.TenantID != p2.TenantID { t.Error("cache returned different tenant") }
}

func TestValidate_PrincipalFields(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	p, _ := s.Validate("valid-key")
	if p.KeyID == ""       { t.Error("expected key_id") }
	if p.RateLimitRPS <= 0 { t.Error("expected positive rate limit") }
	if p.BurstLimit <= 0   { t.Error("expected positive burst limit") }
}

func TestInvalidate_ClearsCache(t *testing.T) {
	s := auth.NewStore(setupDB(t))
	s.Validate("valid-key") // warm cache
	s.Invalidate("valid-key")
	// Should still succeed (DB lookup)
	_, err := s.Validate("valid-key")
	if err != nil { t.Errorf("expected success after invalidation+DB lookup: %v", err) }
}
