package service

import (
	"context"

	pkgtrace "github.com/godfreygan/ai-script/backend/pkg/trace"
)

type taskTrace struct {
	TraceID   string `json:"trace_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func newTaskTrace(ctx context.Context) taskTrace {
	traceID := pkgtrace.TraceIDFromContext(ctx)
	requestID := pkgtrace.RequestIDFromContext(ctx)
	if traceID == "" {
		traceID = requestID
	}
	if requestID == "" {
		requestID = traceID
	}
	return taskTrace{
		TraceID:   traceID,
		RequestID: requestID,
	}
}

func bindTaskTrace(ctx context.Context, meta taskTrace) context.Context {
	traceID := meta.TraceID
	requestID := meta.RequestID
	if traceID == "" {
		traceID = requestID
	}
	if requestID == "" {
		requestID = traceID
	}
	return pkgtrace.WithIDs(ctx, traceID, requestID)
}
