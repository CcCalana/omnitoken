# AI 中转站 Agent 适配架构设计参考

> 来源：[tingly-box](https://github.com/tingly-dev/tingly-box) 项目完整源码级分析  
> 场景：构建一个 AI 中转站（Gateway/Proxy），让 Claude Code、OpenCode、Codex 等 Agent 通过中转站访问底层模型提供商。

---

## 一、核心思想

**不是简单改个 API URL，而是建立一套"配置适配 + 路由映射"的完整桥梁。**

Agent 工具（Claude Code、Codex CLI 等）通常硬编码了模型名和 API 端点。要让它们走中转站，需要：

1. **欺骗 Agent**：让 Agent 以为它在调用官方 API，实际上指向中转站
2. **建立映射**：中转站内部知道 `tingly/cc` 应该路由到哪个真实提供商的哪个模型
3. **统一管理**：新增 Agent 时不需要改动核心逻辑

---

## 二、分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                    业务编排层 (internal/agent)                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ AgentApply   │  │  Builder     │  │  Rule Manager    │  │
│  │ 主入口        │  │  参数构造     │  │  路由规则 CRUD   │  │
│  └──────┬───────┘  └──────────────┘  └──────────────────┘  │
└─────────┼───────────────────────────────────────────────────┘
          │ 调用 Apply(params)
┌─────────▼───────────────────────────────────────────────────┐
│                    核心适配层 (ai/agent)                     │
│  ┌───────────────────────────────────────────────────────┐ │
│  │              Registry (注册表)                         │ │
│  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────┐ │ │
│  │  │ClaudeCode   │ │ OpenCode    │ │    Codex        │ │ │
│  │  │  Config     │ │  Config     │ │   Config        │ │ │
│  │  └─────────────┘ └─────────────┘ └─────────────────┘ │ │
│  └───────────────────────────────────────────────────────┘ │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌───────────────────┐   │
│  │ ClaudeCode  │  │  OpenCode   │  │      Codex        │   │
│  │  Params     │  │   Params    │  │     Params        │   │
│  └─────────────┘  └─────────────┘  └───────────────────┘   │
└─────────────────────────────────────────────────────────────┘
          │ 写入本地配置文件
┌─────────▼───────────────────────────────────────────────────┐
│              本地 Agent 配置文件                             │
│   ~/.claude/settings.json   ~/.config/opencode/opencode.json │
│   ~/.codex/config.toml      ~/.codex/auth.json               │
└─────────────────────────────────────────────────────────────┘
```

### 2.1 为什么分层？

| 层级 | 职责 | 依赖 | 复用性 |
|---|---|---|---|
| `ai/agent`（核心适配层） | 只负责"写入本地配置文件"，纯粹的技术适配 | 不依赖业务系统 | **高**——可以单独拆成库 |
| `internal/agent`（业务编排层） | 负责"调用适配 + 创建路由规则" | 依赖中转站内部的路由系统 | **低**——与业务紧耦合 |

---

## 三、核心适配层完整源码 — `ai/agent/`

### 3.1 统一接口与注册表 — `interface.go`

```go
package agent

// AgentConfig defines the interface for agent configuration operations.
// Each agent type implements this interface independently.
type AgentConfig interface {
	// Apply applies the agent configuration files.
	// Does NOT handle routing rules - that's handled separately.
	Apply(params interface{}) (*ApplyAgentResult, error)

	// Restore restores configuration files from backup.
	Restore() (*RestoreAgentResult, error)
}

// AgentConfigInfo provides metadata about an agent config implementation.
type AgentConfigInfo struct {
	// Type is the agent type
	Type AgentType
	// Name is the display name
	Name string
	// Description is a brief description
	Description string
	// ConfigFiles lists the configuration files this agent uses
	ConfigFiles []string
	// Scenario is the corresponding routing rule scenario
	Scenario string
}

// Registry holds agent config implementations
type Registry struct {
	configs map[AgentType]AgentConfig
}

// NewRegistry creates a new agent config registry
func NewRegistry() *Registry {
	return &Registry{
		configs: make(map[AgentType]AgentConfig),
	}
}

// Register registers an agent config implementation
func (r *Registry) Register(agentType AgentType, config AgentConfig) {
	r.configs[agentType] = config
}

// Get returns the agent config for the given type
func (r *Registry) Get(agentType AgentType) (AgentConfig, bool) {
	config, ok := r.configs[agentType]
	return config, ok
}

// DefaultRegistry is the global registry with all built-in agents
var DefaultRegistry = NewRegistry()

func init() {
	// Register all built-in agents
	DefaultRegistry.Register(AgentTypeClaudeCode, &ClaudeCodeConfig{})
	DefaultRegistry.Register(AgentTypeOpenCode, &OpenCodeConfig{})
	DefaultRegistry.Register(AgentTypeCodex, &CodexConfig{})
}
```

### 3.2 统一数据结构 — `types.go`

```go
package agent

// AgentType represents the type of AI agent to configure
type AgentType string

const (
	// AgentTypeClaudeCode represents Claude Code agent
	AgentTypeClaudeCode AgentType = "claude-code"

	// AgentTypeOpenCode represents OpenCode IDE extension
	AgentTypeOpenCode AgentType = "opencode"

	// AgentTypeCodex represents the OpenAI Codex CLI
	AgentTypeCodex AgentType = "codex"
)

// String returns the string representation of AgentType
func (at AgentType) String() string {
	return string(at)
}

// IsValid checks if the AgentType is valid
func (at AgentType) IsValid() bool {
	switch at {
	case AgentTypeClaudeCode, AgentTypeOpenCode, AgentTypeCodex:
		return true
	default:
		return false
	}
}

// ApplyAgentRequest represents a request to apply agent configuration
type ApplyAgentRequest struct {
	// AgentType is the type of agent to configure (required)
	AgentType AgentType

	// Provider is the provider UUID to use (optional, prompts if empty)
	Provider string

	// Model is the model name to use (optional, prompts if empty)
	Model string

	// Unified specifies unified mode for claude-code (single config for all models)
	// Only applicable for AgentTypeClaudeCode
	Unified bool

	// Force skips confirmation prompts
	Force bool

	// Preview shows what would be applied without actually applying
	Preview bool

	// InstallStatusLine installs the status line script for Claude Code
	// Only applicable for AgentTypeClaudeCode
	InstallStatusLine bool
}

// ApplyAgentResult represents the result of applying agent configuration
type ApplyAgentResult struct {
	// Success indicates whether the operation completed successfully
	Success bool

	// AgentType is the type of agent that was configured
	AgentType AgentType

	// ProviderName is the name of the provider that was selected
	ProviderName string

	// ProviderUUID is the UUID of the provider that was selected
	ProviderUUID string

	// Model is the model name that was selected
	Model string

	// ConfigFiles lists the files that were created or updated
	ConfigFiles []string

	// BackupPaths lists the paths to backup files created
	BackupPaths []string

	// RulesCreated indicates how many new routing rules were created
	RulesCreated int

	// RulesUpdated indicates how many existing routing rules were updated
	RulesUpdated int

	// Warnings collects non-fatal messages emitted during apply
	Warnings []string

	// Message contains a human-readable result message
	Message string
}

// RestoreAgentRequest represents a request to restore agent configuration
// from the most recent on-disk backup.
type RestoreAgentRequest struct {
	// AgentType is the type of agent to restore (required)
	AgentType AgentType

	// Force skips confirmation prompts (CLI use)
	Force bool
}

// RestoreAgentResult represents the result of restoring agent configuration.
type RestoreAgentResult struct {
	// Success is true only when every relevant config file was restored
	// without error.
	Success bool

	// AgentType is the type of agent that was restored.
	AgentType AgentType

	// RestoredFiles lists "<original> <- <backup>" entries for files that
	// were successfully restored.
	RestoredFiles []string

	// PreRestoreBackups lists the safety snapshots taken of each live file
	// before the restore overwrote it.
	PreRestoreBackups []string

	// Failures lists per-file error messages, e.g. "no backup found".
	// Non-empty Failures with empty RestoredFiles means Success == false.
	Failures []string

	// Message is a human-readable summary suitable for CLI output.
	Message string
}
```

### 3.3 Claude Code 适配 — `claude_code.go`

```go
package agent

import (
	"fmt"

	serverconfig "github.com/tingly-dev/tingly-box/internal/server/config"
)

// ClaudeCodeConfig implements AgentConfig for Claude Code
type ClaudeCodeConfig struct{}

// ClaudeCodeParams contains parameters for applying Claude Code configuration
type ClaudeCodeParams struct {
	// BaseURL is the base URL for the Claude API
	BaseURL string

	// APIKey is the authentication token
	APIKey string

	// Model configuration
	ModelConfig ClaudeCodeModelConfig

	// InstallStatusLine installs the status line script
	InstallStatusLine bool

	// ExtraEnv contains additional environment variables beyond the standard ones
	ExtraEnv map[string]string

	// ExtraConfig contains additional config entries for settings.json
	ExtraConfig map[string]interface{}
}

// ClaudeCodeModelConfig defines which models to use for different purposes
type ClaudeCodeModelConfig struct {
	// Default is the default model to use
	Default string

	// Haiku is the model for Haiku requests (optional, uses Default if empty)
	Haiku string

	// Opus is the model for Opus requests (optional, uses Default if empty)
	Opus string

	// Sonnet is the model for Sonnet requests (optional, uses Default if empty)
	Sonnet string

	// SubAgent is the model for sub-agent tasks (optional, uses Default if empty)
	SubAgent string
}

// BuildEnv constructs the complete environment variables map from params
func (p *ClaudeCodeParams) BuildEnv() map[string]string {
	env := map[string]string{
		"DISABLE_TELEMETRY":                        "1",
		"DISABLE_ERROR_REPORTING":                  "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_MAX_OUTPUT_TOKENS":            "32000",
		"API_TIMEOUT_MS":                           "3000000",
		"ANTHROPIC_BASE_URL":                       p.BaseURL,
		"ANTHROPIC_AUTH_TOKEN":                     p.APIKey,
	}

	// Model configuration
	defaultModel := p.ModelConfig.Default
	if defaultModel == "" {
		defaultModel = "tingly/cc"
	}

	env["ANTHROPIC_MODEL"] = defaultModel

	if p.ModelConfig.Haiku != "" {
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = p.ModelConfig.Haiku
	} else {
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = defaultModel
	}

	if p.ModelConfig.Opus != "" {
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = p.ModelConfig.Opus
	} else {
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = defaultModel
	}

	if p.ModelConfig.Sonnet != "" {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = p.ModelConfig.Sonnet
	} else {
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = defaultModel
	}

	if p.ModelConfig.SubAgent != "" {
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = p.ModelConfig.SubAgent
	} else {
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = defaultModel
	}

	// Add extra env vars
	for k, v := range p.ExtraEnv {
		env[k] = v
	}

	return env
}

// Apply applies Claude Code configuration files
func (c *ClaudeCodeConfig) Apply(paramsInterface interface{}) (*ApplyAgentResult, error) {
	params, ok := paramsInterface.(*ClaudeCodeParams)
	if !ok {
		return nil, fmt.Errorf("invalid params type, expected *ClaudeCodeParams")
	}

	// Build env from params
	env := params.BuildEnv()

	// Apply settings.json
	settingsResult, err := applyClaudeSettings(env, params.InstallStatusLine, params.ExtraConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Claude settings: %w", err)
	}

	// Apply .claude.json
	onboardingResult, err := applyClaudeOnboarding()
	if err != nil {
		return nil, fmt.Errorf("failed to apply Claude onboarding: %w", err)
	}

	// Collect results
	result := &ApplyAgentResult{
		AgentType:   AgentTypeClaudeCode,
		Success:     settingsResult.Success && onboardingResult.Success,
		ConfigFiles: collectConfigFiles(settingsResult, onboardingResult),
		BackupPaths: collectBackupPaths(settingsResult, onboardingResult),
	}

	return result, nil
}

// Restore restores Claude Code configuration from backup
func (c *ClaudeCodeConfig) Restore() (*RestoreAgentResult, error) {
	return RestoreAgent(AgentTypeClaudeCode)
}

// ApplyClaudeCode applies Claude Code configuration files.
// Deprecated: Use ClaudeCodeConfig.Apply() instead
func ApplyClaudeCode(params *ClaudeCodeParams) (*ApplyAgentResult, error) {
	config := &ClaudeCodeConfig{}
	return config.Apply(params)
}

// applyClaudeSettings applies Claude Code settings.json
func applyClaudeSettings(env map[string]string, installStatusLine bool, extraConfig map[string]interface{}) (*serverconfig.ApplyResult, error) {
	var opts []serverconfig.ApplyOption
	if installStatusLine {
		_, _, err := serverconfig.InstallStatusLineScript()
		if err != nil {
			return nil, fmt.Errorf("failed to install status line script: %w", err)
		}
		statusLineCmd := "~/.claude/tingly-statusline.sh"
		statusLine := map[string]any{"type": "command", "command": statusLineCmd}
		opts = append(opts, serverconfig.WithExtra("statusLine", statusLine))
	}

	for key, value := range extraConfig {
		opts = append(opts, serverconfig.WithExtra(key, value))
	}

	return serverconfig.ApplyClaudeSettingsFromEnv(env, opts...)
}

// applyClaudeOnboarding applies Claude Code .claude.json
func applyClaudeOnboarding() (*serverconfig.ApplyResult, error) {
	payload := map[string]interface{}{
		"hasCompletedOnboarding": true,
	}
	return serverconfig.ApplyClaudeOnboarding(payload)
}

// collectConfigFiles collects the config file paths from apply results
func collectConfigFiles(results ...*serverconfig.ApplyResult) []string {
	var files []string
	for _, r := range results {
		if r == nil {
			continue
		}
		msg := r.Message
		if len(msg) > 8 && containsPrefix(msg[8:], "Created ") {
			file := extractFilePath(msg[8:])
			if file != "" {
				files = append(files, file+" (created)")
			}
		} else if len(msg) > 8 && containsPrefix(msg[8:], "Updated ") {
			file := extractFilePath(msg[8:])
			if file != "" {
				files = append(files, file+" (updated)")
			}
		}
	}
	return files
}

// collectBackupPaths collects backup paths from apply results
func collectBackupPaths(results ...*serverconfig.ApplyResult) []string {
	var paths []string
	for _, r := range results {
		if r != nil && r.BackupPath != "" {
			paths = append(paths, r.BackupPath)
		}
	}
	return paths
}

// Helper functions to avoid importing strings package

func containsPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func extractFilePath(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '(' {
			return s[:i]
		}
	}
	return ""
}
```

### 3.4 OpenCode 适配 — `opencode.go`

```go
package agent

import (
	"fmt"

	serverconfig "github.com/tingly-dev/tingly-box/internal/server/config"
)

// OpenCodeConfig implements AgentConfig for OpenCode
type OpenCodeConfig struct{}

// OpenCodeParams contains parameters for applying OpenCode configuration
type OpenCodeParams struct {
	// Config is the complete OpenCode configuration object
	// Caller is responsible for constructing this with appropriate structure
	Config map[string]interface{}
}

// Apply applies OpenCode configuration
func (o *OpenCodeConfig) Apply(paramsInterface interface{}) (*ApplyAgentResult, error) {
	params, ok := paramsInterface.(*OpenCodeParams)
	if !ok {
		return nil, fmt.Errorf("invalid params type, expected *OpenCodeParams")
	}

	applyResult, err := serverconfig.ApplyOpenCodeConfig(params.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to apply OpenCode config: %w", err)
	}

	result := &ApplyAgentResult{
		AgentType: AgentTypeOpenCode,
		Success:   applyResult.Success,
	}

	if applyResult.Success {
		result.ConfigFiles = []string{"~/.config/opencode/opencode.json"}
		if applyResult.Created {
			result.ConfigFiles[0] += " (created)"
		} else {
			result.ConfigFiles[0] += " (updated)"
		}
	}

	if applyResult.BackupPath != "" {
		result.BackupPaths = []string{applyResult.BackupPath}
	}

	return result, nil
}

// Restore restores OpenCode configuration from backup
func (o *OpenCodeConfig) Restore() (*RestoreAgentResult, error) {
	return RestoreAgent(AgentTypeOpenCode)
}

// ApplyOpenCode applies OpenCode configuration.
// Deprecated: Use OpenCodeConfig.Apply() instead
func ApplyOpenCode(params *OpenCodeParams) (*ApplyAgentResult, error) {
	config := &OpenCodeConfig{}
	return config.Apply(params)
}
```

### 3.5 Codex 适配 — `codex.go`

```go
package agent

import (
	"fmt"

	serverconfig "github.com/tingly-dev/tingly-box/internal/server/config"
)

// CodexConfig implements AgentConfig for Codex
type CodexConfig struct{}

// CodexParams contains parameters for applying Codex configuration
type CodexParams struct {
	// CodexBaseURL is the base URL for Codex API endpoint
	CodexBaseURL string

	// APIKey is the authentication token
	APIKey string

	// Models is a list of model names for the Codex profiles
	// Caller is responsible for collecting and deduplicating these
	Models []string
}

// Apply applies Codex CLI configuration
func (c *CodexConfig) Apply(paramsInterface interface{}) (*ApplyAgentResult, error) {
	params, ok := paramsInterface.(*CodexParams)
	if !ok {
		return nil, fmt.Errorf("invalid params type, expected *CodexParams")
	}

	// Apply config.toml
	configResult, err := serverconfig.ApplyCodexConfig(params.CodexBaseURL, params.Models)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Codex config: %w", err)
	}

	// Apply auth.json
	authResult, err := serverconfig.ApplyCodexAuth(params.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Codex auth: %w", err)
	}

	result := &ApplyAgentResult{
		AgentType: AgentTypeCodex,
		Success:   configResult.Success && authResult.Success,
	}

	if configResult.Success {
		suffix := " (updated)"
		if configResult.Created {
			suffix = " (created)"
		}
		result.ConfigFiles = append(result.ConfigFiles, "~/.codex/config.toml"+suffix)
	}
	if authResult.Success {
		suffix := " (updated)"
		if authResult.Created {
			suffix = " (created)"
		}
		result.ConfigFiles = append(result.ConfigFiles, "~/.codex/auth.json"+suffix)
	}

	if configResult.BackupPath != "" {
		result.BackupPaths = append(result.BackupPaths, configResult.BackupPath)
	}
	if authResult.BackupPath != "" {
		result.BackupPaths = append(result.BackupPaths, authResult.BackupPath)
	}

	return result, nil
}

// Restore restores Codex configuration from backup
func (c *CodexConfig) Restore() (*RestoreAgentResult, error) {
	return RestoreAgent(AgentTypeCodex)
}

// ApplyCodex applies Codex CLI configuration.
// Deprecated: Use CodexConfig.Apply() instead
func ApplyCodex(params *CodexParams) (*ApplyAgentResult, error) {
	config := &CodexConfig{}
	return config.Apply(params)
}
```

### 3.6 备份恢复 — `restore.go`

```go
package agent

import (
	"fmt"
	"os"
	"path/filepath"
)

// RestoreAgent restores configuration files for the given agent type.
// It looks for the most recent backup in a well-known backup directory.
func RestoreAgent(agentType AgentType) (*RestoreAgentResult, error) {
	result := &RestoreAgentResult{
		AgentType:     agentType,
		Success:       true,
		RestoredFiles: []string{},
		Failures:      []string{},
	}

	backupDir := getBackupDir(agentType)

	// If backup dir doesn't exist, nothing to restore
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		result.Success = false
		result.Failures = append(result.Failures,
			fmt.Sprintf("no backup directory found at %s", backupDir))
		result.Message = "No backups found"
		return result, nil
	}

	// List backup files and restore the most recent for each target file
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		result.Success = false
		result.Failures = append(result.Failures, fmt.Sprintf("read backup dir: %v", err))
		result.Message = "Failed to read backups"
		return result, nil
	}

	// Group by original file name, pick most recent
	latestBackups := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		// Assume backup naming: <original_name>_<timestamp>.bak
		name := entry.Name()
		orig := extractOriginalName(name)
		if orig == "" {
			continue
		}
		existing, ok := latestBackups[orig]
		if !ok || name > existing {
			latestBackups[orig] = filepath.Join(backupDir, name)
		}
	}

	for orig, backupPath := range latestBackups {
		targetPath := getOriginalPath(agentType, orig)

		// Pre-restore backup of current file (safety snapshot)
		if _, err := os.Stat(targetPath); err == nil {
			snapshot := targetPath + ".pre_restore"
			data, _ := os.ReadFile(targetPath)
			_ = os.WriteFile(snapshot, data, 0644)
			result.PreRestoreBackups = append(result.PreRestoreBackups, snapshot)
		}

		data, err := os.ReadFile(backupPath)
		if err != nil {
			result.Success = false
			result.Failures = append(result.Failures,
				fmt.Sprintf("read backup %s: %v", backupPath, err))
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			result.Success = false
			result.Failures = append(result.Failures,
				fmt.Sprintf("mkdir %s: %v", filepath.Dir(targetPath), err))
			continue
		}

		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			result.Success = false
			result.Failures = append(result.Failures,
				fmt.Sprintf("write %s: %v", targetPath, err))
			continue
		}

		result.RestoredFiles = append(result.RestoredFiles,
			fmt.Sprintf("%s <- %s", targetPath, backupPath))
	}

	if len(result.RestoredFiles) == 0 && len(result.Failures) > 0 {
		result.Success = false
	}

	result.Message = buildRestoreMessage(result)
	return result, nil
}

func getBackupDir(agentType AgentType) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tingly-box", "backups", string(agentType))
}

func getOriginalPath(agentType AgentType, filename string) string {
	home, _ := os.UserHomeDir()
	switch agentType {
	case AgentTypeClaudeCode:
		if filename == "settings.json" {
			return filepath.Join(home, ".claude", "settings.json")
		}
		if filename == ".claude.json" {
			return filepath.Join(home, ".claude.json")
		}
	case AgentTypeOpenCode:
		return filepath.Join(home, ".config", "opencode", "opencode.json")
	case AgentTypeCodex:
		return filepath.Join(home, ".codex", filename)
	}
	return filepath.Join(home, filename)
}

func extractOriginalName(backupName string) string {
	// Remove timestamp suffix: settings_20260101_120000.bak -> settings.json
	// This is a simplified heuristic
	for i := len(backupName) - 1; i >= 0; i-- {
		if backupName[i] == '_' {
			return backupName[:i]
		}
	}
	return ""
}

func buildRestoreMessage(r *RestoreAgentResult) string {
	if !r.Success {
		return "Restore failed"
	}
	return fmt.Sprintf("Restored %d file(s) for %s", len(r.RestoredFiles), r.AgentType)
}
```

### 3.7 辅助工具 — `utils.go`

```go
package agent

import (
	"os"
	"path/filepath"
)

// ensureDir creates a directory if it doesn't exist.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// expandHome expands ~/ to the user's home directory.
func expandHome(path string) string {
	if len(path) >= 2 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
```

---

## 四、业务编排层完整源码 — `internal/agent/`

### 4.1 参数构造器 — `builder.go`

```go
package agent

import aiagent "github.com/tingly-dev/tingly-box/ai/agent"

// BuildClaudeCodeEnv constructs environment variables for Claude Code.
// This function contains the business logic for unified vs separate mode.
func BuildClaudeCodeEnv(baseURL, apiKey string, unified bool) map[string]string {
	basePath := baseURL + "/tingly/claude_code"

	env := map[string]string{
		"DISABLE_TELEMETRY":                        "1",
		"DISABLE_ERROR_REPORTING":                  "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"CLAUDE_CODE_MAX_OUTPUT_TOKENS":            "32000",
		"API_TIMEOUT_MS":                           "3000000",
		"ANTHROPIC_BASE_URL":                       basePath,
		"ANTHROPIC_AUTH_TOKEN":                     apiKey,
	}

	if unified {
		// Unified mode - all point to same model
		env["ANTHROPIC_MODEL"] = "tingly/cc"
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = "tingly/cc"
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = "tingly/cc"
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = "tingly/cc"
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = "tingly/cc"
	} else {
		// Separate mode - different models for different purposes
		env["ANTHROPIC_MODEL"] = "tingly/cc-default"
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = "tingly/cc-haiku"
		env["ANTHROPIC_DEFAULT_OPUS_MODEL"] = "tingly/cc-opus"
		env["ANTHROPIC_DEFAULT_SONNET_MODEL"] = "tingly/cc-sonnet"
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = "tingly/cc-subagent"
	}

	return env
}

// BuildClaudeCodeModelConfig constructs the model configuration for Claude Code.
// This contains the business logic for unified vs separate mode.
// Exported for use by HTTP handlers.
func BuildClaudeCodeModelConfig(unified bool) aiagent.ClaudeCodeModelConfig {
	if unified {
		return aiagent.ClaudeCodeModelConfig{
			Default: "tingly/cc",
			// All other fields will use Default
		}
	}

	// Separate mode - different models for different purposes
	return aiagent.ClaudeCodeModelConfig{
		Default:  "tingly/cc-default",
		Haiku:    "tingly/cc-haiku",
		Opus:     "tingly/cc-opus",
		Sonnet:   "tingly/cc-sonnet",
		SubAgent: "tingly/cc-subagent",
	}
}

// BuildOpenCodeConfig constructs the OpenCode configuration object.
// This function contains the business logic for OpenCode config structure.
func BuildOpenCodeConfig(configBaseURL, apiKey string, models map[string]interface{}) map[string]interface{} {
	if len(models) == 0 {
		// Default single-model layout
		models = map[string]interface{}{
			"tingly-opencode": map[string]interface{}{"name": "tingly-opencode"},
		}
	}

	providerConfig := map[string]interface{}{
		"tingly-box": map[string]interface{}{
			"name": "tingly-box",
			"npm":  "@ai-sdk/anthropic",
			"options": map[string]interface{}{
				"baseURL": configBaseURL,
				"apiKey":  apiKey,
			},
			"models": models,
		},
	}

	return map[string]interface{}{
		"$schema":  "https://opencode.ai/config.json",
		"provider": providerConfig,
	}
}

// CollectCodexModels deduplicates and preserves order of model names.
// This helper processes routing rules to extract unique model names.
func CollectCodexModels(rules []string) []string {
	seen := map[string]struct{}{}
	var out []string

	for _, ruleModel := range rules {
		model := trimSpace(ruleModel)
		if model == "" {
			continue
		}
		if _, exists := seen[model]; exists {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}

	return out
}

// String helpers to avoid importing strings package

func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}

	return s[start:end]
}
```

### 4.2 路由规则管理 — `rule.go`

```go
package agent

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/tingly-dev/tingly-box/internal/loadbalance"
	"github.com/tingly-dev/tingly-box/internal/typ"
)

// ClaudeCodeRequestModels defines all request models for Claude Code scenario
var ClaudeCodeRequestModels = []string{
	"tingly/cc",        // General model (for unified mode)
	"tingly/cc-haiku",
	"tingly/cc-sonnet",
	"tingly/cc-opus",
	"tingly/cc-default",
	"tingly/cc-subagent",
}

// OpenCodeRequestModels defines all request models for OpenCode scenario
var OpenCodeRequestModels = []string{
	"tingly-opencode",
}

// CodexRequestModels defines the default request models for the Codex scenario.
var CodexRequestModels = []string{
	"tingly-codex",
}

// createOrUpdateClaudeCodeRules creates or updates all Claude Code rules
func (aa *AgentApply) createOrUpdateClaudeCodeRules(providerUUID, model string) (int, int, error) {
	created := 0
	updated := 0

	service := &loadbalance.Service{
		Active:   true,
		Provider: providerUUID,
		Model:    model,
	}

	for _, requestModel := range ClaudeCodeRequestModels {
		ruleCreated, ruleUpdated, err := aa.createOrUpdateRule(
			typ.ScenarioClaudeCode,
			requestModel,
			service,
			fmt.Sprintf("Claude Code - %s routing", requestModel),
		)
		if err != nil {
			return created, updated, fmt.Errorf("failed to update rule %s: %w", requestModel, err)
		}
		if ruleCreated {
			created++
		}
		if ruleUpdated {
			updated++
		}
	}

	return created, updated, nil
}

// createOrUpdateOpenCodeRules creates or updates OpenCode rules
func (aa *AgentApply) createOrUpdateOpenCodeRules(providerUUID, model string) (int, int, error) {
	created := 0
	updated := 0

	service := &loadbalance.Service{
		Active:   true,
		Provider: providerUUID,
		Model:    model,
	}

	for _, requestModel := range OpenCodeRequestModels {
		ruleCreated, ruleUpdated, err := aa.createOrUpdateRule(
			typ.ScenarioOpenCode,
			requestModel,
			service,
			fmt.Sprintf("OpenCode - %s routing", requestModel),
		)
		if err != nil {
			return created, updated, fmt.Errorf("failed to update rule %s: %w", requestModel, err)
		}
		if ruleCreated {
			created++
		}
		if ruleUpdated {
			updated++
		}
	}

	return created, updated, nil
}

// createOrUpdateCodexRules creates or updates the default Codex rule
func (aa *AgentApply) createOrUpdateCodexRules(providerUUID, model string) (int, int, error) {
	created := 0
	updated := 0

	service := &loadbalance.Service{
		Active:   true,
		Provider: providerUUID,
		Model:    model,
	}

	for _, requestModel := range CodexRequestModels {
		ruleCreated, ruleUpdated, err := aa.createOrUpdateRule(
			typ.ScenarioCodex,
			requestModel,
			service,
			fmt.Sprintf("Codex - %s routing", requestModel),
		)
		if err != nil {
			return created, updated, fmt.Errorf("failed to update rule %s: %w", requestModel, err)
		}
		if ruleCreated {
			created++
		}
		if ruleUpdated {
			updated++
		}
	}

	return created, updated, nil
}

// createOrUpdateRule creates or updates a single rule
func (aa *AgentApply) createOrUpdateRule(
	scenario typ.RuleScenario,
	requestModel string,
	service *loadbalance.Service,
	description string,
) (bool, bool, error) {
	existingRule := aa.config.GetRuleByRequestModelAndScenario(requestModel, scenario)

	if existingRule != nil {
		existingRule.Services = []*loadbalance.Service{service}
		existingRule.Active = true
		if err := aa.config.UpdateRule(existingRule.UUID, *existingRule); err != nil {
			return false, false, fmt.Errorf("failed to update rule: %w", err)
		}
		return false, true, nil
	}

	rule := typ.Rule{
		UUID:          uuid.New().String(),
		Scenario:      scenario,
		RequestModel:  requestModel,
		ResponseModel: "",
		Description:   description,
		Services:      []*loadbalance.Service{service},
		LBTactic: typ.Tactic{
			Type:   loadbalance.TacticRandom,
			Params: typ.DefaultRandomParams(),
		},
		Active: true,
	}

	if err := aa.config.AddRule(rule); err != nil {
		return false, false, fmt.Errorf("failed to add rule: %w", err)
	}

	return true, false, nil
}
```

### 4.3 主入口与业务编排 — `rule_bridge.go`

```go
package agent

import (
	"fmt"
	"strings"

	aiagent "github.com/tingly-dev/tingly-box/ai/agent"
	serverconfig "github.com/tingly-dev/tingly-box/internal/server/config"
	"github.com/tingly-dev/tingly-box/internal/typ"
)

// Re-export types from ai/agent for backward compatibility
type (
	AgentType           = aiagent.AgentType
	ApplyAgentRequest   = aiagent.ApplyAgentRequest
	ApplyAgentResult    = aiagent.ApplyAgentResult
	RestoreAgentRequest = aiagent.RestoreAgentRequest
	RestoreAgentResult  = aiagent.RestoreAgentResult
	AgentInfo           = aiagent.AgentInfo
)

const (
	AgentTypeClaudeCode = aiagent.AgentTypeClaudeCode
	AgentTypeOpenCode   = aiagent.AgentTypeOpenCode
	AgentTypeCodex      = aiagent.AgentTypeCodex
)

var (
	ParseAgentType = aiagent.ParseAgentType
	ListAgentInfo  = aiagent.ListAgentInfo
	GetAgentInfo   = aiagent.GetAgentInfo
)

// AgentApply handles agent configuration with routing rules (Tingly-Box specific)
type AgentApply struct {
	config *serverconfig.Config
	host   string
}

// NewAgentApply creates a new AgentApply instance
func NewAgentApply(cfg *serverconfig.Config, host string) *AgentApply {
	return &AgentApply{
		config: cfg,
		host:   host,
	}
}

// ApplyAgent applies configuration including routing rules
// This is the main entry point for Tingly-Box agent configuration
func (aa *AgentApply) ApplyAgent(req *ApplyAgentRequest) (*ApplyAgentResult, error) {
	if !req.AgentType.IsValid() {
		return nil, fmt.Errorf("unknown agent type: %s", req.AgentType)
	}

	var fileResult *ApplyAgentResult
	var err error

	baseURL, apiKey := aa.getBaseURLAndToken()

	switch req.AgentType {
	case AgentTypeClaudeCode:
		config, ok := aiagent.DefaultRegistry.Get(req.AgentType)
		if !ok {
			return nil, fmt.Errorf("claude code config not registered")
		}
		modelConfig := BuildClaudeCodeModelConfig(req.Unified)
		fileResult, err = config.Apply(&aiagent.ClaudeCodeParams{
			BaseURL:           baseURL + "/tingly/claude_code",
			APIKey:            apiKey,
			ModelConfig:       modelConfig,
			InstallStatusLine: req.InstallStatusLine,
			ExtraEnv:          nil,
			ExtraConfig:       nil,
		})
	case AgentTypeOpenCode:
		config, ok := aiagent.DefaultRegistry.Get(req.AgentType)
		if !ok {
			return nil, fmt.Errorf("opencode config not registered")
		}
		configBaseURL := baseURL + "/tingly/opencode"
		openCodeConfig := BuildOpenCodeConfig(configBaseURL, apiKey, nil)
		fileResult, err = config.Apply(&aiagent.OpenCodeParams{
			Config: openCodeConfig,
		})
	case AgentTypeCodex:
		config, ok := aiagent.DefaultRegistry.Get(req.AgentType)
		if !ok {
			return nil, fmt.Errorf("codex config not registered")
		}
		codexBaseURL := baseURL + "/tingly/codex"
		rawModels := aa.collectCodexRuleModels()
		models := CollectCodexModels(rawModels)
		fileResult, err = config.Apply(&aiagent.CodexParams{
			CodexBaseURL: codexBaseURL,
			APIKey:       apiKey,
			Models:       models,
		})
	}

	if err != nil {
		return nil, err
	}

	// 2. Apply routing rules (Tingly-specific)
	if req.Provider != "" && req.Model != "" {
		provider, err := aa.config.GetProviderByUUID(req.Provider)
		if err != nil || provider == nil {
			fileResult.Warnings = append(fileResult.Warnings,
				fmt.Sprintf("provider %s not found; skipping routing rule update", req.Provider))
		} else {
			fileResult.ProviderName = provider.Name
			fileResult.ProviderUUID = provider.UUID
			fileResult.Model = req.Model

			ruleCreated, ruleUpdated, err := aa.createOrUpdateRules(req.AgentType, req.Provider, req.Model)
			if err != nil {
				fileResult.Warnings = append(fileResult.Warnings,
					fmt.Sprintf("failed to create/update routing rules: %v", err))
			} else {
				fileResult.RulesCreated = ruleCreated
				fileResult.RulesUpdated = ruleUpdated
			}
		}
	}

	fileResult.Message = aa.buildResultMessage(fileResult)
	return fileResult, nil
}

// createOrUpdateRules creates or updates routing rules for the given agent type
func (aa *AgentApply) createOrUpdateRules(agentType AgentType, providerUUID, model string) (int, int, error) {
	switch agentType {
	case AgentTypeClaudeCode:
		return aa.createOrUpdateClaudeCodeRules(providerUUID, model)
	case AgentTypeOpenCode:
		return aa.createOrUpdateOpenCodeRules(providerUUID, model)
	case AgentTypeCodex:
		return aa.createOrUpdateCodexRules(providerUUID, model)
	default:
		return 0, 0, fmt.Errorf("agent type %s not implemented", agentType)
	}
}

// RestoreAgent restores configuration files from backup
func (aa *AgentApply) RestoreAgent(req *RestoreAgentRequest) (*RestoreAgentResult, error) {
	config, ok := aiagent.DefaultRegistry.Get(req.AgentType)
	if !ok {
		return nil, fmt.Errorf("no config registered for agent type: %s", req.AgentType)
	}
	return config.Restore()
}

// collectCodexRuleModels returns the request_models of every active rule under
// the Codex scenario, deduplicated and in declaration order.
func (aa *AgentApply) collectCodexRuleModels() []string {
	seen := map[string]struct{}{}
	var out []string
	for _, rule := range aa.config.GetRequestConfigs() {
		if rule.GetScenario() != typ.ScenarioCodex || !rule.Active {
			continue
		}
		model := strings.TrimSpace(rule.RequestModel)
		if model == "" {
			continue
		}
		if _, dup := seen[model]; dup {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

// getBaseURLAndToken returns the base URL and API token for configuration
func (aa *AgentApply) getBaseURLAndToken() (string, string) {
	port := aa.config.ServerPort
	if port == 0 {
		port = 12580
	}
	baseURL := fmt.Sprintf("http://%s:%d", aa.host, port)
	apiKey := aa.config.GetModelToken()
	return baseURL, apiKey
}

// buildResultMessage builds a human-readable result message
func (aa *AgentApply) buildResultMessage(result *ApplyAgentResult) string {
	if !result.Success {
		return "Configuration application failed"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Configuration applied for %s\n", result.AgentType))
	if result.ProviderName != "" {
		sb.WriteString(fmt.Sprintf("Provider: %s\n", result.ProviderName))
	}
	if result.Model != "" {
		sb.WriteString(fmt.Sprintf("Model: %s\n", result.Model))
	}

	if len(result.ConfigFiles) > 0 {
		sb.WriteString("\nFiles modified:\n")
		for _, f := range result.ConfigFiles {
			sb.WriteString(fmt.Sprintf("  - %s\n", f))
		}
	}

	if result.RulesCreated > 0 {
		sb.WriteString(fmt.Sprintf("\nRouting rules created: %d\n", result.RulesCreated))
	}
	if result.RulesUpdated > 0 {
		sb.WriteString(fmt.Sprintf("Routing rules updated: %d\n", result.RulesUpdated))
	}

	if len(result.BackupPaths) > 0 {
		sb.WriteString("\nBackups:\n")
		for _, p := range result.BackupPaths {
			sb.WriteString(fmt.Sprintf("  - %s\n", p))
		}
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\nWarnings:\n")
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return sb.String()
}
```

---

## 五、各 Agent 最终生成的配置示例

### 5.1 Claude Code — `~/.claude/settings.json`

```json
{
  "env": {
    "DISABLE_TELEMETRY": "1",
    "DISABLE_ERROR_REPORTING": "1",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
    "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "32000",
    "API_TIMEOUT_MS": "3000000",
    "ANTHROPIC_BASE_URL": "http://localhost:12580/tingly/claude_code",
    "ANTHROPIC_AUTH_TOKEN": "test-token",
    "ANTHROPIC_MODEL": "tingly/cc",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "tingly/cc",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "tingly/cc",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "tingly/cc",
    "CLAUDE_CODE_SUBAGENT_MODEL": "tingly/cc"
  }
}
```

**Separate 模式差异**：所有 `"tingly/cc"` 分别替换为 `tingly/cc-default`、`tingly/cc-haiku`、`tingly/cc-opus`、`tingly/cc-sonnet`、`tingly/cc-subagent`。

### 5.2 OpenCode — `~/.config/opencode/opencode.json`

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "tingly-box": {
      "name": "tingly-box",
      "npm": "@ai-sdk/anthropic",
      "options": {
        "baseURL": "http://localhost:12580/tingly/opencode",
        "apiKey": "tok"
      },
      "models": {
        "tingly-opencode": {
          "name": "tingly-opencode"
        }
      }
    }
  }
}
```

### 5.3 Codex — `~/.codex/config.toml`

```toml
baseURL = "http://localhost:12580/tingly/codex"
models = ["tingly-codex"]
```

### 5.4 Codex — `~/.codex/auth.json`

```json
{
  "api_key": "tok"
}
```

---

## 六、关键设计模式总结

| 模式 | 说明 | 所在文件 |
|---|---|---|
| **Registry + Interface** | 统一接口，新增 Agent 只需注册实现 | `ai/agent/interface.go` |
| **统一请求 + 专有参数** | `ApplyAgentRequest` 统一入口，每种 Agent 有自己独立的 `Params` | `ai/agent/types.go` + 各 `*_params.go` |
| **虚拟模型名** | Agent 请求 `tingly/cc`，中转站内部映射到真实模型 | `internal/agent/builder.go` |
| **配置与路由联动** | 写本地配置文件的同时，在系统内建立路由规则 | `internal/agent/rule_bridge.go` |
| **备份恢复** | 每次 Apply 都备份原文件，支持 Restore | `ai/agent/restore.go` |
| **Warning 不阻断** | 路由规则失败只记 Warning，不中断配置流程 | `internal/agent/rule_bridge.go` |

---

## 七、如果要自建类似系统，建议步骤

### Phase 1：核心适配层（可复用）
1. 定义 `AgentConfig` 接口：`Apply()` + `Restore()`
2. 定义 `Registry` 注册表
3. 实现第一个 Agent（建议从 Claude Code 开始，最复杂）
   - 研究该 Agent 的配置机制（环境变量？配置文件？）
   - 实现配置文件读写 + 备份机制
4. 写单元测试验证生成的配置结构正确

### Phase 2：业务编排层（与中转站结合）
5. 定义 `AgentApply` 结构体，注入中转站的配置管理器
6. 实现 `createOrUpdateRules`：将虚拟模型名映射到真实 Provider+Model
7. 实现 `ApplyAgent` 主流程：先写配置 → 再建路由规则
8. 处理错误和回滚（配置写成功了但路由规则失败怎么办？）

### Phase 3：扩展
9. 新增 Agent 时，只需：
   - 在 `ai/agent` 下新建 `xxx_config.go`
   - 实现 `AgentConfig` 接口
   - 在 Registry `init()` 中注册
   - 在业务层新增对应的 `createOrUpdateXxxRules`

---

## 八、目录结构参考

```
ai/agent/                    # 核心适配层（可复用）
  interface.go               # AgentConfig 接口 + Registry
  types.go                   # 统一数据结构
  claude_code.go             # Claude Code 配置实现
  opencode.go                # OpenCode 配置实现
  codex.go                   # Codex 配置实现
  restore.go                 # 备份恢复逻辑
  utils.go                   # 辅助函数

internal/agent/              # 业务编排层（与中转站紧耦合）
  rule_bridge.go             # AgentApply 主入口
  builder.go                 # 参数构造辅助函数
  rule.go                    # 路由规则 CRUD
  apply_test.go              # 测试
```

---

## 九、一句话总结

> **"用一个干净的适配层欺骗 Agent，用一个业务编排层绑定路由，用虚拟模型名解耦上下游。"**
