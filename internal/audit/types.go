package audit

import (
	"encoding/json"
	"net"
	"time"
)

const (
	ActorTypeBootstrap = "bootstrap"
	ActorTypeUser      = "user"
	ActorTypeSystem    = "system"

	ActionUnknownAdminWrite = "unknown_admin_write"
	ActionPanicRecovered    = "panic_recovered"
)

type Actor struct {
	ID   string
	Type string
}

type Entry struct {
	Actor        Actor
	Action       string
	ResourceType string
	ResourceID   string
	Before       any
	After        any
	IP           net.IP
	UserAgent    string
	RequestID    string
	StatusCode   int
	CreatedAt    time.Time
}

type Record struct {
	ActorID      string
	ActorType    string
	Action       string
	ResourceType string
	ResourceID   string
	Before       json.RawMessage
	After        json.RawMessage
	IP           net.IP
	UserAgent    string
	RequestID    string
	StatusCode   int
	CreatedAt    time.Time
}
