package audit

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestRecorderNoopWithoutStore(t *testing.T) {
	t.Parallel()

	if err := NewRecorder(nil, nil).Record(context.Background(), Entry{}); err != nil {
		t.Fatalf("record noop: %v", err)
	}
}

func TestRecorderSerializesSnapshots(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	recorder := NewRecorder(store, nil)
	recorder.now = func() time.Time { return time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC) }

	err := recorder.Record(context.Background(), Entry{
		Actor:        Actor{ID: "bootstrap", Type: ActorTypeBootstrap},
		Action:       "create_virtual_key",
		ResourceType: "virtual_key",
		Before:       nil,
		After:        map[string]string{"key_prefix": "abcdefghijkl"},
		StatusCode:   201,
	})
	if err != nil {
		t.Fatalf("record audit: %v", err)
	}

	got := store.record
	if got.Before != nil {
		t.Fatalf("expected nil before snapshot, got %s", string(got.Before))
	}
	var after map[string]string
	if err := json.Unmarshal(got.After, &after); err != nil {
		t.Fatalf("unmarshal after: %v", err)
	}
	if after["key_prefix"] != "abcdefghijkl" {
		t.Fatalf("unexpected after snapshot: %s", string(got.After))
	}
	if got.CreatedAt.IsZero() || got.StatusCode != 201 {
		t.Fatalf("unexpected normalized record: %+v", got)
	}
}

func TestRecorderDefaultsUnknownWrite(t *testing.T) {
	t.Parallel()

	store := &memoryStore{}
	if err := NewRecorder(store, nil).Record(context.Background(), Entry{}); err != nil {
		t.Fatalf("record audit: %v", err)
	}

	got := store.record
	if got.ActorID != "unknown" || got.ActorType != ActorTypeSystem {
		t.Fatalf("unexpected default actor: %+v", got)
	}
	if got.Action != ActionUnknownAdminWrite || got.ResourceType != "admin_write" {
		t.Fatalf("unexpected default action/resource: %+v", got)
	}
	if got.StatusCode != 200 {
		t.Fatalf("unexpected default status: %d", got.StatusCode)
	}
}

func TestRecorderReturnsStoreError(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("store failed")
	err := NewRecorder(&memoryStore{err: storeErr}, nil).Record(context.Background(), Entry{})
	if !errors.Is(err, storeErr) {
		t.Fatalf("expected store error, got %v", err)
	}
}

func TestRecorderRejectsUnserializableSnapshot(t *testing.T) {
	t.Parallel()

	err := NewRecorder(&memoryStore{}, nil).Record(context.Background(), Entry{
		Before: make(chan int),
	})
	if err == nil {
		t.Fatal("expected serialization error")
	}
}

type memoryStore struct {
	record Record
	err    error
}

func (s *memoryStore) InsertAudit(_ context.Context, record Record) error {
	s.record = record
	return s.err
}
