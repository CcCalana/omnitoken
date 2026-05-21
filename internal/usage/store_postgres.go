package usage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) InsertUsage(ctx context.Context, record UsageRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin usage tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var usageEventID uuid.UUID
	err = tx.QueryRowContext(ctx, insertUsageEventSQL,
		record.RequestID,
		record.OrganizationID,
		record.UserID,
		record.APIKeyID,
		nullableUUID(record.UpstreamCredentialID),
		record.ModelRequested,
		record.ModelRouted,
		nullableText(record.ModelActual),
		record.Provider,
		record.StatusCode,
		nullableText(record.ErrorCode),
		record.LatencyMS,
		record.Streaming,
	).Scan(&usageEventID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}

	if _, err := tx.ExecContext(ctx, insertTokenBreakdownSQL,
		usageEventID,
		record.Tokens.PromptTokens,
		record.Tokens.CompletionTokens,
		record.Tokens.ReasoningTokens,
		record.Tokens.CachedTokens,
		record.Tokens.TotalTokens,
	); err != nil {
		return fmt.Errorf("insert token breakdown: %w", err)
	}

	if _, err := tx.ExecContext(ctx, insertCostLedgerSQL,
		usageEventID,
		record.Provider,
		record.ModelActual,
		record.ModelFallback,
		record.ModelRequested,
		record.Tokens.PromptTokens,
		record.Tokens.CompletionTokens,
		record.Tokens.ReasoningTokens,
		record.Tokens.CachedTokens,
		record.ErrorCode,
	); err != nil {
		return fmt.Errorf("insert cost ledger: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit usage tx: %w", err)
	}
	committed = true
	return nil
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableUUID(value uuid.NullUUID) any {
	if !value.Valid {
		return nil
	}
	return value.UUID
}

const insertUsageEventSQL = `
INSERT INTO usage_events (
  request_id,
  organization_id,
  user_id,
  api_key_id,
  upstream_credential_id,
  model_requested,
  model_routed,
  model_actual,
  provider,
  status_code,
  error_code,
  latency_ms,
  streaming
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (request_id) DO NOTHING
RETURNING id`

const insertTokenBreakdownSQL = `
INSERT INTO usage_token_breakdown (
  usage_event_id,
  prompt_tokens,
  completion_tokens,
  reasoning_tokens,
  cached_tokens,
  total_tokens
)
VALUES ($1, $2, $3, $4, $5, $6)`

const insertCostLedgerSQL = `
WITH pricing AS (
  SELECT
    model_pricing_current.input_rate_usd,
    model_pricing_current.output_rate_usd,
    model_pricing_current.reasoning_rate_usd,
    model_pricing_current.cached_input_rate_usd
  FROM model_catalog
  JOIN model_pricing_current ON model_pricing_current.model_id = model_catalog.id
  WHERE model_catalog.provider = $2
    AND (
      ($3 <> '' AND model_catalog.provider_model = $3)
      OR ($4 <> '' AND model_catalog.canonical_model = $4)
      OR ($5 <> '' AND model_catalog.canonical_model = $5)
    )
  ORDER BY
    CASE
      WHEN model_catalog.provider_model = $3 THEN 0
      WHEN model_catalog.canonical_model = $4 THEN 1
      ELSE 2
    END
  LIMIT 1
),
cost AS (
  SELECT
    COALESCE((
      SELECT
        (($6::numeric / 1000000) * input_rate_usd)
        + (($7::numeric / 1000000) * output_rate_usd)
        + (($8::numeric / 1000000) * reasoning_rate_usd)
        + (($9::numeric / 1000000) * cached_input_rate_usd)
      FROM pricing
    ), 0::numeric) AS cost_usd,
    EXISTS(SELECT 1 FROM pricing) AS has_pricing
)
INSERT INTO cost_ledger (
  usage_event_id,
  cost_usd,
  list_cost_usd,
  discount_amount_usd,
  billing_policy_version,
  settlement_status
)
SELECT
  $1,
  cost_usd,
  cost_usd,
  0,
  'demo-ready-v1',
  CASE WHEN has_pricing AND $10 = '' THEN 'settled' ELSE 'failed' END
FROM cost`
