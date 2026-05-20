package agent_adapter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOpenCodeSettingsFirstWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv(openCodeXDGConfigHome, "")
	result, err := WriteOpenCodeSettings(OpenCodeOptions{
		Home:       home,
		GatewayURL: "http://localhost:8080",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write OpenCode settings: %v", err)
	}
	if result.BackupPath != "" || len(result.BackupPaths) != 0 {
		t.Fatalf("first write should not create backup: %+v", result)
	}
	if result.ConfigPath != filepath.Join(home, ".config", "opencode", openCodeConfigFile) {
		t.Fatalf("unexpected config path: %q", result.ConfigPath)
	}
	root := readOpenCodeRoot(t, result.ConfigPath)
	if root["$schema"] != openCodeSchemaURL {
		t.Fatalf("schema not written: %+v", root["$schema"])
	}
	provider := openCodeProvider(t, root)
	if provider["name"] != openCodeProviderTitle || provider["npm"] != openCodeProviderNPM {
		t.Fatalf("provider metadata mismatch: %+v", provider)
	}
	options := openCodeObject(t, provider, "options")
	if options["baseURL"] != "http://localhost:8080/v1" {
		t.Fatalf("baseURL mismatch: %+v", options)
	}
	if options["apiKey"] != "omt_secret" {
		t.Fatalf("apiKey mismatch: %+v", options)
	}
	models := openCodeObject(t, provider, "models")
	if _, ok := models[defaultOpenCodeModel]; !ok || len(models) != 1 {
		t.Fatalf("expected single default model: %+v", models)
	}
}

func TestWriteOpenCodeSettingsPreservesSchemaOtherProviderAndPluralProviders(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := openCodeConfigPathForHome(home)
	writeFile(t, configPath, `{
  "$schema": "https://example.invalid/schema.json",
  "theme": "dark",
  "providers": {
    "legacy": {
      "keep": true
    }
  },
  "provider": {
    "other": {
      "name": "Other"
    },
    "omnitoken": {
      "name": "Old",
      "options": {
        "apiKey": "old"
      }
    }
  }
}`)

	result, err := WriteOpenCodeSettings(OpenCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example/",
		Token:      "omt_new",
		Model:      "chat-code",
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("write OpenCode settings: %v", err)
	}
	if len(result.BackupPaths) != 1 {
		t.Fatalf("expected one backup, got %+v", result.BackupPaths)
	}
	root := readOpenCodeRoot(t, configPath)
	if root["theme"] != "dark" {
		t.Fatalf("root user key was not preserved: %+v", root)
	}
	if _, ok := root["providers"].(map[string]any); !ok {
		t.Fatalf("plural providers user data was not preserved: %+v", root["providers"])
	}
	providers := openCodeObject(t, root, "provider")
	if _, ok := providers["other"]; !ok {
		t.Fatalf("other provider was not preserved: %+v", providers)
	}
	provider := openCodeProvider(t, root)
	options := openCodeObject(t, provider, "options")
	if options["baseURL"] != "https://gateway.example/v1" || options["apiKey"] != "omt_new" {
		t.Fatalf("managed provider was not replaced: %+v", provider)
	}
	models := openCodeObject(t, provider, "models")
	if _, ok := models["chat-code"]; !ok || len(models) != 1 {
		t.Fatalf("expected single selected model: %+v", models)
	}
}

func TestWriteOpenCodeSettingsRepeatIsIdempotentWithUniqueBackups(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	opts := OpenCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
		Now:        fixedNow,
	}
	if _, err := WriteOpenCodeSettings(opts); err != nil {
		t.Fatalf("first write: %v", err)
	}
	before := readFile(t, openCodeConfigPathForHome(home))
	second, err := WriteOpenCodeSettings(opts)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	after := readFile(t, openCodeConfigPathForHome(home))
	if before != after {
		t.Fatalf("repeat write changed content\nbefore:\n%s\nafter:\n%s", before, after)
	}
	third, err := WriteOpenCodeSettings(opts)
	if err != nil {
		t.Fatalf("third write: %v", err)
	}
	if len(second.BackupPaths) != 1 || len(third.BackupPaths) != 1 || second.BackupPath == third.BackupPath {
		t.Fatalf("expected unique backups: second=%+v third=%+v", second, third)
	}
}

func TestRestoreOpenCodeSettingsRestoresLatestBackup(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := openCodeConfigPathForHome(home)
	writeFile(t, configPath, `{"provider":{"omnitoken":{"name":"current"}}}`)
	backupDir := openCodeBackupDir(home)
	older := filepath.Join(backupDir, "opencode.json.20260519T100203.000000004Z.bak")
	newer := filepath.Join(backupDir, "opencode.json.20260519T100204.000000004Z.bak")
	writeFile(t, older, `{"provider":{"omnitoken":{"name":"older"}}}`)
	writeFile(t, newer, `{"provider":{"omnitoken":{"name":"newer"}}}`)

	result, err := RestoreOpenCodeSettingsWithOptions(RestoreOpenCodeOptions{Home: home})
	if err != nil {
		t.Fatalf("restore OpenCode settings: %v", err)
	}
	if result.RestoredFrom != newer {
		t.Fatalf("expected newest backup %q, got %q", newer, result.RestoredFrom)
	}
	provider := openCodeProvider(t, readOpenCodeRoot(t, configPath))
	if provider["name"] != "newer" {
		t.Fatalf("config was not restored from newest backup: %+v", provider)
	}
}

func TestWriteOpenCodeSettingsHomeOverrideBeatsXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv(openCodeXDGConfigHome, xdg)
	result, err := WriteOpenCodeSettings(OpenCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write OpenCode settings: %v", err)
	}
	want := filepath.Join(home, ".config", "opencode", openCodeConfigFile)
	if result.ConfigPath != want {
		t.Fatalf("--home did not override XDG_CONFIG_HOME: got %q want %q", result.ConfigPath, want)
	}
	if _, err := os.Stat(filepath.Join(xdg, "opencode", openCodeConfigFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("XDG config should not be written when --home is set: %v", err)
	}
}

func TestWriteOpenCodeSettingsUsesXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(openCodeXDGConfigHome, xdg)

	result, err := WriteOpenCodeSettings(OpenCodeOptions{
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write OpenCode settings: %v", err)
	}
	want := filepath.Join(xdg, "opencode", openCodeConfigFile)
	if result.ConfigPath != want {
		t.Fatalf("XDG config path mismatch: got %q want %q", result.ConfigPath, want)
	}
}

func TestWriteOpenCodeSettingsFallsBackToHomeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(openCodeXDGConfigHome, "")

	result, err := WriteOpenCodeSettings(OpenCodeOptions{
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write OpenCode settings: %v", err)
	}
	want := filepath.Join(home, ".config", "opencode", openCodeConfigFile)
	if result.ConfigPath != want {
		t.Fatalf("home fallback path mismatch: got %q want %q", result.ConfigPath, want)
	}
}

func TestWriteOpenCodeSettingsRejectsInvalidRootWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := openCodeConfigPathForHome(home)
	writeFile(t, configPath, `["not-object"]`)

	_, err := WriteOpenCodeSettings(OpenCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingOpenCodeConfig) {
		t.Fatalf("expected invalid OpenCode config error, got %v", err)
	}
	if got := readFile(t, configPath); got != `["not-object"]` {
		t.Fatalf("invalid config was modified: %s", got)
	}
	assertNoBackup(t, openCodeBackupDir(home))
}

func TestWriteOpenCodeSettingsRejectsInvalidProviderWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := openCodeConfigPathForHome(home)
	writeFile(t, configPath, `{"provider":"not-object","theme":"dark"}`)

	_, err := WriteOpenCodeSettings(OpenCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingOpenCodeConfig) {
		t.Fatalf("expected invalid provider error, got %v", err)
	}
	if got := readFile(t, configPath); got != `{"provider":"not-object","theme":"dark"}` {
		t.Fatalf("invalid config was modified: %s", got)
	}
	assertNoBackup(t, openCodeBackupDir(home))
}

func TestManagedOpenCodeProviderKeysReturnsCopy(t *testing.T) {
	t.Parallel()

	keys := ManagedOpenCodeProviderKeys()
	keys[0] = "mutated"
	if ManagedOpenCodeProviderKeys()[0] == "mutated" {
		t.Fatal("managed keys returned mutable package slice")
	}
}

func openCodeConfigPathForHome(home string) string {
	return filepath.Join(home, ".config", "opencode", openCodeConfigFile)
}

func readOpenCodeRoot(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read OpenCode config: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse OpenCode config: %v\n%s", err, data)
	}
	return root
}

func openCodeProvider(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	providers := openCodeObject(t, root, "provider")
	provider, ok := providers[openCodeProviderName].(map[string]any)
	if !ok || provider == nil {
		t.Fatalf("missing OpenCode provider: %+v", providers)
	}
	return provider
}

func openCodeObject(t *testing.T, root map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := root[key].(map[string]any)
	if !ok || value == nil {
		t.Fatalf("%s is not an object: %+v", key, root[key])
	}
	return value
}
