package usage

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
)

func TestServiceRecordWritesParsedUsage(t *testing.T) {
	store := &recordingStore{}
	service := NewRecorder(store, nil)
	subject := testSubject()
	credentialID := uuid.New()

	err := service.Record(context.Background(), RecordInput{
		RequestID:            "req-1",
		Subject:              subject,
		ModelRequested:       "client-model",
		ModelRouted:          "glm-5.1",
		ModelFallback:        "ark-code-latest",
		Provider:             "ark",
		UpstreamCredentialID: credentialID.String(),
		StatusCode:           200,
		LatencyMS:            123,
		Captured:             []byte(`{"model":"glm-5.1","usage":{"prompt_tokens":15,"completion_tokens":2,"total_tokens":17}}`),
	})
	if err != nil {
		t.Fatalf("record usage: %v", err)
	}

	got := store.records[0]
	if got.RequestID != "req-1" || got.OrganizationID != subject.OrgID || got.UserID != subject.UserID || got.APIKeyID != subject.APIKeyID {
		t.Fatalf("record identity mismatch: %#v", got)
	}
	if !got.UpstreamCredentialID.Valid || got.UpstreamCredentialID.UUID != credentialID {
		t.Fatalf("credential id mismatch: %#v", got.UpstreamCredentialID)
	}
	if got.ModelRequested != "client-model" || got.ModelRouted != "glm-5.1" || got.ModelActual != "glm-5.1" || got.ModelFallback != "ark-code-latest" {
		t.Fatalf("model mismatch: %#v", got)
	}
	if got.ErrorCode != "" || got.Tokens.TotalTokens != 17 {
		t.Fatalf("usage mismatch: %#v", got)
	}
}

func TestServiceRecordMissingUsageWritesFailedRecord(t *testing.T) {
	store := &recordingStore{}
	service := NewRecorder(store, nil)

	err := service.Record(context.Background(), RecordInput{
		RequestID:      "req-missing",
		Subject:        testSubject(),
		ModelRequested: "client-model",
		ModelFallback:  "ark-code-latest",
		Provider:       "ark",
		StatusCode:     200,
		Captured:       []byte(`{"model":"glm-5.1","choices":[]}`),
	})
	if err != nil {
		t.Fatalf("record missing usage: %v", err)
	}

	got := store.records[0]
	if got.ErrorCode != ErrorCodeUsageMissing {
		t.Fatalf("error code = %q", got.ErrorCode)
	}
	if got.Tokens != (TokenBreakdown{}) {
		t.Fatalf("expected zero tokens: %#v", got.Tokens)
	}
}

func TestServiceRecordParsesStream(t *testing.T) {
	store := &recordingStore{}
	service := NewRecorder(store, nil)

	err := service.Record(context.Background(), RecordInput{
		RequestID:      "req-stream",
		Subject:        testSubject(),
		ModelRequested: "client-model",
		ModelFallback:  "ark-code-latest",
		Provider:       "ark",
		StatusCode:     200,
		Streaming:      true,
		Captured: []byte(`data: {"model":"glm-5.1","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2}}

data: [DONE]
`),
	})
	if err != nil {
		t.Fatalf("record stream: %v", err)
	}
	got := store.records[0]
	if !got.Streaming || got.ModelActual != "glm-5.1" || got.Tokens.TotalTokens != 3 {
		t.Fatalf("stream record mismatch: %#v", got)
	}
}

func TestServiceRecordParseFailedWritesFailedRecord(t *testing.T) {
	store := &recordingStore{}
	service := NewRecorder(store, nil)

	err := service.Record(context.Background(), RecordInput{
		RequestID:      "req-bad",
		Subject:        testSubject(),
		ModelRequested: "client-model",
		ModelFallback:  "ark-code-latest",
		Provider:       "ark",
		StatusCode:     200,
		Captured:       []byte(`not-json`),
	})
	if err != nil {
		t.Fatalf("record parse failure: %v", err)
	}
	got := store.records[0]
	if got.ErrorCode != ErrorCodeUsageParseFailed {
		t.Fatalf("error code = %q", got.ErrorCode)
	}
	if got.ModelActual != "ark-code-latest" {
		t.Fatalf("model actual fallback = %q", got.ModelActual)
	}
}

func TestServiceRecordReturnsStoreError(t *testing.T) {
	storeErr := errors.New("store failed")
	service := NewRecorder(&recordingStore{err: storeErr}, nil)

	err := service.Record(context.Background(), RecordInput{
		RequestID: "req-fail",
		Subject:   testSubject(),
		Captured:  []byte(`{"usage":{"total_tokens":1}}`),
	})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected store error, got %v", err)
	}
}

func TestServiceRecordNilStoreNoops(t *testing.T) {
	if err := NewRecorder(nil, nil).Record(context.Background(), RecordInput{}); err != nil {
		t.Fatalf("nil store should noop: %v", err)
	}
}

type recordingStore struct {
	records []UsageRecord
	err     error
}

func (s *recordingStore) InsertUsage(_ context.Context, record UsageRecord) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, record)
	return nil
}

func testSubject() auth.Subject {
	return auth.Subject{
		UserID:   uuid.New(),
		OrgID:    uuid.New(),
		APIKeyID: uuid.New(),
	}
}
