package middleware_test

import (
	"context"
	"log/slog"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/timothydevQ/llm-platform/services/router/internal/middleware"
)

var noopInfo = &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

func okHandler(_ context.Context, _ any) (any, error)    { return "ok", nil }
func errHandler(_ context.Context, _ any) (any, error)   { return nil, status.Errorf(codes.Internal, "err") }
func panicHandler(_ context.Context, _ any) (any, error) { panic("boom") }

// ── Recovery ──────────────────────────────────────────────────────────────────

func TestRecovery_NoPanic(t *testing.T) {
	_, err := middleware.Recovery()(context.Background(), nil, noopInfo, okHandler)
	if err != nil { t.Errorf("unexpected: %v", err) }
}

func TestRecovery_CatchesPanic(t *testing.T) {
	_, err := middleware.Recovery()(context.Background(), nil, noopInfo, panicHandler)
	if err == nil { t.Fatal("expected error from panic") }
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal { t.Errorf("expected INTERNAL, got %v", st.Code()) }
}

func TestRecovery_PropagatesError(t *testing.T) {
	_, err := middleware.Recovery()(context.Background(), nil, noopInfo, errHandler)
	if err == nil { t.Fatal("expected error to propagate") }
}

// ── RequestID ─────────────────────────────────────────────────────────────────

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var id string
	middleware.RequestID()(context.Background(), nil, noopInfo, func(ctx context.Context, _ any) (any, error) {
		id = middleware.RequestIDFromContext(ctx)
		return nil, nil
	})
	if id == "" { t.Error("expected generated request_id") }
}

func TestRequestID_ReadsFromMetadata(t *testing.T) {
	md  := metadata.Pairs("x-request-id", "req-123", "x-trace-id", "trace-456")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	var reqID, traceID string
	middleware.RequestID()(ctx, nil, noopInfo, func(c context.Context, _ any) (any, error) {
		reqID   = middleware.RequestIDFromContext(c)
		traceID = middleware.TraceIDFromContext(c)
		return nil, nil
	})
	if reqID   != "req-123"   { t.Errorf("reqID: got %q", reqID) }
	if traceID != "trace-456" { t.Errorf("traceID: got %q", traceID) }
}

func TestRequestID_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		var id string
		middleware.RequestID()(context.Background(), nil, noopInfo, func(ctx context.Context, _ any) (any, error) {
			id = middleware.RequestIDFromContext(ctx); return nil, nil
		})
		if seen[id] { t.Fatalf("duplicate ID: %s", id) }
		seen[id] = true
	}
}

// ── Metrics ───────────────────────────────────────────────────────────────────

func TestMetrics_CountsSuccess(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	middleware.Metrics(m)(context.Background(), nil, noopInfo, okHandler)
	if m.TotalCalls != 1 { t.Errorf("expected 1, got %d", m.TotalCalls) }
	if m.ErrorCalls != 0 { t.Errorf("expected 0 errors, got %d", m.ErrorCalls) }
}

func TestMetrics_CountsErrors(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	middleware.Metrics(m)(context.Background(), nil, noopInfo, errHandler)
	if m.ErrorCalls != 1 { t.Errorf("expected 1, got %d", m.ErrorCalls) }
}

func TestMetrics_AvgLatency_NoData(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	if m.AvgLatencyMs() != 0 { t.Error("expected 0") }
}

func TestMetrics_ErrorRate_NoData(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	if m.ErrorRate() != 0 { t.Error("expected 0") }
}

func TestMetrics_ErrorRate_AllErrors(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	for i := 0; i < 4; i++ {
		middleware.Metrics(m)(context.Background(), nil, noopInfo, errHandler)
	}
	if m.ErrorRate() != 1.0 { t.Errorf("expected 1.0, got %f", m.ErrorRate()) }
}

func TestMetrics_MultipleCalls(t *testing.T) {
	m := &middleware.InterceptorMetrics{}
	for i := 0; i < 5; i++ {
		middleware.Metrics(m)(context.Background(), nil, noopInfo, okHandler)
	}
	if m.TotalCalls != 5 { t.Errorf("expected 5, got %d", m.TotalCalls) }
}

// ── DeadlineCheck ─────────────────────────────────────────────────────────────

func TestDeadlineCheck_ActiveContext(t *testing.T) {
	_, err := middleware.DeadlineCheck()(context.Background(), nil, noopInfo, okHandler)
	if err != nil { t.Errorf("unexpected: %v", err) }
}

func TestDeadlineCheck_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := middleware.DeadlineCheck()(ctx, nil, noopInfo, okHandler)
	if err == nil { t.Fatal("expected error for cancelled context") }
	st, _ := status.FromError(err)
	if st.Code() != codes.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", st.Code())
	}
}

// ── Chain ─────────────────────────────────────────────────────────────────────

func TestChain_OrderPreserved(t *testing.T) {
	var log []string
	makeIC := func(name string) grpc.UnaryServerInterceptor {
		return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
			log = append(log, name+":before")
			r, e := h(ctx, req)
			log = append(log, name+":after")
			return r, e
		}
	}
	middleware.Chain(makeIC("A"), makeIC("B"))(context.Background(), nil, noopInfo, okHandler)
	want := []string{"A:before", "B:before", "B:after", "A:after"}
	for i, w := range want {
		if log[i] != w { t.Errorf("[%d] want %s got %s", i, w, log[i]) }
	}
}

// ── Logging ───────────────────────────────────────────────────────────────────

func TestLogging_DoesNotPanic(t *testing.T) {
	_, err := middleware.Logging(slog.Default())(context.Background(), nil, noopInfo, okHandler)
	if err != nil { t.Errorf("unexpected: %v", err) }
}

func TestLogging_PropagatesError(t *testing.T) {
	_, err := middleware.Logging(slog.Default())(context.Background(), nil, noopInfo, errHandler)
	if err == nil { t.Error("expected error propagation") }
}
// tw_6059_31338
// tw_6059_29637
// tw_6059_2800
// tw_6059_16228
