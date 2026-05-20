package agent_adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultClaudeCodeModel = "chat-balanced"

var (
	ErrInvalidExistingConfig = errors.New("invalid existing Claude Code settings")
)

type ClaudeCodeOptions = BaseOptions

type RestoreClaudeCodeOptions = BaseRestoreOptions

type ClaudeCodeConfig struct{}

var omniTokenManagedKeys = []string{
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"CLAUDE_CODE_SUBAGENT_MODEL",
	"API_TIMEOUT_MS",
	"DISABLE_TELEMETRY",
	"DISABLE_ERROR_REPORTING",
	"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC",
	"CLAUDE_CODE_MAX_OUTPUT_TOKENS",
}

func ManagedClaudeCodeEnvKeys() []string {
	return append([]string(nil), omniTokenManagedKeys...)
}

func WriteClaudeCodeSettings(opts ClaudeCodeOptions) (Result, error) {
	return (&ClaudeCodeConfig{}).Write(opts)
}

func (ClaudeCodeConfig) Type() AgentType {
	return AgentTypeClaudeCode
}

func (ClaudeCodeConfig) Write(opts BaseOptions) (Result, error) {
	if strings.TrimSpace(opts.GatewayURL) == "" {
		return Result{}, fmt.Errorf("gateway url is required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return Result{}, fmt.Errorf("token is required")
	}

	home, err := resolveHome(opts.Home)
	if err != nil {
		return Result{}, err
	}
	settingsPath := claudeCodeSettingsPath(home)
	backupDir := claudeCodeBackupDir(home)

	root, existed, err := readJSONObject(settingsPath, "Claude Code settings")
	if err != nil {
		return Result{}, err
	}
	env, err := envObject(root)
	if err != nil {
		return Result{}, err
	}
	for key, value := range buildClaudeCodeEnv(opts) {
		env[key] = value
	}

	backupPath := ""
	if existed {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create backup dir: %w", err)
		}
		backupPath, err = uniqueBackupPath(backupDir, "settings.json", nowUTC(opts.Now))
		if err != nil {
			return Result{}, err
		}
		if err := copyFile(settingsPath, backupPath); err != nil {
			return Result{}, fmt.Errorf("backup Claude Code settings: %w", err)
		}
	}

	if err := writeJSONFile(settingsPath, root); err != nil {
		return Result{}, fmt.Errorf("write Claude Code settings: %w", err)
	}

	return Result{
		Paths:        paths(map[string]string{"settings": settingsPath}),
		BackupPaths:  nonEmptyStrings(backupPath),
		ManagedKeys:  ManagedClaudeCodeEnvKeys(),
		SettingsPath: settingsPath,
		BackupPath:   backupPath,
	}, nil
}

func RestoreClaudeCodeSettings() (Result, error) {
	return RestoreClaudeCodeSettingsWithOptions(RestoreClaudeCodeOptions{})
}

func RestoreClaudeCodeSettingsWithOptions(opts RestoreClaudeCodeOptions) (Result, error) {
	return (&ClaudeCodeConfig{}).Restore(opts)
}

func (ClaudeCodeConfig) Restore(opts BaseRestoreOptions) (Result, error) {
	home, err := resolveHome(opts.Home)
	if err != nil {
		return Result{}, err
	}
	settingsPath := claudeCodeSettingsPath(home)
	backupPath, err := latestBackupPath(claudeCodeBackupDir(home))
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("create Claude Code settings dir: %w", err)
	}
	if err := copyFile(backupPath, settingsPath); err != nil {
		return Result{}, fmt.Errorf("restore Claude Code settings: %w", err)
	}
	return Result{
		Paths: paths(map[string]string{"settings": settingsPath}),
		RestoredFromPaths: []string{
			backupPath,
		},
		ManagedKeys:  ManagedClaudeCodeEnvKeys(),
		SettingsPath: settingsPath,
		RestoredFrom: backupPath,
	}, nil
}

func buildClaudeCodeEnv(opts ClaudeCodeOptions) map[string]string {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultClaudeCodeModel
	}
	return map[string]string{
		"ANTHROPIC_BASE_URL":                       strings.TrimSpace(opts.GatewayURL),
		"ANTHROPIC_AUTH_TOKEN":                     strings.TrimSpace(opts.Token),
		"ANTHROPIC_MODEL":                          model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":            model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":             model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL":           model,
		"CLAUDE_CODE_SUBAGENT_MODEL":               model,
		"API_TIMEOUT_MS":                           "3000000",
		"DISABLE_TELEMETRY":                        "1",
		"DISABLE_ERROR_REPORTING":                  "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_MAX_OUTPUT_TOKENS":            "32000",
	}
}

func readClaudeCodeSettings(path string) (map[string]any, bool, error) {
	root, existed, err := readJSONObject(path, "Claude Code settings")
	if err != nil {
		if !errors.Is(err, ErrInvalidExistingConfig) {
			return nil, false, fmt.Errorf("read Claude Code settings: %w", err)
		}
		return nil, false, err
	}
	return root, existed, nil
}

func envObject(root map[string]any) (map[string]any, error) {
	raw, ok := root["env"]
	if !ok {
		env := map[string]any{}
		root["env"] = env
		return env, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: env must be a JSON object", ErrInvalidExistingConfig)
	}
	env, ok := raw.(map[string]any)
	if !ok || env == nil {
		return nil, fmt.Errorf("%w: env must be a JSON object", ErrInvalidExistingConfig)
	}
	return env, nil
}

func writeJSONFile(path string, root map[string]any) error {
	return writeJSONAtomic(path, root, 0o600)
}

func claudeCodeSettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func claudeCodeBackupDir(home string) string {
	return filepath.Join(home, ".omnitoken", "backups", "claude-code")
}
