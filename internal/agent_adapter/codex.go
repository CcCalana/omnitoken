package agent_adapter

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultCodexModel       = "chat-balanced"
	codexProviderName       = "omnitoken"
	codexProviderTitle      = "OmniToken"
	codexOpenAIAPIKey       = "OPENAI_API_KEY"
	codexPreferredAuth      = "apikey"
	codexCredentialsStore   = "file"
	codexWireAPI            = "chat"
	codexConfigFile         = "config.toml"
	codexAuthFile           = "auth.json"
	codexProviderHeader     = "[model_providers.omnitoken]"
	codexUnsupportedInline  = "unsupported config style"
	codexCredentialsWarning = "cli_auth_credentials_store: %s -> file (OmniToken-managed)"
	codexRequiresAuthWarn   = "requires_openai_auth: %s -> true (OmniToken-managed)"
)

var ErrInvalidExistingCodexConfig = errors.New("invalid existing Codex config")

type CodexOptions = BaseOptions

type RestoreCodexOptions = BaseRestoreOptions

type CodexConfig struct{}

var managedCodexTomlKeys = []string{
	"model",
	"model_provider",
	"preferred_auth_method",
	"cli_auth_credentials_store",
	"model_providers.omnitoken.name",
	"model_providers.omnitoken.base_url",
	"model_providers.omnitoken.env_key",
	"model_providers.omnitoken.wire_api",
	"model_providers.omnitoken.requires_openai_auth",
}

var managedCodexEnvKeys = []string{
	codexOpenAIAPIKey,
}

func ManagedCodexTomlKeys() []string {
	return append([]string(nil), managedCodexTomlKeys...)
}

func ManagedCodexEnvKeys() []string {
	return append([]string(nil), managedCodexEnvKeys...)
}

func WriteCodexSettings(opts CodexOptions) (Result, error) {
	return (&CodexConfig{}).Write(opts)
}

func (CodexConfig) Type() AgentType {
	return AgentTypeCodex
}

func (CodexConfig) Write(opts BaseOptions) (Result, error) {
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
	configPath := codexConfigPath(home)
	authPath := codexAuthPath(home)
	backupDir := codexBackupDir(home)

	configInput, configExisted, err := readOptionalText(configPath)
	if err != nil {
		return Result{}, fmt.Errorf("read Codex config: %w", err)
	}
	managed := buildCodexManagedConfig(opts)
	configOutput, warnings, err := patchCodexConfig(configInput, managed)
	if err != nil {
		return Result{}, err
	}

	authRoot, authExisted, err := readJSONObject(authPath, "Codex auth")
	if err != nil {
		if errors.Is(err, ErrInvalidExistingConfig) {
			return Result{}, fmt.Errorf("%w: auth.json: %v", ErrInvalidExistingCodexConfig, err)
		}
		return Result{}, err
	}
	authRoot[codexOpenAIAPIKey] = strings.TrimSpace(opts.Token)

	var backupPaths []string
	if configExisted || authExisted {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return Result{}, fmt.Errorf("create backup dir: %w", err)
		}
		now := nowUTC(opts.Now)
		if configExisted {
			backupPath, err := uniqueBackupPath(backupDir, codexConfigFile, now)
			if err != nil {
				return Result{}, err
			}
			if err := copyFile(configPath, backupPath); err != nil {
				return Result{}, fmt.Errorf("backup Codex config: %w", err)
			}
			backupPaths = append(backupPaths, backupPath)
		}
		if authExisted {
			backupPath, err := uniqueBackupPath(backupDir, codexAuthFile, now)
			if err != nil {
				return Result{}, err
			}
			if err := copyFile(authPath, backupPath); err != nil {
				return Result{}, fmt.Errorf("backup Codex auth: %w", err)
			}
			backupPaths = append(backupPaths, backupPath)
		}
	}

	if err := writeAtomic(configPath, []byte(configOutput), 0o600); err != nil {
		return Result{}, fmt.Errorf("write Codex config: %w", err)
	}
	if err := writeJSONFile(authPath, authRoot); err != nil {
		return Result{}, fmt.Errorf("write Codex auth: %w", err)
	}

	managedKeys := append(ManagedCodexEnvKeys(), ManagedCodexTomlKeys()...)
	return Result{
		Paths:           paths(map[string]string{"config": configPath, "auth": authPath}),
		BackupPaths:     backupPaths,
		ManagedKeys:     managedKeys,
		Warnings:        warnings,
		ConfigPath:      configPath,
		AuthPath:        authPath,
		SettingsPath:    configPath,
		BackupPath:      firstString(backupPaths),
		ManagedEnvKeys:  ManagedCodexEnvKeys(),
		ManagedTomlKeys: ManagedCodexTomlKeys(),
	}, nil
}

func RestoreCodexSettingsWithOptions(opts RestoreCodexOptions) (Result, error) {
	return (&CodexConfig{}).Restore(opts)
}

func (CodexConfig) Restore(opts BaseRestoreOptions) (Result, error) {
	home, err := resolveHome(opts.Home)
	if err != nil {
		return Result{}, err
	}
	configPath := codexConfigPath(home)
	authPath := codexAuthPath(home)
	backupDir := codexBackupDir(home)

	configBackup, configErr := latestNamedBackupPath(backupDir, codexConfigFile)
	authBackup, authErr := latestNamedBackupPath(backupDir, codexAuthFile)
	if configErr != nil && authErr != nil {
		if errors.Is(configErr, errNoBackupFound) && errors.Is(authErr, errNoBackupFound) {
			return Result{}, errNoBackupFound
		}
		return Result{}, fmt.Errorf("find Codex backups: config=%v auth=%v", configErr, authErr)
	}
	if configErr != nil && !errors.Is(configErr, errNoBackupFound) {
		return Result{}, fmt.Errorf("find Codex config backup: %w", configErr)
	}
	if authErr != nil && !errors.Is(authErr, errNoBackupFound) {
		return Result{}, fmt.Errorf("find Codex auth backup: %w", authErr)
	}

	var restored []string
	if configErr == nil {
		if err := copyFile(configBackup, configPath); err != nil {
			return Result{}, fmt.Errorf("restore Codex config: %w", err)
		}
		restored = append(restored, configBackup)
	}
	if authErr == nil {
		if err := copyFile(authBackup, authPath); err != nil {
			return Result{}, fmt.Errorf("restore Codex auth: %w", err)
		}
		restored = append(restored, authBackup)
	}

	managedKeys := append(ManagedCodexEnvKeys(), ManagedCodexTomlKeys()...)
	return Result{
		Paths:             paths(map[string]string{"config": configPath, "auth": authPath}),
		RestoredFromPaths: restored,
		ManagedKeys:       managedKeys,
		ConfigPath:        configPath,
		AuthPath:          authPath,
		SettingsPath:      configPath,
		RestoredFrom:      firstString(restored),
		ManagedEnvKeys:    ManagedCodexEnvKeys(),
		ManagedTomlKeys:   ManagedCodexTomlKeys(),
	}, nil
}

type codexManagedConfig struct {
	Model               string
	BaseURL             string
	ProviderName        string
	ProviderTitle       string
	EnvKey              string
	WireAPI             string
	PreferredAuthMethod string
	CredentialsStore    string
	RequiresOpenAIAuth  bool
}

type codexConfigScan struct {
	topLevelManaged map[string]int
	providerKeys    map[string]codexValue
	providerStart   int
	providerEnd     int
	inlineProvider  bool
	warnings        []string
}

type codexValue struct {
	raw  string
	kind string
}

func buildCodexManagedConfig(opts CodexOptions) codexManagedConfig {
	model := strings.TrimSpace(opts.Model)
	if model == "" {
		model = defaultCodexModel
	}
	return codexManagedConfig{
		Model:               model,
		BaseURL:             strings.TrimRight(strings.TrimSpace(opts.GatewayURL), "/") + "/v1",
		ProviderName:        codexProviderName,
		ProviderTitle:       codexProviderTitle,
		EnvKey:              codexOpenAIAPIKey,
		WireAPI:             codexWireAPI,
		PreferredAuthMethod: codexPreferredAuth,
		CredentialsStore:    codexCredentialsStore,
		RequiresOpenAIAuth:  true,
	}
}

func patchCodexConfig(input string, managed codexManagedConfig) (string, []string, error) {
	scan, err := scanCodexConfig(input)
	if err != nil {
		return "", nil, err
	}
	if scan.inlineProvider {
		return "", nil, fmt.Errorf("%w: %s: inline model_providers table is not supported; run restore first", ErrInvalidExistingCodexConfig, codexUnsupportedInline)
	}

	lines := splitLines(input)
	for _, key := range []string{"model", "model_provider", "preferred_auth_method", "cli_auth_credentials_store"} {
		if indexes := managedKeyIndexes(scan.topLevelManaged, key); len(indexes) > 1 {
			return "", nil, fmt.Errorf("%w: duplicate top-level %s", ErrInvalidExistingCodexConfig, key)
		}
	}
	for _, key := range []string{"name", "base_url", "env_key", "wire_api", "requires_openai_auth"} {
		if value, ok := scan.providerKeys[key]; ok {
			if key == "requires_openai_auth" && value.kind != "bool" {
				return "", nil, fmt.Errorf("%w: %s must be a bool", ErrInvalidExistingCodexConfig, key)
			}
			if key != "requires_openai_auth" && value.kind != "string" {
				return "", nil, fmt.Errorf("%w: %s must be a string", ErrInvalidExistingCodexConfig, key)
			}
		}
	}

	scan.warnings = append(scan.warnings, codexManagedWarnings(scan.providerKeys, scan.topLevelManaged, lines)...)
	lines = setTopLevelString(lines, scan.topLevelManaged, "model", managed.Model)
	lines = setTopLevelString(lines, scan.topLevelManaged, "model_provider", managed.ProviderName)
	lines = setTopLevelString(lines, scan.topLevelManaged, "preferred_auth_method", managed.PreferredAuthMethod)
	lines = setTopLevelString(lines, scan.topLevelManaged, "cli_auth_credentials_store", managed.CredentialsStore)
	rescan, err := scanCodexConfig(joinLines(lines))
	if err != nil {
		return "", nil, err
	}
	lines = replaceCodexProviderBlock(lines, rescan.providerStart, rescan.providerEnd, managed)
	return joinLines(lines), scan.warnings, nil
}

func scanCodexConfig(input string) (codexConfigScan, error) {
	lines := splitLines(input)
	scan := codexConfigScan{
		topLevelManaged: map[string]int{},
		providerKeys:    map[string]codexValue{},
		providerStart:   -1,
		providerEnd:     -1,
	}
	section := ""
	inMultiline := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if inMultiline {
			if closesTripleString(trimmed) {
				inMultiline = false
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			if !strings.HasSuffix(trimmed, "]") || strings.Count(trimmed, "[") != strings.Count(trimmed, "]") {
				return scan, fmt.Errorf("%w: malformed table header on line %d", ErrInvalidExistingCodexConfig, i+1)
			}
			section = strings.TrimSpace(strings.Trim(trimmed, "[]"))
			if section == "model_providers.omnitoken" {
				scan.providerStart = i
			} else if scan.providerStart >= 0 && scan.providerEnd < 0 {
				scan.providerEnd = i
			}
			continue
		}

		key, value, ok := splitTomlAssignment(trimmed)
		if !ok {
			continue
		}
		if key == "model_providers" && section == "" && strings.HasPrefix(strings.TrimSpace(value), "{") {
			scan.inlineProvider = true
		}
		if startsTripleString(value) {
			if !closesTripleString(strings.TrimSpace(value[3:])) {
				inMultiline = true
			}
			continue
		}
		if hasUnterminatedString(value) {
			return scan, fmt.Errorf("%w: unterminated string on line %d", ErrInvalidExistingCodexConfig, i+1)
		}
		valueKind := tomlValueKind(value)
		if section == "" {
			if isManagedTopLevelCodexKey(key) {
				if _, exists := scan.topLevelManaged[key]; exists {
					return scan, fmt.Errorf("%w: duplicate top-level %s", ErrInvalidExistingCodexConfig, key)
				}
				scan.topLevelManaged[key] = i
			}
			continue
		}
		if section == "model_providers.omnitoken" {
			if _, exists := scan.providerKeys[key]; exists {
				return scan, fmt.Errorf("%w: duplicate provider key %s", ErrInvalidExistingCodexConfig, key)
			}
			scan.providerKeys[key] = codexValue{raw: strings.TrimSpace(value), kind: valueKind}
		}
	}
	if inMultiline {
		return scan, fmt.Errorf("%w: unterminated multiline string", ErrInvalidExistingCodexConfig)
	}
	if scan.providerStart >= 0 && scan.providerEnd < 0 {
		scan.providerEnd = len(lines)
	}
	return scan, nil
}

func codexManagedWarnings(providerKeys map[string]codexValue, topLevelManaged map[string]int, lines []string) []string {
	var warnings []string
	if index, ok := topLevelManaged["cli_auth_credentials_store"]; ok {
		_, value, _ := splitTomlAssignment(strings.TrimSpace(lines[index]))
		old := trimTomlString(value)
		if old != "" && old != codexCredentialsStore {
			warnings = append(warnings, fmt.Sprintf(codexCredentialsWarning, old))
		}
	}
	if value, ok := providerKeys["requires_openai_auth"]; ok && strings.TrimSpace(value.raw) != "true" {
		warnings = append(warnings, fmt.Sprintf(codexRequiresAuthWarn, strings.TrimSpace(value.raw)))
	}
	return warnings
}

func setTopLevelString(lines []string, indexes map[string]int, key string, value string) []string {
	line := fmt.Sprintf("%s = %s", key, quoteTomlString(value))
	if index, ok := indexes[key]; ok {
		lines[index] = line
		return lines
	}
	insertAt := firstTableIndex(lines)
	if insertAt < 0 {
		insertAt = len(lines)
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, line)
	out = append(out, lines[insertAt:]...)
	shiftIndexes(indexes, insertAt)
	indexes[key] = insertAt
	return out
}

func replaceCodexProviderBlock(lines []string, start int, end int, managed codexManagedConfig) []string {
	block := []string{
		codexProviderHeader,
		"name = " + quoteTomlString(managed.ProviderTitle),
		"base_url = " + quoteTomlString(managed.BaseURL),
		"env_key = " + quoteTomlString(managed.EnvKey),
		"wire_api = " + quoteTomlString(managed.WireAPI),
		"requires_openai_auth = true",
	}
	if start >= 0 {
		out := append([]string{}, lines[:start]...)
		out = append(out, block...)
		out = append(out, lines[end:]...)
		return out
	}
	out := append([]string{}, lines...)
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, block...)
	return out
}

func splitLines(input string) []string {
	if input == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(input, "\n")
	return strings.Split(trimmed, "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func splitTomlAssignment(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := strings.TrimSpace(stripTomlComment(line[idx+1:]))
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func stripTomlComment(value string) string {
	inString := false
	inLiteral := false
	escaped := false
	for i, r := range value {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString && !inLiteral:
			escaped = true
		case r == '"' && !inLiteral:
			inString = !inString
		case r == '\'' && !inString:
			inLiteral = !inLiteral
		case r == '#' && !inString && !inLiteral:
			return value[:i]
		}
	}
	return value
}

func startsTripleString(value string) bool {
	return strings.HasPrefix(strings.TrimSpace(value), `"""`)
}

func closesTripleString(value string) bool {
	return strings.Contains(value, `"""`)
}

func hasUnterminatedString(value string) bool {
	inString := false
	inLiteral := false
	escaped := false
	for _, r := range value {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && inString && !inLiteral:
			escaped = true
		case r == '"' && !inLiteral:
			inString = !inString
		case r == '\'' && !inString:
			inLiteral = !inLiteral
		}
	}
	return inString || inLiteral
}

func tomlValueKind(value string) string {
	trimmed := strings.TrimSpace(value)
	if ((strings.HasPrefix(trimmed, `"`) && strings.HasSuffix(trimmed, `"`)) ||
		(strings.HasPrefix(trimmed, `'`) && strings.HasSuffix(trimmed, `'`))) &&
		!hasUnterminatedString(trimmed) {
		return "string"
	}
	if trimmed == "true" || trimmed == "false" {
		return "bool"
	}
	return "other"
}

func trimTomlString(value string) string {
	trimmed := strings.TrimSpace(stripTomlComment(value))
	if unquoted, err := strconv.Unquote(trimmed); err == nil {
		return unquoted
	}
	if strings.HasPrefix(trimmed, "'") && strings.HasSuffix(trimmed, "'") && len(trimmed) >= 2 {
		return strings.TrimSuffix(strings.TrimPrefix(trimmed, "'"), "'")
	}
	return trimmed
}

func quoteTomlString(value string) string {
	return strconv.Quote(value)
}

func isManagedTopLevelCodexKey(key string) bool {
	switch key {
	case "model", "model_provider", "preferred_auth_method", "cli_auth_credentials_store":
		return true
	default:
		return false
	}
}

func firstTableIndex(lines []string) int {
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "[") {
			return i
		}
	}
	return -1
}

func shiftIndexes(indexes map[string]int, insertAt int) {
	for key, index := range indexes {
		if index >= insertAt {
			indexes[key] = index + 1
		}
	}
}

func managedKeyIndexes(indexes map[string]int, key string) []int {
	if index, ok := indexes[key]; ok {
		return []int{index}
	}
	return nil
}

func readOptionalText(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

func codexConfigPath(home string) string {
	return filepath.Join(home, ".codex", codexConfigFile)
}

func codexAuthPath(home string) string {
	return filepath.Join(home, ".codex", codexAuthFile)
}

func codexBackupDir(home string) string {
	return filepath.Join(home, ".omnitoken", "backups", "codex")
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	return sorted[0]
}
