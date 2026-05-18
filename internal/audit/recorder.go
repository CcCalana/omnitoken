package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Store interface {
	InsertAudit(context.Context, Record) error
}

type Recorder interface {
	Record(context.Context, Entry) error
}

type Service struct {
	store  Store
	logger *slog.Logger
	now    func() time.Time
}

func NewRecorder(store Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store, logger: logger, now: time.Now}
}

func (s *Service) Record(ctx context.Context, entry Entry) error {
	if s == nil || s.store == nil {
		return nil
	}

	record, err := normalizeEntry(entry, s.now)
	if err != nil {
		return err
	}
	if err := s.store.InsertAudit(ctx, record); err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

func normalizeEntry(entry Entry, now func() time.Time) (Record, error) {
	if now == nil {
		now = time.Now
	}
	createdAt := entry.CreatedAt
	if createdAt.IsZero() {
		createdAt = now().UTC()
	}
	statusCode := entry.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}

	before, err := marshalSnapshot(entry.Before)
	if err != nil {
		return Record{}, fmt.Errorf("marshal before audit snapshot: %w", err)
	}
	after, err := marshalSnapshot(entry.After)
	if err != nil {
		return Record{}, fmt.Errorf("marshal after audit snapshot: %w", err)
	}

	return Record{
		ActorID:      defaultString(entry.Actor.ID, "unknown"),
		ActorType:    defaultString(entry.Actor.Type, ActorTypeSystem),
		Action:       defaultString(strings.TrimSpace(entry.Action), ActionUnknownAdminWrite),
		ResourceType: defaultString(strings.TrimSpace(entry.ResourceType), "admin_write"),
		ResourceID:   strings.TrimSpace(entry.ResourceID),
		Before:       before,
		After:        after,
		IP:           entry.IP,
		UserAgent:    strings.TrimSpace(entry.UserAgent),
		RequestID:    strings.TrimSpace(entry.RequestID),
		StatusCode:   statusCode,
		CreatedAt:    createdAt,
	}, nil
}

func marshalSnapshot(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func defaultString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
