package httpx

import (
	"context"
	"sync"
)

type virtualModelContextKey struct{}
type modelRoutedContextKey struct{}
type upstreamCredentialIDContextKey struct{}
type upstreamCredentialRecorderContextKey struct{}

type upstreamCredentialRecorder struct {
	mu sync.RWMutex
	id string
}

func WithVirtualModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, virtualModelContextKey{}, model)
}

func VirtualModelFromContext(ctx context.Context) string {
	model, _ := ctx.Value(virtualModelContextKey{}).(string)
	return model
}

func WithModelRouted(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelRoutedContextKey{}, model)
}

func ModelRoutedFromContext(ctx context.Context) string {
	model, _ := ctx.Value(modelRoutedContextKey{}).(string)
	return model
}

func WithUpstreamCredentialID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, upstreamCredentialIDContextKey{}, id)
}

func WithUpstreamCredentialRecorder(ctx context.Context) context.Context {
	return context.WithValue(ctx, upstreamCredentialRecorderContextKey{}, &upstreamCredentialRecorder{})
}

func SetUpstreamCredentialID(ctx context.Context, id string) {
	recorder, _ := ctx.Value(upstreamCredentialRecorderContextKey{}).(*upstreamCredentialRecorder)
	if recorder == nil {
		return
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.id = id
}

func UpstreamCredentialIDFromContext(ctx context.Context) string {
	recorder, _ := ctx.Value(upstreamCredentialRecorderContextKey{}).(*upstreamCredentialRecorder)
	if recorder != nil {
		recorder.mu.RLock()
		defer recorder.mu.RUnlock()
		return recorder.id
	}
	id, _ := ctx.Value(upstreamCredentialIDContextKey{}).(string)
	return id
}
