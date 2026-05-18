package trace

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const (
	ContextKeyTraceID   contextKey = "trace_id"
	ContextKeyRequestID contextKey = "request_id"

	HeaderTraceID    = "X-Trace-Id"
	HeaderRequestID  = "X-Request-Id"
	HeaderTraceIDAlt = "trace_id"
	HeaderRequestAlt = "request_id"
)

func WithIDs(ctx context.Context, traceID, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if traceID != "" {
		ctx = context.WithValue(ctx, ContextKeyTraceID, traceID)
	}
	if requestID != "" {
		ctx = context.WithValue(ctx, ContextKeyRequestID, requestID)
	}
	return ctx
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ContextKeyTraceID).(string); ok {
		return v
	}
	return ""
}

func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return v
	}
	return ""
}

func IDsFromHeaders(h http.Header) (traceID, requestID string) {
	traceID = firstNonEmpty(
		h.Get(HeaderTraceID),
		h.Get(HeaderTraceIDAlt),
		h.Get(HeaderRequestID),
		h.Get(HeaderRequestAlt),
	)
	requestID = firstNonEmpty(
		h.Get(HeaderRequestID),
		h.Get(HeaderRequestAlt),
		traceID,
	)
	if traceID == "" {
		traceID = requestID
	}
	return strings.TrimSpace(traceID), strings.TrimSpace(requestID)
}

func SetHeaders(h http.Header, traceID, requestID string) {
	if h == nil {
		return
	}
	if traceID != "" {
		h.Set(HeaderTraceID, traceID)
		h.Set(HeaderTraceIDAlt, traceID)
	}
	if requestID != "" {
		h.Set(HeaderRequestID, requestID)
		h.Set(HeaderRequestAlt, requestID)
	}
}

func PropagateHeaders(ctx context.Context, h http.Header) {
	if h == nil {
		return
	}
	traceID := TraceIDFromContext(ctx)
	requestID := RequestIDFromContext(ctx)
	if traceID == "" {
		traceID = requestID
	}
	if requestID == "" {
		requestID = traceID
	}
	SetHeaders(h, traceID, requestID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
