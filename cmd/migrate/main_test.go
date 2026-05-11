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

func TestRunCLIForceAllowsExplicitNegativeTwoVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	fake := &fakeMigrator{}

	previousOpenMigrator := openMigrator
	openMigrator = func(cliOptions) (migrator, error) {
		return fake, nil
	}
	t.Cleanup(func() {
		openMigrator = previousOpenMigrator
	})

	code := runCLI(
		[]string{"-database-url", "postgres://user:pass@localhost/db?sslmode=disable", "force", "-version", "-2"},
		func(string) string { return "" },
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("exit code mismatch: got %d stderr=%q", code, stderr.String())
	}
	if !fake.forceCalled {
		t.Fatal("expected force to be called")
	}
	if fake.forceVersion != -2 {
		t.Fatalf("force version mismatch: got %d", fake.forceVersion)
	}
	if strings.Contains(stderr.String(), "force requires -version") {
		t.Fatalf("explicit -version -2 should not be treated as missing, stderr=%q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "forced version: -2") {
		t.Fatalf("stdout should report forced version, got %q", stdout.String())
	}
}

type fakeMigrator struct {
	forceCalled  bool
	forceVersion int
}

func (f *fakeMigrator) Up() error {
	return nil
}

func (f *fakeMigrator) Steps(int) error {
	return nil
}

func (f *fakeMigrator) Version() (uint, bool, error) {
	return 0, false, nil
}

func (f *fakeMigrator) Force(version int) error {
	f.forceCalled = true
	f.forceVersion = version
	return nil
}

func (f *fakeMigrator) Close() (error, error) {
	return nil, nil
}
