package agent_adapter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCodexSettingsFirstWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	result, err := WriteCodexSettings(CodexOptions{
		Home:       home,
		GatewayURL: "http://localhost:8080",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write Codex settings: %v", err)
	}
	if result.BackupPath != "" || len(result.BackupPaths) != 0 {
		t.Fatalf("first write should not create backup: %+v", result)
	}
	config := readFile(t, result.ConfigPath)
	for _, want := range []string{
		`model = "chat-balanced"`,
		`model_provider = "omnitoken"`,
		`preferred_auth_method = "apikey"`,
		`cli_auth_credentials_store = "file"`,
		codexProviderHeader,
		`base_url = "http://localhost:8080/v1"`,
		`env_key = "OPENAI_API_KEY"`,
		`wire_api = "chat"`,
		`requires_openai_auth = true`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
	if got := readCodexAPIKey(t, result.AuthPath); got != "omt_secret" {
		t.Fatalf("auth token mismatch: %q", got)
	}
}

func TestWriteCodexSettingsPreservesTomlCommentsAndOtherFields(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := codexConfigPath(home)
	writeFile(t, configPath, `# user note
model = "old"
approval_policy = "never"

[profiles.work]
model = "keep-profile-model"

[model_providers.other]
base_url = "https://other.example/v1"
`)
	authPath := codexAuthPath(home)
	writeFile(t, authPath, `{
  "OPENAI_API_KEY": "old",
  "refresh_token": "keep"
}`)

	result, err := WriteCodexSettings(CodexOptions{
		Home:       home,
		GatewayURL: "https://gateway.example/",
		Token:      "omt_new",
		Model:      "chat-code",
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("write Codex settings: %v", err)
	}
	if len(result.BackupPaths) != 2 {
		t.Fatalf("expected config and auth backups, got %+v", result.BackupPaths)
	}
	config := readFile(t, configPath)
	for _, want := range []string{
		"# user note",
		`approval_policy = "never"`,
		`[profiles.work]`,
		`model = "keep-profile-model"`,
		`[model_providers.other]`,
		`base_url = "https://other.example/v1"`,
		`model = "chat-code"`,
		`base_url = "https://gateway.example/v1"`,
	} {
		if !strings.Contains(config, want) {
			t.Fatalf("config did not preserve/write %q:\n%s", want, config)
		}
	}
	auth := readCodexAuth(t, authPath)
	if auth["refresh_token"] != "keep" {
		t.Fatalf("auth unrelated key not preserved: %+v", auth)
	}
	if auth[codexOpenAIAPIKey] != "omt_new" {
		t.Fatalf("managed auth key not overwritten: %+v", auth)
	}
}

func TestWriteCodexSettingsRepeatIsIdempotentWithUniqueBackups(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	opts := CodexOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
		Now:        fixedNow,
	}
	if _, err := WriteCodexSettings(opts); err != nil {
		t.Fatalf("first write: %v", err)
	}
	before := readFile(t, codexConfigPath(home)) + readFile(t, codexAuthPath(home))
	second, err := WriteCodexSettings(opts)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}
	after := readFile(t, codexConfigPath(home)) + readFile(t, codexAuthPath(home))
	if before != after {
		t.Fatalf("repeat write changed content\nbefore:\n%s\nafter:\n%s", before, after)
	}
	third, err := WriteCodexSettings(opts)
	if err != nil {
		t.Fatalf("third write: %v", err)
	}
	if len(second.BackupPaths) != 2 || len(third.BackupPaths) != 2 {
		t.Fatalf("expected two backups per repeat write: second=%+v third=%+v", second.BackupPaths, third.BackupPaths)
	}
	if second.BackupPaths[0] == third.BackupPaths[0] || second.BackupPaths[1] == third.BackupPaths[1] {
		t.Fatalf("expected unique backup paths: second=%+v third=%+v", second.BackupPaths, third.BackupPaths)
	}
}

func TestRestoreCodexSettingsRestoresConfigAndAuth(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	writeFile(t, codexConfigPath(home), `model = "current"`)
	writeFile(t, codexAuthPath(home), `{"OPENAI_API_KEY":"current"}`)
	backupDir := codexBackupDir(home)
	writeFile(t, filepath.Join(backupDir, "config.toml.20260519T100203.000000004Z.bak"), `model = "older"`)
	writeFile(t, filepath.Join(backupDir, "config.toml.20260519T100204.000000004Z.bak"), `model = "newer"`)
	writeFile(t, filepath.Join(backupDir, "auth.json.20260519T100204.000000004Z.bak"), `{"OPENAI_API_KEY":"backup"}`)

	result, err := RestoreCodexSettingsWithOptions(RestoreCodexOptions{Home: home})
	if err != nil {
		t.Fatalf("restore Codex settings: %v", err)
	}
	if len(result.RestoredFromPaths) != 2 {
		t.Fatalf("expected two restored backups, got %+v", result.RestoredFromPaths)
	}
	if !strings.Contains(readFile(t, codexConfigPath(home)), `"newer"`) {
		t.Fatalf("config was not restored from newest backup")
	}
	if got := readCodexAPIKey(t, codexAuthPath(home)); got != "backup" {
		t.Fatalf("auth was not restored from backup: %q", got)
	}
}

func TestWriteCodexSettingsHomeOverride(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	result, err := WriteCodexSettings(CodexOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if err != nil {
		t.Fatalf("write Codex settings: %v", err)
	}
	if result.ConfigPath != filepath.Join(home, ".codex", "config.toml") {
		t.Fatalf("config path did not use home override: %q", result.ConfigPath)
	}
	if result.AuthPath != filepath.Join(home, ".codex", "auth.json") {
		t.Fatalf("auth path did not use home override: %q", result.AuthPath)
	}
}

func TestWriteCodexSettingsRejectsInvalidExistingConfigWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := codexConfigPath(home)
	authPath := codexAuthPath(home)
	writeFile(t, configPath, `model = "old`)
	writeFile(t, authPath, `{"OPENAI_API_KEY":"old"}`)

	_, err := WriteCodexSettings(CodexOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingCodexConfig) {
		t.Fatalf("expected invalid Codex config error, got %v", err)
	}
	if got := readFile(t, configPath); got != `model = "old` {
		t.Fatalf("invalid config was modified: %s", got)
	}
	if got := readCodexAPIKey(t, authPath); got != "old" {
		t.Fatalf("auth changed after invalid config: %q", got)
	}
	assertNoBackup(t, codexBackupDir(home))
}

func TestWriteCodexSettingsRejectsInvalidAuthWithoutBackupOrWrite(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := codexConfigPath(home)
	authPath := codexAuthPath(home)
	writeFile(t, configPath, `model = "old"`)
	writeFile(t, authPath, `["not-object"]`)

	_, err := WriteCodexSettings(CodexOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	})
	if !errors.Is(err, ErrInvalidExistingCodexConfig) {
		t.Fatalf("expected invalid Codex auth error, got %v", err)
	}
	if got := readFile(t, configPath); got != `model = "old"` {
		t.Fatalf("config changed after invalid auth: %s", got)
	}
	if got := readFile(t, authPath); got != `["not-object"]` {
		t.Fatalf("invalid auth was modified: %s", got)
	}
	assertNoBackup(t, codexBackupDir(home))
}

func TestPatchCodexConfigScannerEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("managed key inside other table is preserved", func(t *testing.T) {
		t.Parallel()
		out, _, err := patchCodexConfig("[foo]\nmodel = \"x\"\n", buildCodexManagedConfig(CodexOptions{
			GatewayURL: "https://gateway.example",
			Token:      "omt_secret",
		}))
		if err != nil {
			t.Fatalf("patch config: %v", err)
		}
		if !strings.Contains(out, "[foo]\nmodel = \"x\"") {
			t.Fatalf("non-top-level model was not preserved:\n%s", out)
		}
		if strings.Count(out, `model = "chat-balanced"`) != 1 {
			t.Fatalf("top-level managed model missing or duplicated:\n%s", out)
		}
	})

	t.Run("multiline string is not reported as unterminated", func(t *testing.T) {
		t.Parallel()
		out, _, err := patchCodexConfig("notes = \"\"\"\nhello\n\"\"\"\n", buildCodexManagedConfig(CodexOptions{
			GatewayURL: "https://gateway.example",
			Token:      "omt_secret",
		}))
		if err != nil {
			t.Fatalf("patch config: %v", err)
		}
		if !strings.Contains(out, "notes = \"\"\"\nhello\n\"\"\"") {
			t.Fatalf("multiline string not preserved:\n%s", out)
		}
	})

	t.Run("inline provider table is rejected", func(t *testing.T) {
		t.Parallel()
		_, _, err := patchCodexConfig(`model_providers = { omnitoken = { base_url = "https://old" } }`, buildCodexManagedConfig(CodexOptions{
			GatewayURL: "https://gateway.example",
			Token:      "omt_secret",
		}))
		if !errors.Is(err, ErrInvalidExistingCodexConfig) || !strings.Contains(err.Error(), codexUnsupportedInline) {
			t.Fatalf("expected unsupported inline error, got %v", err)
		}
	})
}

func TestPatchCodexConfigManagedWarnings(t *testing.T) {
	t.Parallel()

	_, warnings, err := patchCodexConfig(`cli_auth_credentials_store = "system"
[model_providers.omnitoken]
requires_openai_auth = false
`, buildCodexManagedConfig(CodexOptions{
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	}))
	if err != nil {
		t.Fatalf("patch config: %v", err)
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "cli_auth_credentials_store: system -> file") {
		t.Fatalf("missing credentials store warning: %+v", warnings)
	}
	if !strings.Contains(joined, "requires_openai_auth: false -> true") {
		t.Fatalf("missing requires_openai_auth warning: %+v", warnings)
	}
}

func TestPatchCodexConfigLiteralStringCommentCharacter(t *testing.T) {
	t.Parallel()

	out, _, err := patchCodexConfig(`cli_auth_credentials_store = 'sys#tem'
`, buildCodexManagedConfig(CodexOptions{
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
	}))
	if err != nil {
		t.Fatalf("patch config: %v", err)
	}
	if !strings.Contains(out, `cli_auth_credentials_store = "file"`) {
		t.Fatalf("credentials store was not patched:\n%s", out)
	}
}

func TestManagedCodexKeysReturnCopies(t *testing.T) {
	t.Parallel()

	tomlKeys := ManagedCodexTomlKeys()
	tomlKeys[0] = "mutated"
	if ManagedCodexTomlKeys()[0] == "mutated" {
		t.Fatal("managed TOML keys returned mutable slice")
	}
	envKeys := ManagedCodexEnvKeys()
	envKeys[0] = "mutated"
	if ManagedCodexEnvKeys()[0] == "mutated" {
		t.Fatal("managed env keys returned mutable slice")
	}
}

func readCodexAuth(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth: %v", err)
	}
	var auth map[string]string
	if err := json.Unmarshal(data, &auth); err != nil {
		t.Fatalf("parse auth: %v\n%s", err, data)
	}
	return auth
}

func readCodexAPIKey(t *testing.T, path string) string {
	t.Helper()
	return readCodexAuth(t, path)[codexOpenAIAPIKey]
}

func assertNoBackup(t *testing.T, backupDir string) {
	t.Helper()
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("backup should not be created for invalid config")
	}
}
