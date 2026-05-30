package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/omnitoken/omnitoken/internal/agent_adapter"
)

const defaultModel = "chat-balanced"
const adminClientTimeout = 10 * time.Second

type ensureModelOptions struct {
	AdminURL    string
	RealModel   string
	Provider    string
	EnsureModel bool
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "adopt":
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
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt claude-code: %v\n", err)
		return 1
	}

	result, err := agent_adapter.WriteClaudeCodeSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt claude-code: %v\n", err)
		if errors.Is(err, agent_adapter.ErrInvalidExistingConfig) {
			return 2
		}
		return 1
	}

	fmt.Fprintf(stdout, "updated %s\n", result.SettingsPath)
	if result.BackupPath != "" {
		fmt.Fprintf(stdout, "backup %s\n", result.BackupPath)
	}
	fmt.Fprintf(stdout, "managed_env %s\n", strings.Join(result.ManagedKeys, ","))
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
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt codex: %v\n", err)
		return 1
	}

	result, err := agent_adapter.WriteCodexSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt codex: %v\n", err)
		if errors.Is(err, agent_adapter.ErrInvalidExistingCodexConfig) {
			return 2
		}
		return 1
	}

	fmt.Fprintf(stdout, "updated %s\n", result.ConfigPath)
	fmt.Fprintf(stdout, "updated %s\n", result.AuthPath)
	for _, backupPath := range result.BackupPaths {
		fmt.Fprintf(stdout, "backup %s\n", backupPath)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(stdout, "WARN %s\n", warning)
	}
	fmt.Fprintf(stdout, "managed_env %s\n", strings.Join(result.ManagedEnvKeys, ","))
	fmt.Fprintf(stdout, "managed_toml %s\n", strings.Join(result.ManagedTomlKeys, ","))
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
	ensure := registerEnsureModelFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if err := ensureVirtualModelIfRequested(ensure, opts.Model, opts.Token); err != nil {
		fmt.Fprintf(stderr, "adopt opencode: %v\n", err)
		return 1
	}

	result, err := agent_adapter.WriteOpenCodeSettings(opts)
	if err != nil {
		fmt.Fprintf(stderr, "adopt opencode: %v\n", err)
		if errors.Is(err, agent_adapter.ErrInvalidExistingOpenCodeConfig) {
			return 2
		}
		return 1
	}

	fmt.Fprintf(stdout, "updated %s\n", result.ConfigPath)
	if result.BackupPath != "" {
		fmt.Fprintf(stdout, "backup %s\n", result.BackupPath)
	}
	fmt.Fprintf(stdout, "managed_provider %s\n", strings.Join(result.ManagedKeys, ","))
	return 0
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

	result, err := agent_adapter.RestoreClaudeCodeSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore claude-code: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "restored %s\n", result.SettingsPath)
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

	result, err := agent_adapter.RestoreCodexSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore codex: %v\n", err)
		return 1
	}
	for _, path := range []string{result.ConfigPath, result.AuthPath} {
		if path != "" {
			fmt.Fprintf(stdout, "restored %s\n", path)
		}
	}
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

	result, err := agent_adapter.RestoreOpenCodeSettingsWithOptions(opts)
	if err != nil {
		fmt.Fprintf(stderr, "restore opencode: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "restored %s\n", result.ConfigPath)
	fmt.Fprintf(stdout, "from %s\n", result.RestoredFrom)
	return 0
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
