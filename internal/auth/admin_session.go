package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	UserID    uuid.UUID
	OrgID     uuid.UUID
	Role      string
	ExpiresAt time.Time
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]Session),
		ttl:      ttl,
	}
}

func (s *SessionStore) Create(userID, orgID uuid.UUID, role string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = Session{
		UserID:    userID,
		OrgID:     orgID,
		Role:      role,
		ExpiresAt: time.Now().UTC().Add(s.ttl),
	}
	return token, nil
}

func (s *SessionStore) Validate(token string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[token]
	if !ok {
		return Session{}, false
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		return Session{}, false
	}
	return session, true
}

func (s *SessionStore) Revoke(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}
