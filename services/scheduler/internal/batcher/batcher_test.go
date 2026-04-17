package batcher_test

import (
	"testing"

	"github.com/timothydevQ/llm-platform/services/scheduler/internal/batcher"
)

func TestDefaultConfig(t *testing.T) {
	cfg := batcher.DefaultConfig()
	if cfg.MaxBatchSize <= 0 { t.Error("expected positive max batch size") }
	if cfg.MaxWaitMs <= 0    { t.Error("expected positive max wait ms") }
	if cfg.MinWaitMs <= 0    { t.Error("expected positive min wait ms") }
	if cfg.P99SLOMs <= 0     { t.Error("expected positive p99 SLO") }
	if cfg.MinWaitMs >= cfg.MaxWaitMs {
		t.Error("expected min wait < max wait")
	}
}

func TestBatchMetrics_AvgBatchSize_NoData(t *testing.T) {
	m := &batcher.BatchMetrics{}
	if m.AvgBatchSize() != 0 { t.Error("expected 0 with no data") }
}

func TestBatchMetrics_AvgBatchSize(t *testing.T) {
	m := &batcher.BatchMetrics{}
	m.Record(4)
	m.Record(8)
	avg := m.AvgBatchSize()
	if avg != 6.0 { t.Errorf("expected 6.0, got %f", avg) }
}

func TestBatchMetrics_RequestsProcessed(t *testing.T) {
	m := &batcher.BatchMetrics{}
	m.Record(5)
	m.Record(3)
	if m.RequestsProcessed != 8 { t.Errorf("expected 8, got %d", m.RequestsProcessed) }
}

func TestBatchMetrics_BatchesDispatched(t *testing.T) {
	m := &batcher.BatchMetrics{}
	m.Record(1)
	m.Record(1)
	m.Record(1)
	if m.BatchesDispatched != 3 { t.Errorf("expected 3, got %d", m.BatchesDispatched) }
}
