package credentials

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"

	HealthHealthy     = "healthy"
	HealthDegraded    = "degraded"
	HealthQuarantined = "quarantined"
)

var (
	ErrAliasExists       = errors.New("credential alias already exists")
	ErrCredentialMissing = errors.New("credential not found")
)

type Credential struct {
	ID          string
	Provider    string
	BaseURL     string
	Secret      string
	Region      string
	Priority    int
	Weight      int
	Status      string
	HealthState string
	LastError   string
	QuotaHint   json.RawMessage
	Metadata    json.RawMessage
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateParams struct {
	Provider string
	Alias    string
	BaseURL  string
	Secret   string
	Priority int
}

type PublicCredential struct {
	ID          string          `json:"id"`
	Provider    string          `json:"provider"`
	BaseURL     string          `json:"base_url"`
	Region      string          `json:"region,omitempty"`
	Priority    int             `json:"priority"`
	Weight      int             `json:"weight"`
	Status      string          `json:"status"`
	HealthState string          `json:"health_state"`
	LastError   string          `json:"last_error,omitempty"`
	QuotaHint   json.RawMessage `json:"quota_hint,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
