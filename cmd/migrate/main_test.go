package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestMigrationSourceURLNormalizesRelativePath(t *testing.T) {
	sourceURL, err := migrationSourceURL("migrations")
	if err != nil {
		t.Fatalf("migrationSourceURL returned error: %v", err)
	}

	if !strings.HasPrefix(sourceURL, "file://") {
		t.Fatalf("source URL should use file scheme, got %q", sourceURL)
	}
	if strings.Contains(sourceURL, "\\") {
		t.Fatalf("source URL should use slash separators, got %q", sourceURL)
	}
}

func TestMigrationSourceURLNormalizesWindowsDrivePath(t *testing.T) {
	sourceURL, err := migrationSourceURL(`C:\workspace\omnitoken\migrations`)
	if err != nil {
		t.Fatalf("migrationSourceURL returned error: %v", err)
	}

	want := "file://C:/workspace/omnitoken/migrations"
	if sourceURL != want {
		t.Fatalf("source URL mismatch\nwant: %q\n got: %q", want, sourceURL)
	}
}

func TestMigrationSourceURLKeepsNonFileURL(t *testing.T) {
	sourceURL, err := migrationSourceURL("s3://bucket/migrations")
	if err != nil {
		t.Fatalf("migrationSourceURL returned error: %v", err)
	}
	if sourceURL != "s3://bucket/migrations" {
		t.Fatalf("unexpected source URL: %q", sourceURL)
	}
}

func TestRunCLIRequiresDatabaseURL(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI([]string{"up"}, func(string) string { return "" }, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code mismatch: got %d", code)
	}
	if !strings.Contains(stderr.String(), "missing -database-url") {
		t.Fatalf("stderr should explain missing database URL, got %q", stderr.String())
	}
}

func TestRunCLIDownRejectsNonPositiveSteps(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI(
		[]string{"-database-url", "postgres://user:pass@localhost/db?sslmode=disable", "down", "-steps", "0"},
		func(string) string { return "" },
		&stdout,
		&stderr,
	)
	if code != 2 {
		t.Fatalf("exit code mismatch: got %d", code)
	}
	if !strings.Contains(stderr.String(), "-steps must be greater than 0") {
		t.Fatalf("stderr should explain invalid steps, got %q", stderr.String())
	}
}

func TestRunCLIForceRequiresVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runCLI(
		[]string{"-database-url", "postgres://user:pass@localhost/db?sslmode=disable", "force"},
		func(string) string { return "" },
		&stdout,
		&stderr,
	)
	if code != 2 {
		t.Fatalf("exit code mismatch: got %d", code)
	}
	if !strings.Contains(stderr.String(), "force requires -version") {
		t.Fatalf("stderr should explain missing version, got %q", stderr.String())
	}
}
