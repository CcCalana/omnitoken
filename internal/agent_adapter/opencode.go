// OpenCode does not expose a separate secret store; options.apiKey lives in
// opencode.json by design. CLI output must never echo this field's value.
package agent_adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultOpenCodeModel   = "chat-balanced"
	openCodeProviderName   = "omnitoken"
	openCodeProviderTitle  = "OmniToken"
	openCodeProviderNPM    = "@ai-sdk/openai-compatible"
	openCodeConfigFile     = "opencode.json"
	openCodeSchemaURL      = "https://opencode.ai/config.json"
	openCodeXDGConfigHome  = "XDG_CONFIG_HOME"
	openCodeBackupFilename = "opencode.json"
)

var ErrInvalidExistingOpenCodeConfig = errors.New("invalid existing OpenCode config")

type OpenCodeOptions struct {
	Home       string
	GatewayURL string
	Token      string
	Model      string
	Now        func() time.Time
}

type RestoreOpenCodeOptions struct {
	Home string
}

var managedOpenCodeProviderKeys = []string{
	"provider.omnitoken.name",
	"provider.omnitoken.npm",
	"provider.omnitoken.options.baseURL",
	"provider.omnitoken.options.apiKey",
	"provider.omnitoken.models.<model>.name",
}

func ManagedOpenCodeProviderKeys() []string {
	return append([]string(nil), managedOpenCodeProviderKeys...)
}

func WriteOpenCodeSettings(opts OpenCodeOptions) (Result, error) {
	if strings.TrimSpace(opts.GatewayURL) == "" {
		return Result{}, fmt.Errorf("gateway url is required")
	}
	if strings.TrimSpace(opts.Token) == "" {
		return Result{}, fmt.Errorf("token is required")
	}

	configPath, home, err := resolveOpenCodeConfigPath(opts.Home)
	if err != nil {
		return Result{}, err
	}
	backupDir := openCodeBackupDir(home)

	root, existed, err := readJSONObject(configPath, "OpenCode config")
	if err != nil {
		if errors.Is(err, ErrInvalidExistingConfig) {
			return Result{}, fmt.Errorf("%w: %v", ErrInvalidExistingOpenCodeConfig, err)
		}
		return Result{}, err
	}
	providers, err := providerObject(root)
	if err != nil {
		return Result{}, err
	}
	root["$schema"] = openCodeSchemaURL
	providers[openCodeProviderName] = buildOpenCodeProvider(opts)

	backupPath := ""
	if existed {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create backup dir: %w", err)
		}
		backupPath, err = uniqueBackupPath(backupDir, openCodeBackupFilename, nowUTC(opts.Now))
		if err != nil {
			return Result{}, err
		}
		if err := copyFile(configPath, backupPath); err != nil {
			return Result{}, fmt.Errorf("backup OpenCode config: %w", err)
		}
	}

	if err := writeJSONFile(configPath, root); err != nil {
		return Result{}, fmt.Errorf("write OpenCode config: %w", err)
	}

	return Result{
		SettingsPath: configPath,
		ConfigPath:   configPath,
		BackupPath:   backupPath,
		BackupPaths:  nonEmptyStrings(backupPath),
		ManagedKeys:  ManagedOpenCodeProviderKeys(),
	}, nil
}

func RestoreOpenCodeSettingsWithOptions(opts RestoreOpenCodeOptions) (Result, error) {
	configPath, home, err := resolveOpenCodeConfigPath(opts.Home)
	if err != nil {
		return Result{}, err
	}
	backupPath, err := latestNamedBackupPath(openCodeBackupDir(home), openCodeBackupFilename)
	if err != nil {
		return Result{}, err
	}
	if err := copyFile(backupPath, configPath); err != nil {
		return Result{}, fmt.Errorf("restore OpenCode config: %w", err)
	}
	return Result{
		SettingsPath: configPath,
		ConfigPath:   configPath,
		RestoredFrom: backupPath,
		RestoredFromPaths: []string{
			backupPath,
		},
		ManagedKeys: ManagedOpenCodeProviderKeys(),
	}, nil
}

func buildOpenCodeProvider(opts OpenCodeOptions) map[string]any {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultOpenCodeModel
	}
	return map[string]any{
		"name": openCodeProviderTitle,
		"npm":  openCodeProviderNPM,
		"options": map[string]any{
			"baseURL": strings.TrimRight(strings.TrimSpace(opts.GatewayURL), "/") + "/v1",
			"apiKey":  strings.TrimSpace(opts.Token),
		},
		"models": map[string]any{
			model: map[string]any{
				"name": model,
			},
		},
	}
}

func providerObject(root map[string]any) (map[string]any, error) {
	raw, ok := root["provider"]
	if !ok {
		providers := map[string]any{}
		root["provider"] = providers
		return providers, nil
	}
	if raw == nil {
		return nil, fmt.Errorf("%w: provider must be a JSON object", ErrInvalidExistingOpenCodeConfig)
	}
	providers, ok := raw.(map[string]any)
	if !ok || providers == nil {
		return nil, fmt.Errorf("%w: provider must be a JSON object", ErrInvalidExistingOpenCodeConfig)
	}
	return providers, nil
}

func resolveOpenCodeConfigPath(homeOverride string) (string, string, error) {
	home, err := resolveHome(homeOverride)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(homeOverride) != "" {
		return filepath.Join(home, ".config", "opencode", openCodeConfigFile), home, nil
	}
	if configHome := strings.TrimSpace(os.Getenv(openCodeXDGConfigHome)); configHome != "" {
		return filepath.Join(filepath.Clean(configHome), "opencode", openCodeConfigFile), home, nil
	}
	return filepath.Join(home, ".config", "opencode", openCodeConfigFile), home, nil
}

func openCodeBackupDir(home string) string {
	return filepath.Join(home, ".omnitoken", "backups", "opencode")
}
