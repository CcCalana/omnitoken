package usage

import (
	"context"
	"fmt"
	"log/slog"
)

type Service struct {
	store  Store
	logger *slog.Logger
}

func NewRecorder(store Store, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store, logger: logger}
}

func (s *Service) Record(ctx context.Context, input RecordInput) error {
	if s.store == nil {
		return nil
	}

	parsed, found, parseErr := parseCaptured(input)
	record := UsageRecord{
		RequestID:      input.RequestID,
		OrganizationID: input.Subject.OrgID,
		UserID:         input.Subject.UserID,
		APIKeyID:       input.Subject.APIKeyID,
		ModelRequested: coalesce(input.ModelRequested, "unknown"),
		ModelActual:    parsed.ModelActual,
		ModelFallback:  input.ModelFallback,
		Provider:       coalesce(input.Provider, "ark"),
		StatusCode:     input.StatusCode,
		LatencyMS:      input.LatencyMS,
		Streaming:      input.Streaming,
		Tokens:         parsed.Tokens,
	}
	if parseErr != nil {
		record.ErrorCode = ErrorCodeUsageParseFailed
		s.logger.Warn("usage parse failed", "request_id", input.RequestID, "err", parseErr)
	} else if !found {
		record.ErrorCode = ErrorCodeUsageMissing
	}
	if record.ModelActual == "" {
		record.ModelActual = firstNonEmpty(input.ModelFallback, input.ModelRequested)
	}

	if err := s.store.InsertUsage(ctx, record); err != nil {
		return fmt.Errorf("insert usage: %w", err)
	}
	return nil
}

func parseCaptured(input RecordInput) (ParsedUsage, bool, error) {
	if input.Streaming {
		return ParseStream(input.Captured)
	}
	return ParseNonStream(input.Captured)
}

func coalesce(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
