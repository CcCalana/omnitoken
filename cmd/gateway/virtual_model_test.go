package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/omnitoken/omnitoken/internal/httpx"
	"github.com/omnitoken/omnitoken/internal/router"
)

type mockResolver struct {
	mapping map[string]router.Resolution
	err     error
}

func (m *mockResolver) Resolve(ctx context.Context, requested string) (router.Resolution, error) {
	if m.err != nil {
		return router.Resolution{}, m.err
	}
	if res, ok := m.mapping[requested]; ok {
		return res, nil
	}
	return router.Resolution{RealModel: requested, IsVirtual: false}, nil
}

func TestVirtualModelMiddleware(t *testing.T) {
	resolver := &mockResolver{
		mapping: map[string]router.Resolution{
			"chat-fast":     {RealModel: "kimi-k2.6", IsVirtual: true},
			"chat-disabled": {}, // We handle disabled via error usually, let's mock the error case.
		},
	}

	tests := []struct {
		name           string
		body           string
		resolverErr    error
		expectedStatus int
		expectedModel  string
		expectedCtx    string
	}{
		{
			name:           "Gateway Integration (Rewrite Success)",
			body:           `{"model": "chat-fast", "messages": []}`,
			expectedStatus: http.StatusOK,
			expectedModel:  "kimi-k2.6",
			expectedCtx:    "chat-fast",
		},
		{
			name:           "Real Model Passthrough",
			body:           `{"model": "gpt-4", "messages": []}`,
			expectedStatus: http.StatusOK,
			expectedModel:  "gpt-4",
			expectedCtx:    "",
		},
		{
			name:           "Resolver Disabled",
			body:           `{"model": "chat-disabled", "messages": []}`,
			resolverErr:    router.ErrVirtualModelDisabled,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Resolver DB Error",
			body:           `{"model": "chat-fast", "messages": []}`,
			resolverErr:    context.DeadlineExceeded,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resolver.err = tc.resolverErr
			middleware := resolveVirtualModel(resolver)
			var capturedBody map[string]any
			var capturedCtx string

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewDecoder(r.Body).Decode(&capturedBody)
				capturedCtx = httpx.VirtualModelFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/chat", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			if tc.expectedStatus == http.StatusOK {
				if capturedBody["model"] != tc.expectedModel {
					t.Errorf("expected model %s, got %s", tc.expectedModel, capturedBody["model"])
				}
				if capturedCtx != tc.expectedCtx {
					t.Errorf("expected ctx %s, got %s", tc.expectedCtx, capturedCtx)
				}
			}
		})
	}
}
