package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestRequireVirtualKey(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x22}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	record := VirtualKeyRecord{
		APIKeyID: uuid.New(),
		OrgID:    uuid.New(),
		UserID:   uuid.New(),
		KeyHash:  key.Hash,
		Status:   "active",
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if _, ok := SubjectFromContext(r.Context()); !ok {
			t.Fatal("expected subject")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireVirtualKey(fakeStore{record: record}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequireVirtualKeyAcceptsXApiKeyHeader(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x22}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	record := VirtualKeyRecord{
		APIKeyID: uuid.New(),
		OrgID:    uuid.New(),
		UserID:   uuid.New(),
		KeyHash:  key.Hash,
		Status:   "active",
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := RequireVirtualKey(fakeStore{record: record}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})(next)

	req := httptest.NewRequest(http.MethodGet, "/v1/messages", nil)
	req.Header.Set("x-api-key", key.Token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequireVirtualKeyRejectsMissingHeader(t *testing.T) {
	handler := RequireVirtualKey(fakeStore{}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRequireVirtualKeyReturnsServerErrorForStoreFailure(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x44}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	handler := RequireVirtualKey(fakeStore{err: context.Canceled}, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+key.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
