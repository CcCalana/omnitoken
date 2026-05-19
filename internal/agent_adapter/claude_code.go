package agent_adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const defaultClaudeCodeModel = "chat-balanced"

var (
	ErrInvalidExistingConfig = errors.New("invalid existing Claude Code settings")
	errNoBackupFound         = errors.New("no Claude Code settings backup found")
)

type ClaudeCodeOptions struct {
	Home       string
	GatewayURL string
	Token      string
	Model      string
	Now        func() time.Time
}

type RestoreClaudeCodeOptions struct {
	Home string
}

type Result struct {
	SettingsPath string
	BackupPath   string
	RestoredFrom string
	ManagedKeys  []string
}

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

	root, existed, err := readClaudeCodeSettings(settingsPath)
	if err != nil {
		return Result{}, err
	}
	env, err := envObject(root)
	if err != nil {
		return Result{}, err
	}
	for key, value := range buildClaudeCodeEnv(opts) {
		encoded, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return Result{}, fmt.Errorf("marshal env value %s: %w", key, marshalErr)
		}
		env[key] = encoded
	}
	encodedEnv, err := json.Marshal(env)
	if err != nil {
		return Result{}, fmt.Errorf("marshal env object: %w", err)
	}
	root["env"] = encodedEnv

	backupPath := ""
	if existed {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create backup dir: %w", err)
		}
		backupPath, err = uniqueBackupPath(backupDir, nowUTC(opts.Now))
		if err != nil {
			return Result{}, err
		}
		if err := copyFile(settingsPath, backupPath); err != nil {
			return Result{}, fmt.Errorf("backup Claude Code settings: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return Result{}, fmt.Errorf("create Claude Code settings dir: %w", err)
	}
	if err := writeJSONFile(settingsPath, root); err != nil {
		return Result{}, fmt.Errorf("write Claude Code settings: %w", err)
	}

	return Result{
		SettingsPath: settingsPath,
		BackupPath:   backupPath,
		ManagedKeys:  ManagedClaudeCodeEnvKeys(),
	}, nil
}

func RestoreClaudeCodeSettings() (Result, error) {
	return RestoreClaudeCodeSettingsWithOptions(RestoreClaudeCodeOptions{})
}

func RestoreClaudeCodeSettingsWithOptions(opts RestoreClaudeCodeOptions) (Result, error) {
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
		SettingsPath: settingsPath,
		RestoredFrom: backupPath,
		ManagedKeys:  ManagedClaudeCodeEnvKeys(),
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

func readClaudeCodeSettings(path string) (map[string]json.RawMessage, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]json.RawMessage{}, false, nil
		}
		return nil, false, fmt.Errorf("read Claude Code settings: %w", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false, fmt.Errorf("%w: parse Claude Code settings: %v", ErrInvalidExistingConfig, err)
	}
	if root == nil {
		return nil, false, fmt.Errorf("%w: root must be a JSON object", ErrInvalidExistingConfig)
	}
	return root, true, nil
}

func envObject(root map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	raw, ok := root["env"]
	if !ok || len(raw) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("%w: env must be a JSON object", ErrInvalidExistingConfig)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil || env == nil {
		return nil, fmt.Errorf("%w: env must be a JSON object", ErrInvalidExistingConfig)
	}
	return env, nil
}

func writeJSONFile(path string, root map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func uniqueBackupPath(dir string, now time.Time) (string, error) {
	base := filepath.Join(dir, "settings.json."+now.Format("20060102T150405.000000000Z")+".bak")
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		return base, nil
	} else if err != nil {
		return "", fmt.Errorf("check backup path: %w", err)
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%03d", base, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("check backup path: %w", err)
		}
	}
}

func latestBackupPath(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "settings.json.*.bak*"))
	if err != nil {
		return "", fmt.Errorf("list Claude Code backups: %w", err)
	}
	if len(matches) == 0 {
		return "", errNoBackupFound
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

func copyFile(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func resolveHome(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Clean(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home dir is empty")
	}
	return home, nil
}

func claudeCodeSettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

func claudeCodeBackupDir(home string) string {
	return filepath.Join(home, ".omnitoken", "backups", "claude-code")
}

func nowUTC(now func() time.Time) time.Time {
	if now == nil {
		return time.Now().UTC()
	}
	return now().UTC()
}
