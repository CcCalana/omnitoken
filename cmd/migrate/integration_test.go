package main

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/lib/pq"
)

func TestModelPricingCurrentViewIntegration(t *testing.T) {
	databaseURL := os.Getenv("OMNITOKEN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set OMNITOKEN_TEST_DATABASE_URL to run SQL integration tests")
	}

	migrationsPath, err := repoMigrationPath()
	if err != nil {
		t.Fatal(err)
	}

	m, err := newMigrator(cliOptions{databaseURL: databaseURL, path: migrationsPath})
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("migrate up: %v", err)
	}
	if sourceErr, databaseErr := m.Close(); sourceErr != nil || databaseErr != nil {
		t.Fatalf("close migrator: source=%v database=%v", sourceErr, databaseErr)
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	modelID := "10000000-0000-0000-0000-000000000001"
	_, err = db.Exec(`
INSERT INTO model_catalog (id, canonical_model, provider_model, provider, created_at, updated_at)
VALUES ($1, 'integration-current-view', 'integration-current-view', 'test', now(), now())
ON CONFLICT (canonical_model, provider) DO UPDATE SET updated_at = excluded.updated_at
`, modelID)
	if err != nil {
		t.Fatalf("insert model_catalog: %v", err)
	}

	now := time.Now().UTC()
	_, err = db.Exec(`
INSERT INTO model_pricing (
  model_id,
  input_rate_usd,
  output_rate_usd,
  effective_from,
  effective_to
)
VALUES
  ($1, 0.10, 0.20, $2, $3),
  ($1, 0.30, 0.40, $3, NULL)
`, modelID, now.Add(-2*time.Hour), now.Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("insert model_pricing: %v", err)
	}

	var inputRate string
	err = db.QueryRow(`SELECT input_rate_usd::text FROM model_pricing_current WHERE model_id = $1`, modelID).Scan(&inputRate)
	if err != nil {
		t.Fatalf("query model_pricing_current: %v", err)
	}
	if inputRate != "0.3000000000" {
		t.Fatalf("current input rate mismatch: got %s", inputRate)
	}
}

func repoMigrationPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(wd, "..", "..", "migrations"))
}
