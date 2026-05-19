package auth_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/omnitoken/omnitoken/internal/auth"
)

func TestSessionStore(t *testing.T) {
	store := auth.NewSessionStore(100 * time.Millisecond)
	userID := uuid.New()
	orgID := uuid.New()

	token, err := store.Create(userID, orgID, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	session, ok := store.Validate(token)
	if !ok {
		t.Fatalf("Validate failed for valid token")
	}
	if session.UserID != userID || session.OrgID != orgID {
		t.Errorf("Validate returned wrong session data")
	}

	store.Revoke(token)
	_, ok = store.Validate(token)
	if ok {
		t.Errorf("Validate succeeded after revoke")
	}

	token2, _ := store.Create(userID, orgID, "admin")
	time.Sleep(150 * time.Millisecond)
	_, ok = store.Validate(token2)
	if ok {
		t.Errorf("Validate succeeded after expiration")
	}
}
