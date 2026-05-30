package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omnitoken/omnitoken/internal/agent_adapter"
)

const defaultModel = "chat-balanced"
const adminClientTimeout = 10 * time.Second
const defaultAdminURL = "http://localhost:8081"

type ensureModelOptions struct {
	AdminURL    string
	RealModel   string
	Provider    string
	EnsureModel bool
}

var (
	cliStdin = io.Reader(os.Stdin)
	inputTTY = func() bool {
		info, err := os.Stdin.Stat()
		return err == nil && (info.Mode()&os.ModeCharDevice) != 0
	}
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "adopt":
		if len(args) < 2 {
			printUsage(stderr)
			return 2
		}
		switch args[1] {
		case "claude-code":
			return runAdoptClaudeCode(args[2:], stdout, stderr)
		case "codex":
			return runAdoptCodex(args[2:], stdout, stderr)
		case "opencode":
			return runAdoptOpenCode(args[2:], stdout, stderr)
		default:
			fmt.Fprintf(stderr, "unsupported adopt target %q\n", args[1])
			return 2
		}
	case "restore":
		if len(args) < 2 {
			printUsage(stderr)
			return 2
		}
		switch args[1] {
		case "claude-code":
			return runRestoreClaudeCode(args[2:], stdout, stderr)
		case "codex":
			return runRestoreCodex(args[2:], stdout, stderr)
		case "opencode":
			return runRestoreOpenCode(args[2:], stdout, stderr)
		default:
			fmt.Fprintf(stderr, "unsupported restore target %q\n", args[1])
			return 2
		}
	case "status":
		return runStatus(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func runAdoptClaudeCode(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.ClaudeCodeOptions
	fs := flag.NewFlagSet("adopt claude-code", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.GatewayURL, "gateway-url", "", "OmniToken gateway URL")
	fs.StringVar(&opts.Token, "token", "", "OmniToken virtual key")
	fs.StringVar(&opts.Model, "model", defaultModel, "virtual model name")
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	dryRun := fs.Bool("dry-run", false, "validate and show planned writes without changing files")
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := collectAdoptInputs("claude-code", &opts.GatewayURL, &opts.Token, &opts.Model, ensure, stdout); err != nil {
		fmt.Fprintf(stderr, "adopt claude-code: %v\n", err)
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt claude-code: %v\n", polishAdoptError(err))
		return 1
	}
	if *dryRun {
		printDryRun(stdout, "claude-code", expectedClaudeCodePaths(opts.Home), opts.GatewayURL, opts.Model)
		return 0
	}

	result, err := agent_adapter.WriteClaudeCodeSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt claude-code: %v\n", polishAdoptError(err))
		if errors.Is(err, agent_adapter.ErrInvalidExistingConfig) {
			return 2
		}
		return 1
	}

	printAdoptSummary(stdout, "claude-code", []string{result.SettingsPath}, opts.GatewayURL, opts.Model, compactNonEmpty([]string{result.BackupPath}), nil)
	return 0
}

func runAdoptCodex(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.CodexOptions
	fs := flag.NewFlagSet("adopt codex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.GatewayURL, "gateway-url", "", "OmniToken gateway URL")
	fs.StringVar(&opts.Token, "token", "", "OmniToken virtual key")
	fs.StringVar(&opts.Model, "model", defaultModel, "virtual model name")
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	dryRun := fs.Bool("dry-run", false, "validate and show planned writes without changing files")
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := collectAdoptInputs("codex", &opts.GatewayURL, &opts.Token, &opts.Model, ensure, stdout); err != nil {
		fmt.Fprintf(stderr, "adopt codex: %v\n", err)
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt codex: %v\n", polishAdoptError(err))
		return 1
	}
	if *dryRun {
		printDryRun(stdout, "codex", expectedCodexPaths(opts.Home), opts.GatewayURL, opts.Model)
		return 0
	}

	result, err := agent_adapter.WriteCodexSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt codex: %v\n", polishAdoptError(err))
		if errors.Is(err, agent_adapter.ErrInvalidExistingCodexConfig) {
			return 2
		}
		return 1
	}

	printAdoptSummary(stdout, "codex", []string{result.ConfigPath, result.AuthPath}, opts.GatewayURL, opts.Model, result.BackupPaths, result.Warnings)
	return 0
}

func runAdoptOpenCode(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.OpenCodeOptions
	fs := flag.NewFlagSet("adopt opencode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.GatewayURL, "gateway-url", "", "OmniToken gateway URL")
	fs.StringVar(&opts.Token, "token", "", "OmniToken virtual key")
	fs.StringVar(&opts.Model, "model", defaultModel, "virtual model name")
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	dryRun := fs.Bool("dry-run", false, "validate and show planned writes without changing files")
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := collectAdoptInputs("opencode", &opts.GatewayURL, &opts.Token, &opts.Model, ensure, stdout); err != nil {
		fmt.Fprintf(stderr, "adopt opencode: %v\n", err)
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt opencode: %v\n", polishAdoptError(err))
		return 1
	}
	if *dryRun {
		printDryRun(stdout, "opencode", expectedOpenCodePaths(opts.Home), opts.GatewayURL, opts.Model)
		return 0
	}

	result, err := agent_adapter.WriteOpenCodeSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt opencode: %v\n", polishAdoptError(err))
		if errors.Is(err, agent_adapter.ErrInvalidExistingOpenCodeConfig) {
			return 2
		}
		return 1
	}

	printAdoptSummary(stdout, "opencode", []string{result.ConfigPath}, opts.GatewayURL, opts.Model, compactNonEmpty([]string{result.BackupPath}), nil)
	return 0
}

func printAdoptSummary(stdout io.Writer, agent string, paths []string, gatewayURL string, model string, backups []string, warnings []string) {
	fmt.Fprintf(stdout, "✓ %s configured\n", agent)
	fmt.Fprintf(stdout, "config %s\n", strings.Join(paths, ", "))
	fmt.Fprintf(stdout, "gateway %s\n", gatewayURL)
	fmt.Fprintf(stdout, "model %s\n", model)
	if len(backups) > 0 {
		fmt.Fprintf(stdout, "backup %s\n", strings.Join(backups, ", "))
	} else if len(warnings) > 0 {
		fmt.Fprintf(stdout, "WARN %s\n", strings.Join(warnings, "; "))
	}
}

func runRestoreClaudeCode(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.RestoreClaudeCodeOptions
	fs := flag.NewFlagSet("restore claude-code", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !confirmRestore(stdout, "claude-code", expectedClaudeCodePaths(opts.Home)) {
		fmt.Fprintln(stdout, "restore claude-code: cancelled")
		return 1
	}

	result, err := agent_adapter.RestoreClaudeCodeSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore claude-code: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "✓ 已恢复原始配置")
	fmt.Fprintf(stdout, "files %s\n", result.SettingsPath)
	fmt.Fprintf(stdout, "from %s\n", result.RestoredFrom)
	return 0
}

func runRestoreCodex(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.RestoreCodexOptions
	fs := flag.NewFlagSet("restore codex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !confirmRestore(stdout, "codex", expectedCodexPaths(opts.Home)) {
		fmt.Fprintln(stdout, "restore codex: cancelled")
		return 1
	}

	result, err := agent_adapter.RestoreCodexSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore codex: %v\n", err)
		return 1
	}
	restoredFiles := make([]string, 0, 2)
	for _, path := range []string{result.ConfigPath, result.AuthPath} {
		if path != "" {
			restoredFiles = append(restoredFiles, path)
		}
	}
	fmt.Fprintln(stdout, "✓ 已恢复原始配置")
	fmt.Fprintf(stdout, "files %s\n", strings.Join(restoredFiles, ", "))
	for _, restoredFrom := range result.RestoredFromPaths {
		fmt.Fprintf(stdout, "from %s\n", restoredFrom)
	}
	return 0
}

func runRestoreOpenCode(args []string, stdout io.Writer, stderr io.Writer) int {
	var opts agent_adapter.RestoreOpenCodeOptions
	fs := flag.NewFlagSet("restore opencode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.Home, "home", "", "override home directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if !confirmRestore(stdout, "opencode", expectedOpenCodePaths(opts.Home)) {
		fmt.Fprintln(stdout, "restore opencode: cancelled")
		return 1
	}

	result, err := agent_adapter.RestoreOpenCodeSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore opencode: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "✓ 已恢复原始配置")
	fmt.Fprintf(stdout, "files %s\n", result.ConfigPath)
	fmt.Fprintf(stdout, "from %s\n", result.RestoredFrom)
	return 0
}

func collectAdoptInputs(agent string, gatewayURL *string, token *string, model *string, ensure *ensureModelOptions, stdout io.Writer) error {
	*gatewayURL = strings.TrimSpace(*gatewayURL)
	*token = strings.TrimSpace(*token)
	*model = strings.TrimSpace(*model)
	if *model == "" {
		*model = defaultModel
	}
	if *gatewayURL != "" && *token != "" && (!inputTTY() || !ensureNeedsPrompt(ensure)) {
		return nil
	}
	if !inputTTY() {
		if *gatewayURL == "" {
			return fmt.Errorf("--gateway-url is required when stdin is not a TTY")
		}
		return fmt.Errorf("--token is required when stdin is not a TTY")
	}

	reader := bufio.NewScanner(cliStdin)
	if *gatewayURL == "" {
		value, err := promptLine(reader, stdout, "Gateway URL: ")
		if err != nil {
			return err
		}
		*gatewayURL = strings.TrimSpace(value)
	}
	if *token == "" {
		value, err := promptLine(reader, stdout, "Virtual key token: ")
		if err != nil {
			return err
		}
		*token = strings.TrimSpace(value)
	}
	if *model == defaultModel {
		value, err := promptLine(reader, stdout, fmt.Sprintf("Model [%s]: ", defaultModel))
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) != "" {
			*model = strings.TrimSpace(value)
		}
	}
	if ensure != nil && ensure.EnsureModel && strings.TrimSpace(ensure.AdminURL) == "" {
		value, err := promptLine(reader, stdout, fmt.Sprintf("Admin URL for virtual model ensure [skip/%s]: ", defaultAdminURL))
		if err != nil {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" || strings.EqualFold(value, "skip") {
			ensure.EnsureModel = false
		} else {
			ensure.AdminURL = value
		}
	}
	if ensureRequested(ensure) {
		if strings.TrimSpace(ensure.RealModel) == "" {
			value, err := promptLine(reader, stdout, "Real model: ")
			if err != nil {
				return err
			}
			ensure.RealModel = strings.TrimSpace(value)
		}
		if strings.TrimSpace(ensure.Provider) == "" {
			value, err := promptLine(reader, stdout, "Provider [ark/deepseek]: ")
			if err != nil {
				return err
			}
			ensure.Provider = strings.TrimSpace(value)
		}
	}
	if *gatewayURL == "" {
		return fmt.Errorf("gateway URL is required for %s", agent)
	}
	if *token == "" {
		return fmt.Errorf("token is required for %s", agent)
	}
	return nil
}

func promptLine(scanner *bufio.Scanner, stdout io.Writer, prompt string) (string, error) {
	fmt.Fprint(stdout, prompt)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", io.ErrUnexpectedEOF
	}
	return scanner.Text(), nil
}

func confirmRestore(stdout io.Writer, agent string, paths []string) bool {
	if !inputTTY() {
		return true
	}
	reader := bufio.NewScanner(cliStdin)
	answer, err := promptLine(reader, stdout, fmt.Sprintf("将恢复 %s (%s)，继续？[y/N] ", agent, strings.Join(paths, ", ")))
	if err != nil {
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func ensureRequested(opts *ensureModelOptions) bool {
	return opts != nil && opts.EnsureModel && strings.TrimSpace(opts.AdminURL) != ""
}

func ensureNeedsPrompt(opts *ensureModelOptions) bool {
	return ensureRequested(opts) && (strings.TrimSpace(opts.RealModel) == "" || strings.TrimSpace(opts.Provider) == "")
}

func printDryRun(stdout io.Writer, agent string, paths []string, gatewayURL string, model string) {
	fmt.Fprintf(stdout, "✓ Dry run: %s\n", agent)
	fmt.Fprintf(stdout, "Would write settings to %s\n", strings.Join(paths, ", "))
	fmt.Fprintf(stdout, "gateway %s\n", gatewayURL)
	fmt.Fprintf(stdout, "model %s\n", model)
}

func compactNonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	target, home, err := parseStatusArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 2
	}
	agents := []string{"claude-code", "codex", "opencode"}
	if target != "" {
		agents = []string{target}
	}
	for _, agent := range agents {
		info := readAgentStatus(agent, home)
		if !info.Configured {
			fmt.Fprintf(stdout, "%s: 未配置\n", agent)
			continue
		}
		fmt.Fprintf(stdout, "%s: 已配置 gateway=%s model=%s token=%s\n", agent, info.GatewayURL, info.Model, tokenPrefix(info.Token))
	}
	return 0
}

func parseStatusArgs(args []string) (string, string, error) {
	var target, home string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--home":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--home requires a value")
			}
			home = args[i+1]
			i++
		case "-h", "--help":
			return "", "", flag.ErrHelp
		default:
			if strings.HasPrefix(args[i], "-") {
				return "", "", fmt.Errorf("unknown flag %s", args[i])
			}
			if target != "" {
				return "", "", fmt.Errorf("unexpected argument %q", args[i])
			}
			target = args[i]
		}
	}
	if target != "" && target != "claude-code" && target != "codex" && target != "opencode" {
		return "", "", fmt.Errorf("unsupported status target %q", target)
	}
	return target, home, nil
}

type agentStatus struct {
	Configured bool
	GatewayURL string
	Model      string
	Token      string
}

func readAgentStatus(agent string, homeOverride string) agentStatus {
	switch agent {
	case "claude-code":
		return readClaudeCodeStatus(expectedClaudeCodePaths(homeOverride)[0])
	case "codex":
		paths := expectedCodexPaths(homeOverride)
		return readCodexStatus(paths[0], paths[1])
	case "opencode":
		return readOpenCodeStatus(expectedOpenCodePaths(homeOverride)[0])
	default:
		return agentStatus{}
	}
}

func readClaudeCodeStatus(path string) agentStatus {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentStatus{}
	}
	var body struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return agentStatus{}
	}
	return completeStatus(agentStatus{
		GatewayURL: body.Env["ANTHROPIC_BASE_URL"],
		Model:      body.Env["ANTHROPIC_MODEL"],
		Token:      body.Env["ANTHROPIC_AUTH_TOKEN"],
	})
}

func readCodexStatus(configPath string, authPath string) agentStatus {
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return agentStatus{}
	}
	authData, err := os.ReadFile(authPath)
	if err != nil {
		return agentStatus{}
	}
	var auth map[string]string
	if err := json.Unmarshal(authData, &auth); err != nil {
		return agentStatus{}
	}
	config := parseSimpleTOMLValues(string(configData))
	return completeStatus(agentStatus{
		GatewayURL: config["base_url"],
		Model:      config["model"],
		Token:      auth["OPENAI_API_KEY"],
	})
}

func readOpenCodeStatus(path string) agentStatus {
	data, err := os.ReadFile(path)
	if err != nil {
		return agentStatus{}
	}
	var body struct {
		Provider map[string]struct {
			Options map[string]string `json:"options"`
			Models  map[string]any    `json:"models"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return agentStatus{}
	}
	provider, ok := body.Provider["omnitoken"]
	if !ok {
		return agentStatus{}
	}
	model := ""
	for name := range provider.Models {
		model = name
		break
	}
	return completeStatus(agentStatus{
		GatewayURL: provider.Options["baseURL"],
		Model:      model,
		Token:      provider.Options["apiKey"],
	})
}

func completeStatus(status agentStatus) agentStatus {
	status.GatewayURL = strings.TrimSpace(status.GatewayURL)
	status.Model = strings.TrimSpace(status.Model)
	status.Token = strings.TrimSpace(status.Token)
	status.Configured = status.GatewayURL != "" && status.Model != "" && status.Token != ""
	return status
}

func parseSimpleTOMLValues(data string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"`)
		if key == "model" || key == "base_url" {
			values[key] = value
		}
	}
	return values
}

func tokenPrefix(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return "未配置"
	}
	if len(token) <= 8 {
		return token + "..."
	}
	return token[:8] + "..."
}

func expectedClaudeCodePaths(homeOverride string) []string {
	home := resolveHome(homeOverride)
	return []string{filepath.Join(home, ".claude", "settings.json")}
}

func expectedCodexPaths(homeOverride string) []string {
	home := resolveHome(homeOverride)
	return []string{
		filepath.Join(home, ".codex", "config.toml"),
		filepath.Join(home, ".codex", "auth.json"),
	}
}

func expectedOpenCodePaths(homeOverride string) []string {
	if strings.TrimSpace(homeOverride) != "" {
		return []string{filepath.Join(homeOverride, ".config", "opencode", "opencode.json")}
	}
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return []string{filepath.Join(xdg, "opencode", "opencode.json")}
	}
	return []string{filepath.Join(resolveHome(""), ".config", "opencode", "opencode.json")}
}

func resolveHome(homeOverride string) string {
	if strings.TrimSpace(homeOverride) != "" {
		return homeOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func polishAdoptError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "admin API returned 401"):
		return fmt.Errorf("%w: admin API 401，请确认 --token 是有效的 admin 角色虚拟 key", err)
	case strings.Contains(msg, "query virtual models:") || strings.Contains(msg, "create virtual model:"):
		if strings.Contains(msg, "admin API returned") {
			return err
		}
		return fmt.Errorf("%w: gateway unreachable，请检查 --admin-url 是否正确且 admin 服务已启动", err)
	case strings.Contains(msg, "exists with provider="):
		return fmt.Errorf("%w: virtual model conflict，请换用 --model 或在管理端修正路由规则", err)
	default:
		return err
	}
}

func registerEnsureModelFlags(fs *flag.FlagSet) *ensureModelOptions {
	opts := &ensureModelOptions{EnsureModel: true}
	fs.StringVar(&opts.AdminURL, "admin-url", "", "OmniToken admin API URL")
	fs.StringVar(&opts.RealModel, "real-model", "", "upstream real model for ensuring the virtual model")
	fs.StringVar(&opts.Provider, "provider", "", "upstream provider for ensuring the virtual model")
	fs.BoolVar(&opts.EnsureModel, "ensure-model", true, "ensure the virtual model exists when --admin-url is set")
	return opts
}

func ensureVirtualModelIfRequested(opts *ensureModelOptions, model string, token string) error {
	if opts == nil || strings.TrimSpace(opts.AdminURL) == "" || !opts.EnsureModel {
		return nil
	}
	params := ensureVirtualModelParams{
		Name:      strings.TrimSpace(model),
		RealModel: strings.TrimSpace(opts.RealModel),
		Provider:  strings.TrimSpace(opts.Provider),
		Token:     strings.TrimSpace(token),
	}
	if params.Name == "" {
		return fmt.Errorf("model is required for ensure-model")
	}
	if params.RealModel == "" || params.Provider == "" {
		return fmt.Errorf("--real-model and --provider are required when --admin-url is set")
	}
	if params.Provider != "ark" && params.Provider != "deepseek" {
		return fmt.Errorf("--provider must be ark or deepseek")
	}
	client := adminVirtualModelClient{
		BaseURL: strings.TrimRight(strings.TrimSpace(opts.AdminURL), "/"),
		Token:   params.Token,
		HTTP:    &http.Client{Timeout: adminClientTimeout},
	}
	return client.Ensure(context.Background(), params)
}

type ensureVirtualModelParams struct {
	Name      string
	RealModel string
	Provider  string
	Token     string
}

type adminVirtualModelClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

type virtualModelsResponse struct {
	VirtualModels []virtualModel `json:"virtual_models"`
}

type virtualModel struct {
	Name      string `json:"name"`
	RealModel string `json:"real_model"`
	Provider  string `json:"provider"`
}

func (c adminVirtualModelClient) Ensure(ctx context.Context, params ensureVirtualModelParams) error {
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: adminClientTimeout}
	}
	existing, found, err := c.find(ctx, params.Name)
	if err != nil {
		return err
	}
	if found {
		if existing.Provider != params.Provider || existing.RealModel != params.RealModel {
			return fmt.Errorf("virtual model %q exists with provider=%s real_model=%s", params.Name, existing.Provider, existing.RealModel)
		}
		return nil
	}
	return c.create(ctx, params)
}

func (c adminVirtualModelClient) find(ctx context.Context, name string) (virtualModel, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/api/admin/virtual-models", nil)
	if err != nil {
		return virtualModel{}, false, fmt.Errorf("build admin request: %w", err)
	}
	c.authorize(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return virtualModel{}, false, fmt.Errorf("query virtual models: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return virtualModel{}, false, fmt.Errorf("query virtual models: admin API returned %d", resp.StatusCode)
	}
	var body virtualModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return virtualModel{}, false, fmt.Errorf("decode virtual models: %w", err)
	}
	for _, item := range body.VirtualModels {
		if item.Name == name {
			return item, true, nil
		}
	}
	return virtualModel{}, false, nil
}

func (c adminVirtualModelClient) create(ctx context.Context, params ensureVirtualModelParams) error {
	payload, err := json.Marshal(map[string]string{
		"name":        params.Name,
		"real_model":  params.RealModel,
		"provider":    params.Provider,
		"description": "created by omnitoken-adopt",
	})
	if err != nil {
		return fmt.Errorf("encode virtual model: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/admin/virtual-models", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build admin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.authorize(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("create virtual model: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("create virtual model: admin API returned %d", resp.StatusCode)
	}
	return nil
}

func (c adminVirtualModelClient) authorize(req *http.Request) {
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	req.Header.Set("Accept", "application/json")
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  omnitoken-adopt adopt claude-code --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt adopt codex --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt adopt opencode --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore claude-code [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore codex [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore opencode [--home <path>]")
}
