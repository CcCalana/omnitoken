package agent_adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type stubConfig struct {
	agentType AgentType
}

func (s stubConfig) Type() AgentType {
	return s.agentType
}

func (s stubConfig) Write(BaseOptions) (Result, error) {
	return Result{}, nil
}

func (s stubConfig) Restore(BaseRestoreOptions) (Result, error) {
	return Result{}, nil
}

func TestRegistryRegisterGetListAndDuplicate(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if got := registry.List(); len(got) != 0 {
		t.Fatalf("new registry should be empty, got %+v", got)
	}
	if err := registry.Register(stubConfig{agentType: AgentTypeOpenCode}); err != nil {
		t.Fatalf("register opencode: %v", err)
	}
	if err := registry.Register(stubConfig{agentType: AgentTypeClaudeCode}); err != nil {
		t.Fatalf("register claude-code: %v", err)
	}
	if err := registry.Register(stubConfig{agentType: AgentTypeCodex}); err != nil {
		t.Fatalf("register codex: %v", err)
	}
	wantList := []AgentType{AgentTypeClaudeCode, AgentTypeCodex, AgentTypeOpenCode}
	if got := registry.List(); !reflect.DeepEqual(got, wantList) {
		t.Fatalf("registry list mismatch: got %+v want %+v", got, wantList)
	}
	config, ok := registry.Get(AgentTypeCodex)
	if !ok || config.Type() != AgentTypeCodex {
		t.Fatalf("get codex mismatch: config=%+v ok=%v", config, ok)
	}
	if _, ok := registry.Get(AgentType("missing")); ok {
		t.Fatal("missing agent type should not be registered")
	}
	if err := registry.Register(stubConfig{agentType: AgentTypeCodex}); err == nil {
		t.Fatal("duplicate register should fail")
	}
}

func TestRegistryRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	if err := registry.Register(nil); err == nil {
		t.Fatal("nil config should fail")
	}
	if err := registry.Register(stubConfig{}); err == nil {
		t.Fatal("empty agent type should fail")
	}
}

func TestRegistryMustRegisterPanicsOnDuplicate(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	registry.MustRegister(stubConfig{agentType: AgentTypeCodex})
	defer func() {
		if recover() == nil {
			t.Fatal("MustRegister duplicate should panic")
		}
	}()
	registry.MustRegister(stubConfig{agentType: AgentTypeCodex})
}

func TestDefaultRegistryContainsBuiltInAdapters(t *testing.T) {
	t.Parallel()

	want := []AgentType{AgentTypeClaudeCode, AgentTypeCodex, AgentTypeOpenCode}
	if got := DefaultRegistry.List(); !reflect.DeepEqual(got, want) {
		t.Fatalf("default registry list mismatch: got %+v want %+v", got, want)
	}
	for _, agentType := range want {
		config, ok := DefaultRegistry.Get(agentType)
		if !ok {
			t.Fatalf("default registry missing %s", agentType)
		}
		if config.Type() != agentType {
			t.Fatalf("default registry config type mismatch: got %s want %s", config.Type(), agentType)
		}
	}
}

func TestRegistryWriteMatchesExportedWrappers(t *testing.T) {
	cases := []struct {
		name        string
		agentType   AgentType
		registryRun func(BaseOptions) (Result, error)
		wrapperRun  func(BaseOptions) (Result, error)
		pathRole    string
		pathSuffix  string
	}{
		{
			name:      "claude-code",
			agentType: AgentTypeClaudeCode,
			registryRun: func(opts BaseOptions) (Result, error) {
				config, _ := DefaultRegistry.Get(AgentTypeClaudeCode)
				return config.Write(opts)
			},
			wrapperRun: WriteClaudeCodeSettings,
			pathRole:   "settings",
			pathSuffix: filepath.Join(".claude", "settings.json"),
		},
		{
			name:      "codex",
			agentType: AgentTypeCodex,
			registryRun: func(opts BaseOptions) (Result, error) {
				config, _ := DefaultRegistry.Get(AgentTypeCodex)
				return config.Write(opts)
			},
			wrapperRun: WriteCodexSettings,
			pathRole:   "config",
			pathSuffix: filepath.Join(".codex", "config.toml"),
		},
		{
			name:      "opencode",
			agentType: AgentTypeOpenCode,
			registryRun: func(opts BaseOptions) (Result, error) {
				config, _ := DefaultRegistry.Get(AgentTypeOpenCode)
				return config.Write(opts)
			},
			wrapperRun: WriteOpenCodeSettings,
			pathRole:   "config",
			pathSuffix: filepath.Join(".config", "opencode", "opencode.json"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			registryHome := t.TempDir()
			wrapperHome := t.TempDir()
			base := BaseOptions{
				GatewayURL: fmt.Sprintf("https://%s.example", strings.ReplaceAll(tc.name, "-", "")),
				Token:      "omt_secret",
				Model:      "chat-code",
			}
			registryResult, err := tc.registryRun(withHome(base, registryHome))
			if err != nil {
				t.Fatalf("registry write: %v", err)
			}
			wrapperResult, err := tc.wrapperRun(withHome(base, wrapperHome))
			if err != nil {
				t.Fatalf("wrapper write: %v", err)
			}
			assertResultShape(t, registryResult, registryHome, tc.pathRole, tc.pathSuffix)
			assertResultShape(t, wrapperResult, wrapperHome, tc.pathRole, tc.pathSuffix)
		})
	}
}

func TestCanonicalResultFieldsArePopulated(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	settingsPath := claudeCodeSettingsPath(home)
	writeFile(t, settingsPath, `{"env":{"ANTHROPIC_MODEL":"old"}}`)
	result, err := WriteClaudeCodeSettings(ClaudeCodeOptions{
		Home:       home,
		GatewayURL: "https://gateway.example",
		Token:      "omt_secret",
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("write Claude Code settings: %v", err)
	}
	if result.Paths["settings"] != settingsPath {
		t.Fatalf("canonical settings path mismatch: %+v", result.Paths)
	}
	if len(result.BackupPaths) != 1 {
		t.Fatalf("expected canonical backup path: %+v", result.BackupPaths)
	}
	if result.BackupPath != result.BackupPaths[0] {
		t.Fatalf("legacy backup path not populated from canonical paths: %+v", result)
	}
	if len(result.ManagedKeys) == 0 {
		t.Fatalf("managed keys not populated: %+v", result)
	}
}

func TestNonEmptyStringsSharedHelper(t *testing.T) {
	t.Parallel()

	got := nonEmptyStrings("", "a", "", "b")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nonEmptyStrings mismatch: got %+v want %+v", got, want)
	}
}

func withHome(opts BaseOptions, home string) BaseOptions {
	opts.Home = home
	return opts
}

func assertResultShape(t *testing.T, result Result, home string, role string, suffix string) {
	t.Helper()
	wantPath := filepath.Join(home, suffix)
	if result.Paths[role] != wantPath {
		t.Fatalf("canonical path mismatch: got %+v role=%s want=%s", result.Paths, role, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected written path %s: %v", wantPath, err)
	}
	if len(result.BackupPaths) != 0 {
		t.Fatalf("first write should not create backups: %+v", result.BackupPaths)
	}
}
