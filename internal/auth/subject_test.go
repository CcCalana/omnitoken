package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestSubjectContextRoundTrip(t *testing.T) {
	subject := Subject{
		UserID:   uuid.New(),
		OrgID:    uuid.New(),
		APIKeyID: uuid.New(),
	}

	got, ok := SubjectFromContext(WithSubject(context.Background(), subject))
	if !ok {
		t.Fatal("expected subject in context")
	}
	if got != subject {
		t.Fatalf("subject mismatch: got %#v want %#v", got, subject)
	}
}

func TestSubjectFromContextMissing(t *testing.T) {
	if _, ok := SubjectFromContext(context.Background()); ok {
		t.Fatal("unexpected subject")
	}
}
