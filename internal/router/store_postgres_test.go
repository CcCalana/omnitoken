package router_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/omnitoken/omnitoken/internal/router"
	_ "modernc.org/sqlite"
)

func TestPostgresResolver(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE virtual_models (
			name text PRIMARY KEY,
			real_model text NOT NULL,
			provider text NOT NULL DEFAULT 'ark',
			status text DEFAULT 'active'
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO virtual_models (name, real_model, provider, status)
		VALUES
			('chat-fast', 'deepseek-v4-flash', 'deepseek', 'active'),
			('chat-disabled', 'old-model', 'ark', 'disabled')
	`)
	if err != nil {
		t.Fatalf("failed to insert test data: %v", err)
	}

	resolver := router.NewPostgresStore(db)
	ctx := context.Background()

	t.Run("Hit Active", func(t *testing.T) {
		res, err := resolver.Resolve(ctx, "chat-fast")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.IsVirtual || res.RealModel != "deepseek-v4-flash" || res.Provider != "deepseek" {
			t.Errorf("unexpected resolution: %+v", res)
		}
	})

	t.Run("Hit Disabled", func(t *testing.T) {
		_, err := resolver.Resolve(ctx, "chat-disabled")
		if err != router.ErrVirtualModelDisabled {
			t.Errorf("expected ErrVirtualModelDisabled, got: %v", err)
		}
	})

	t.Run("Miss Passthrough", func(t *testing.T) {
		res, err := resolver.Resolve(ctx, "real-model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.IsVirtual || res.RealModel != "real-model" {
			t.Errorf("unexpected resolution: %+v", res)
		}
	})

	t.Run("DB Error", func(t *testing.T) {
		db.Close() // Force DB error
		_, err := resolver.Resolve(ctx, "chat-fast")
		if err == nil {
			t.Errorf("expected error on closed db")
		}
	})
}
