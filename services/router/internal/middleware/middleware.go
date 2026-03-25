// Package middleware provides gRPC server interceptors for the router service.
//
// Interceptors are composable. Wire them at server construction time:
//
//	grpc.NewServer(
//	    grpc.ChainUnaryInterceptor(
//	        middleware.Recovery(),
//	        middleware.RequestID(),
//	        middleware.Logging(log),
//	        middleware.Metrics(&m),
//	    ),
//	)
//
// Order matters: Recovery wraps everything so panics are always caught.
// RequestID runs before Logging so the ID is available in log entries.
package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ── Context keys ──────────────────────────────────────────────────────────────

type ctxKey string

const (
	ctxRequestID ctxKey = "request_id"
	ctxTraceID   ctxKey = "trace_id"
)

// RequestIDFromContext extracts the request ID injected by RequestID().
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxRequestID).(string); ok {
		return v
	}
	return ""
}

// TraceIDFromContext extracts the trace ID injected by RequestID().
func TraceIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(ctxTraceID).(string); ok {
		return v
	}
	return ""
}

// ── InterceptorMetrics ────────────────────────────────────────────────────────

// InterceptorMetrics is updated by the Metrics interceptor on every call.
// Values are atomic so the router's HTTP admin endpoint can read them safely.
type InterceptorMetrics struct {
	TotalCalls    int64
	ErrorCalls    int64
	TotalLatencyMs int64 // sum for avg calculation
}

func (m *InterceptorMetrics) RecordCall(latencyMs int64, err error) {
	atomic.AddInt64(&m.TotalCalls, 1)
	atomic.AddInt64(&m.TotalLatencyMs, latencyMs)
	if err != nil {
		atomic.AddInt64(&m.ErrorCalls, 1)
	}
}

func (m *InterceptorMetrics) AvgLatencyMs() float64 {
	calls := atomic.LoadInt64(&m.TotalCalls)
	if calls == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.TotalLatencyMs)) / float64(calls)
}

func (m *InterceptorMetrics) ErrorRate() float64 {
	calls := atomic.LoadInt64(&m.TotalCalls)
	if calls == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&m.ErrorCalls)) / float64(calls)
}

// ── Recovery ──────────────────────────────────────────────────────────────────

// Recovery catches panics in any downstream handler and converts them to
// gRPC INTERNAL status errors so the server stays alive.
func Recovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in gRPC handler",
					"method", info.FullMethod,
					"panic", fmt.Sprintf("%v", r),
					"stack", string(debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal server error: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// ── RequestID ─────────────────────────────────────────────────────────────────

// RequestID reads x-request-id and x-trace-id from incoming gRPC metadata
// (set by the API gateway). If absent, generates new values. Injects both
// into the context so downstream handlers can log them.
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := mdValue(ctx, "x-request-id")
		if requestID == "" {
			requestID = newID()
		}
		traceID := mdValue(ctx, "x-trace-id")
		if traceID == "" {
			traceID = newID()
		}

		ctx = context.WithValue(ctx, ctxRequestID, requestID)
		ctx = context.WithValue(ctx, ctxTraceID, traceID)

		// Propagate IDs downstream in outgoing metadata
		ctx = metadata.AppendToOutgoingContext(ctx,
			"x-request-id", requestID,
			"x-trace-id", traceID,
		)
		return handler(ctx, req)
	}
}

// ── Logging ───────────────────────────────────────────────────────────────────

// Logging emits one structured log line per gRPC call with method, latency,
// status code, request_id, and trace_id.
func Logging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		code := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				code = st.Code()
			} else {
				code = codes.Internal
			}
		}
		log.Info("grpc",
			"method",     info.FullMethod,
			"code",       code.String(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", RequestIDFromContext(ctx),
			"trace_id",   TraceIDFromContext(ctx),
		)
		return resp, err
	}
}

// ── Metrics ───────────────────────────────────────────────────────────────────

// Metrics records per-call latency and error rate into an InterceptorMetrics
// struct that the HTTP admin /metrics endpoint can read.
func Metrics(m *InterceptorMetrics) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		m.RecordCall(time.Since(start).Milliseconds(), err)
		return resp, err
	}
}

// ── Deadline enforcement ──────────────────────────────────────────────────────

// DeadlineCheck rejects requests whose context is already expired before the
// handler even starts. This catches requests that spent too long in the
// scheduler queue.
func DeadlineCheck() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		select {
		case <-ctx.Done():
			return nil, status.Errorf(codes.DeadlineExceeded,
				"request deadline already expired before reaching %s", info.FullMethod)
		default:
			return handler(ctx, req)
		}
	}
}

// ── Chain helper ──────────────────────────────────────────────────────────────

// Chain composes multiple unary interceptors left-to-right. The first
// interceptor in the list is the outermost wrapper.
// Use grpc.ChainUnaryInterceptor instead — this is retained for reference.
func Chain(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		build := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			next := build
			ic := interceptors[i]
			build = func(ctx context.Context, req any) (any, error) {
				return ic(ctx, req, info, next)
			}
		}
		return build(ctx, req)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func mdValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}
// tw_6059_20271
// tw_6059_17881
// tw_6059_4965
// tw_6059_5676
// tw_6059_22898
