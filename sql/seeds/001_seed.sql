-- Seed 001: Default model registry, routing rules, tenants

-- ── Models ─────────────────────────────────────────────────────────────────

INSERT OR IGNORE INTO models (model_id, version, name, tier, cost_per_1k, avg_latency_ms, max_tokens, executor_addr) VALUES
    ('gpt-small',  'v1', 'GPT Small',  'small',  0.000200, 200,  4096,  'model-executor:50051'),
    ('gpt-medium', 'v1', 'GPT Medium', 'medium', 0.002000, 500,  8192,  'model-executor:50051'),
    ('gpt-large',  'v1', 'GPT Large',  'large',  0.020000, 1200, 32768, 'model-executor:50051'),
    ('embed-v2',   'v1', 'Embed v2',   'small',  0.000100, 50,   8192,  'model-executor:50051'),
    ('rerank-v1',  'v1', 'Rerank v1',  'small',  0.000200, 100,  4096,  'model-executor:50051');

-- ── Model capabilities ─────────────────────────────────────────────────────

INSERT OR IGNORE INTO model_capabilities VALUES
    ('gpt-small',  'chat'),     ('gpt-small',  'summarize'),
    ('gpt-small',  'classify'), ('gpt-small',  'moderate'),
    ('gpt-medium', 'chat'),     ('gpt-medium', 'summarize'),
    ('gpt-medium', 'classify'), ('gpt-medium', 'moderate'),
    ('gpt-large',  'chat'),     ('gpt-large',  'summarize'),
    ('embed-v2',   'embed'),
    ('rerank-v1',  'rerank');

-- ── Routing rules ──────────────────────────────────────────────────────────

INSERT OR IGNORE INTO routing_rules (rule_name, cost_budget, target_tier, priority) VALUES
    ('low_budget_small',  'low',  'small', 100),
    ('high_budget_large', 'high', 'large', 90);

INSERT OR IGNORE INTO routing_rules (rule_name, max_prompt_len, target_tier, priority) VALUES
    ('short_prompt_small', 500, 'small', 50);

INSERT OR IGNORE INTO routing_rules (rule_name, min_prompt_len, target_tier, priority) VALUES
    ('long_prompt_large', 2000, 'large', 50);

-- ── Default tenant ─────────────────────────────────────────────────────────

INSERT OR IGNORE INTO tenants (tenant_id, name, routing_mode, rate_limit_rps, burst_limit) VALUES
    ('tenant-default',  'Default',  'balanced',           50,  100),
    ('tenant-premium',  'Premium',  'latency_optimized',  200, 500),
    ('tenant-economy',  'Economy',  'cost_optimized',     20,  40);

INSERT OR IGNORE INTO quotas (tenant_id, tokens_per_minute, tokens_per_day, budget_usd_per_day) VALUES
    ('tenant-default', 100000,  5000000,  50.0),
    ('tenant-premium', 500000, 25000000, 500.0),
    ('tenant-economy',  50000,  2000000,  10.0);

INSERT OR IGNORE INTO api_keys (key_id, key_hash, tenant_id, name) VALUES
    ('key-test',     'test-key-1234',     'tenant-default', 'Test Key'),
    ('key-platform', 'platform-key-5678', 'tenant-premium', 'Platform Key'),
    ('key-economy',  'economy-key-9999',  'tenant-economy', 'Economy Key');
