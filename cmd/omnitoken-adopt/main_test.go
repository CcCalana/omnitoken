package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCLIAdoptClaudeCodeUsesHomeOverride(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"adopt",
		"claude-code",
		"--gateway-url",
		"https://gateway.example",
		"--token",
		"omt_secret",
		"--home",
		home,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err != nil {
		t.Fatalf("settings file not created under home override: %v", err)
	}
	if strings.Contains(stdout.String(), "omt_secret") || strings.Contains(stderr.String(), "omt_secret") {
		t.Fatalf("CLI output leaked token stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "managed_env ANTHROPIC_BASE_URL") {
		t.Fatalf("managed env list missing from stdout: %s", stdout.String())
	}
}

func TestRunCLIInvalidExistingConfigExitsTwo(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"env":"bad"}`), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{
		"adopt",
		"claude-code",
		"--gateway-url",
		"https://gateway.example",
		"--token",
		"omt_secret",
		"--home",
		home,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if strings.Count(strings.TrimSpace(stderr.String()), "\n") != 0 {
		t.Fatalf("expected one-line stderr, got %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "panic") {
		t.Fatalf("stderr should not include panic stack: %s", stderr.String())
	}
}

func TestRunCLIRestoreClaudeCode(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	backupDir := filepath.Join(home, ".omnitoken", "backups", "claude-code")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"env":{"ANTHROPIC_MODEL":"current"}}`), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "settings.json.20260519T100204.000000004Z.bak"), []byte(`{"env":{"ANTHROPIC_MODEL":"backup"}}`), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"restore", "claude-code", "--home", home}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read restored settings: %v", err)
	}
	if !strings.Contains(string(data), `"backup"`) {
		t.Fatalf("settings were not restored: %s", data)
	}
}
