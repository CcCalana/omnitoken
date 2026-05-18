package audit

import (
	"context"
	"sync"
)

type scopeContextKey struct{}

type Scope struct {
	mu           sync.Mutex
	action       string
	resourceType string
	resourceID   string
	before       any
	after        any
}

func WithScope(ctx context.Context) (context.Context, *Scope) {
	scope := &Scope{}
	return context.WithValue(ctx, scopeContextKey{}, scope), scope
}

func ScopeFromContext(ctx context.Context) (*Scope, bool) {
	scope, ok := ctx.Value(scopeContextKey{}).(*Scope)
	return scope, ok
}

func SetAction(ctx context.Context, action string) {
	if scope, ok := ScopeFromContext(ctx); ok {
		scope.mu.Lock()
		scope.action = action
		scope.mu.Unlock()
	}
}

func SetResource(ctx context.Context, resourceType string, resourceID string) {
	if scope, ok := ScopeFromContext(ctx); ok {
		scope.mu.Lock()
		scope.resourceType = resourceType
		scope.resourceID = resourceID
		scope.mu.Unlock()
	}
}

func SetBefore(ctx context.Context, snapshot any) {
	if scope, ok := ScopeFromContext(ctx); ok {
		scope.mu.Lock()
		scope.before = snapshot
		scope.mu.Unlock()
	}
}

func SetAfter(ctx context.Context, snapshot any) {
	if scope, ok := ScopeFromContext(ctx); ok {
		scope.mu.Lock()
		scope.after = snapshot
		scope.mu.Unlock()
	}
}

func (s *Scope) snapshot() (string, string, string, any, any) {
	if s == nil {
		return "", "", "", nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.action, s.resourceType, s.resourceID, s.before, s.after
}
