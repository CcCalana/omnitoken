package agent_adapter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteClaudeCodeSettingsFirstWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	result, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "http://localhost:8080/v1/messages",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if result.BackupPath != "" {
		t.Fatalf("first write should not create backup, got %q", result.BackupPath)
	}
	if result.SettingsPath != filepath.Join(home, ".claude", "settings.json") {
		t.Fatalf("unexpected settings path: %q", result.SettingsPath)
	}

	env := readEnv(t, result.SettingsPath)
	for _, key := range omniTokenManagedKeys {
		if _, ok := env[key]; !ok {
			t.Fatalf("managed key %s not written", key)
		}
	}
	if env["ANTHROPIC_BASE_URL"] != "http://localhost:8080/v1/messages" {
		t.Fatalf("unexpected base URL: %q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "omt_secret" {
		t.Fatalf("unexpected token: %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["ANTHROPIC_MODEL"] != defaultClaudeCodeModel {
		t.Fatalf("unexpected default model: %q", env["ANTHROPIC_MODEL"])
	}
}

func TestWriteClaudeCodeSettingsMergesAndPreservesUserFields(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	mkdirAll(t, filepath.Dir(settingsPath))
	writeFile(t, settingsPath, `{
  "theme": "dark",
  "env": {
    "USER_KEEP": "yes",
    "ANTHROPIC_MODEL": "old-model"
  },
  "permissions": {
    "allow": ["Bash(git status)"]
  }
}`)

	result, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example/anthropic",
		Token:      "omt_new",
		Model:      "chat-code",
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("write settings: %v", err)
	}
	if result.BackupPath == "" {
		t.Fatal("expected backup for existing settings")
	}

	var root map[string]json.RawMessage
	readJSON(t, settingsPath, &root)
	if string(root["theme"]) != `"dark"` {
		t.Fatalf("theme was not preserved: %s", root["theme"])
	}
	if !strings.Contains(string(root["permissions"]), "Bash(git status)") {
		t.Fatalf("permissions were not preserved: %s", root["permissions"])
	}
	env := readEnv(t, settingsPath)
	if env["USER_KEEP"] != "yes" {
		t.Fatalf("unrelated env key was not preserved: %+v", env)
	}
	if env["ANTHROPIC_MODEL"] != "chat-code" || env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "chat-code" {
		t.Fatalf("managed model keys were not overwritten: %+v", env)
	}

	backup := readFile(t, result.BackupPath)
	if !strings.Contains(backup, `"ANTHROPIC_MODEL": "old-model"`) {
		t.Fatalf("backup did not preserve original settings:\n%s", backup)
	}
}

func TestWriteClaudeCodeSettingsRepeatCreatesUniqueBackups(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	opts := ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_one",
		Now:        fixedNow,
	}
	if _, err := WriteClaudeCodeSettings(opts); err != nil {
		t.Fatalf("first write: %v", err)
	}
	first, err := WriteClaudeCodeSettings(opts)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	second, err := WriteClaudeCodeSettings(opts)
	if err != nil {
		t.Fatalf("third write: %v", err)
	}
	if first.BackupPath == "" || second.BackupPath == "" || first.BackupPath == second.BackupPath {
		t.Fatalf("expected unique backups, got %q and %q", first.BackupPath, second.BackupPath)
	}
	if !strings.Contains(filepath.Base(first.BackupPath), "20260519T100203.000000004Z") {
		t.Fatalf("backup timestamp format did not use compact UTC layout: %q", first.BackupPath)
	}
}

func TestRestoreClaudeCodeSettingsRestoresLatestBackup(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	backupDir := claudeCodeBackupDir(home)
	mkdirAll(t, filepath.Dir(settingsPath))
	mkdirAll(t, backupDir)
	writeFile(t, settingsPath, `{"env":{"ANTHROPIC_MODEL":"current"}}`)
	older := filepath.Join(backupDir, "settings.json.20260519T100203.000000004Z.bak")
	newer := filepath.Join(backupDir, "settings.json.20260519T100204.000000004Z.bak")
	writeFile(t, older, `{"env":{"ANTHROPIC_MODEL":"older"}}`)
	writeFile(t, newer, `{"env":{"ANTHROPIC_MODEL":"newer"}}`)

	result, err := RestoreClaudeCodeSettingsWithOptions(RestoreClaudeCodeOptions{Home: home})
	if err != nil {
		t.Fatalf("restore settings: %v", err)
	}
	if result.RestoredFrom != newer {
		t.Fatalf("expected latest backup %q, got %q", newer, result.RestoredFrom)
	}
	env := readEnv(t, settingsPath)
	if env["ANTHROPIC_MODEL"] != "newer" {
		t.Fatalf("settings were not restored from latest backup: %+v", env)
	}
}

func TestRestoreClaudeCodeSettingsUsesDefaultHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	settingsPath := claudeCodeSettingsPath(home)
	backupDir := claudeCodeBackupDir(home)
	writeFile(t, settingsPath, `{"env":{"ANTHROPIC_MODEL":"current"}}`)
	writeFile(t, filepath.Join(backupDir, "settings.json.20260519T100204.000000004Z.bak"), `{"env":{"ANTHROPIC_MODEL":"default-home"}}`)

	result, err := RestoreClaudeCodeSettings()
	if err != nil {
		t.Fatalf("restore settings: %v", err)
	}
	if result.SettingsPath != settingsPath {
		t.Fatalf("expected default home settings path %q, got %q", settingsPath, result.SettingsPath)
	}
	env := readEnv(t, settingsPath)
	if env["ANTHROPIC_MODEL"] != "default-home" {
		t.Fatalf("settings were not restored via default home: %+v", env)
	}
}

func TestWriteClaudeCodeSettingsRejectsInvalidRootWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	backupDir := claudeCodeBackupDir(home)
	mkdirAll(t, filepath.Dir(settingsPath))
	writeFile(t, settingsPath, `["not-object"]`)

	_, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
	if got := readFile(t, settingsPath); got != `["not-object"]` {
		t.Fatalf("invalid settings were modified: %s", got)
	}
	if entries, readErr := os.ReadDir(backupDir); readErr == nil && len(entries) > 0 {
		t.Fatalf("backup should not be created for invalid settings")
	} else if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		t.Fatalf("read backup dir: %v", readErr)
	}
}

func TestWriteClaudeCodeSettingsRejectsInvalidEnvWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	mkdirAll(t, filepath.Dir(settingsPath))
	writeFile(t, settingsPath, `{"env":"not-object","theme":"dark"}`)

	_, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingConfig) {
		t.Fatalf("expected invalid env error, got %v", err)
	}
	if got := readFile(t, settingsPath); got != `{"env":"not-object","theme":"dark"}` {
		t.Fatalf("invalid settings were modified: %s", got)
	}
}

func TestWriteClaudeCodeSettingsRejectsNullEnv(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	writeFile(t, settingsPath, `{"env":null}`)

	_, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingConfig) {
		t.Fatalf("expected invalid env error, got %v", err)
	}
	if got := readFile(t, settingsPath); got != `{"env":null}` {
		t.Fatalf("invalid settings were modified: %s", got)
	}
}

func TestRestoreClaudeCodeSettingsReportsMissingBackup(t *testing.T) {
	t.Parallel()

	_, err := RestoreClaudeCodeSettingsWithOptions(RestoreClaudeCodeOptions{Home: t.TempDir()})
	if !errors.Is(err, errNoBackupFound) {
		t.Fatalf("expected missing backup error, got %v", err)
	}
}

func TestWriteClaudeCodeSettingsValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts ClaudeCodeOptions
		want string
	}{
		{
			name: "missing gateway",
			opts: ClaudeCodeOptions{Home: t.TempDir(), Token: "omt_secret"},
			want: "gateway url is required",
		},
		{
			name: "missing token",
			opts: ClaudeCodeOptions{Home: t.TempDir(), GatewayURL: "https://gateway.example"},
			want: "token is required",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := WriteClaudeCodeSettings(tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestManagedClaudeCodeEnvKeysReturnsCopy(t *testing.T) {
	t.Parallel()

	keys := ManagedClaudeCodeEnvKeys()
	keys[0] = "MUTATED"
	again := ManagedClaudeCodeEnvKeys()
	if again[0] == "MUTATED" {
		t.Fatal("managed keys returned mutable package slice")
	}
}

func TestWriteClaudeCodeSettingsRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	writeFile(t, settingsPath, `{"env":`)

	_, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestReadClaudeCodeSettingsReadError(t *testing.T) {
	t.Parallel()

	dirPath := t.TempDir()
	_, _, err := readClaudeCodeSettings(dirPath)
	if err == nil || !strings.Contains(err.Error(), "read Claude Code settings") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestLatestBackupPathSortsSuffixes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	base := "settings.json.20260519T100204.000000004Z.bak"
	writeFile(t, filepath.Join(dir, base), "base")
	writeFile(t, filepath.Join(dir, base+".001"), "suffix")

	latest, err := latestBackupPath(dir)
	if err != nil {
		t.Fatalf("latest backup: %v", err)
	}
	if filepath.Base(latest) != base+".001" {
		t.Fatalf("expected suffix backup to sort latest, got %q", latest)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 19, 10, 2, 3, 4, time.UTC)
}

func readEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	var root struct {
		Env map[string]string `json:"env"`
	}
	readJSON(t, path, &root)
	return root.Env
}

func readJSON(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("parse %s: %v\n%s", path, err, data)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path string, data string) {
	t.Helper()
	mkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
