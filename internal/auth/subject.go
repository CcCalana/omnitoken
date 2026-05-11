package auth

import (
	"context"

	"github.com/google/uuid"
)

type Subject struct {
	UserID   uuid.UUID
	OrgID    uuid.UUID
	APIKeyID uuid.UUID
}

type subjectContextKey struct{}

func WithSubject(ctx context.Context, subject Subject) context.Context {
	return context.WithValue(ctx, subjectContextKey{}, subject)
}

func SubjectFromContext(ctx context.Context) (Subject, bool) {
	subject, ok := ctx.Value(subjectContextKey{}).(Subject)
	return subject, ok
}
