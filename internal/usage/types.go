package usage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
)

const (
	DefaultCaptureLimit  = int64(1 << 20)
	DefaultRecordTimeout = 3 * time.Second

	ErrorCodeUsageMissing     = "usage_missing"
	ErrorCodeUsageParseFailed = "usage_parse_failed"
)

type Recorder interface {
	Record(ctx context.Context, input RecordInput) error
}

type Store interface {
	InsertUsage(ctx context.Context, record UsageRecord) error
}

type RecordInput struct {
	RequestID            string
	Subject              auth.Subject
	ModelRequested       string
	ModelRouted          string
	ModelFallback        string
	Provider             string
	UpstreamCredentialID string
	StatusCode           int
	LatencyMS            int
	Streaming            bool
	Captured             []byte
}

type TokenBreakdown struct {
	PromptTokens     int
	CompletionTokens int
	ReasoningTokens  int
	CachedTokens     int
	TotalTokens      int
}

type ParsedUsage struct {
	ModelActual string
	Tokens      TokenBreakdown
}

type UsageRecord struct {
	RequestID            string
	OrganizationID       uuid.UUID
	UserID               uuid.UUID
	APIKeyID             uuid.UUID
	UpstreamCredentialID uuid.NullUUID
	ModelRequested       string
	ModelRouted          string
	ModelActual          string
	ModelFallback        string
	Provider             string
	StatusCode           int
	ErrorCode            string
	LatencyMS            int
	Streaming            bool
	Tokens               TokenBreakdown
}
