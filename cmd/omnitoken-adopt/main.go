package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/omnitoken/omnitoken/internal/agent_adapter"
)

const defaultModel = "chat-balanced"

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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected argument %q\n", fs.Arg(0))
		return 2
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

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  omnitoken-adopt adopt claude-code --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt adopt codex --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt adopt opencode --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore claude-code [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore codex [--home <path>]")
	fmt.Fprintln(w, "  omnitoken-adopt restore opencode [--home <path>]")
}
