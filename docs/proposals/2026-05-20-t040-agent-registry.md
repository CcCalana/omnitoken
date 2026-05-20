# T-040 Agent registry proposal

refs T-040.

## Decision 1: interface signature

Choose route (c): a shared base options struct plus a normal, homogeneous interface.

```go
type AgentType string

const (
  AgentTypeClaudeCode AgentType = "claude-code"
  AgentTypeCodex      AgentType = "codex"
  AgentTypeOpenCode   AgentType = "opencode"
)

type BaseOptions struct {
  Home       string
  GatewayURL string
  Token      string
  Model      string
  Now        func() time.Time
}

type BaseRestoreOptions struct {
  Home string
}

type AgentConfig interface {
  Type() AgentType
  Write(BaseOptions) (Result, error)
  Restore(BaseRestoreOptions) (Result, error)
}
```

Rationale:

- Route (a), `interface{}` params, copies tingly-box but pushes type assertions into every adapter and makes invalid calls runtime-only.
- Route (b), `AgentConfig[Opts any]`, is type-safe per adapter but cannot form a simple `map[AgentType]AgentConfig[...]` registry without erasing the type again.
- Route (c) matches the current reality: all three write options are the same fields (`Home`, `GatewayURL`, `Token`, `Model`, `Now`) and restore options are currently just `Home`. It keeps Registry homogeneous and gives static field names to adapter implementations.

Backward compatibility plan:

- Keep `ClaudeCodeOptions`, `CodexOptions`, `OpenCodeOptions`, `RestoreClaudeCodeOptions`, `RestoreCodexOptions`, and `RestoreOpenCodeOptions` as type aliases or thin structs for existing callers.
- Keep existing exported `WriteXxxSettings` and `RestoreXxxSettingsWithOptions` functions. They should delegate to concrete configs:

```go
func WriteCodexSettings(opts CodexOptions) (Result, error) {
  return (&CodexConfig{}).Write(BaseOptions(opts))
}
```

If type aliases are used, the conversion is unnecessary. If named structs are kept, add small conversion helpers. CLI code remains unchanged in T-040.

## Decision 2: Result shape

Shrink `Result` to canonical multi-value fields plus role paths:

```go
type Result struct {
  Paths             map[string]string
  BackupPaths       []string
  RestoredFromPaths []string
  ManagedKeys       []string
  Warnings          []string
}
```

Path roles:

- Claude Code: `Paths["settings"]`
- Codex: `Paths["config"]`, `Paths["auth"]`
- OpenCode: `Paths["config"]`

Remove these fields from the canonical struct: `SettingsPath`, `ConfigPath`, `AuthPath`, `BackupPath`, `RestoredFrom`, `ManagedEnvKeys`, `ManagedTomlKeys`.

Migration plan:

- Update CLI to read paths through role helpers or direct map lookups, while preserving stdout text exactly: `updated <path>`, `backup <path>`, `restored <path>`, `from <path>`, `managed_env`, `managed_toml`, `managed_provider`.
- `BackupPath` assertions become `len(result.BackupPaths) == 1` plus `result.BackupPaths[0]`.
- `RestoredFrom` assertions become `len(result.RestoredFromPaths) == 1` plus `result.RestoredFromPaths[0]`.
- `ManagedEnvKeys` / `ManagedTomlKeys` become filtered lists built by adapter-specific helpers or grouped constants. For Codex CLI, preserve two output lines by returning `ManagedKeys` with all keys and letting `ManagedCodexEnvKeys()` / `ManagedCodexTomlKeys()` remain available for the wrapper/CLI.

Important caveat: because T-040 says `cmd/omnitoken-adopt/main.go` must be zero-modified, implementation has two viable choices:

1. Keep compatibility accessors as methods on `Result` and leave CLI untouched.
2. Keep legacy fields temporarily while adding canonical fields, then remove legacy fields in T-046 when CLI switches to registry.

I recommend option 2 for T-040 to satisfy the explicit "CLI zero modification" constraint. The canonical fields should be populated and tested now; legacy fields stay as compatibility shims and should be marked internal-deprecation in comments. Full physical removal can happen in T-046 when CLI is allowed to change. This avoids breaking the current CLI while still moving adapter internals and new tests to the slim result shape.

## Decision 3: registry registration

Use package `init()` to register defaults into a package-level default registry.

```go
var DefaultRegistry = NewRegistry()

func init() {
  MustRegisterDefault(&ClaudeCodeConfig{})
  MustRegisterDefault(&CodexConfig{})
  MustRegisterDefault(&OpenCodeConfig{})
}
```

`Registry` should support:

- `Register(config AgentConfig) error`
- `MustRegister(config AgentConfig)`
- `Get(agentType AgentType) (AgentConfig, bool)`
- `List() []AgentType`

Duplicate registration should return an error from `Register` and panic only through `MustRegister`. Tests should use `NewRegistry()` for an empty isolated registry and should verify deterministic `List()` ordering.

Rationale:

- `init()` keeps default registry use zero-boilerplate for T-044/T-046.
- `NewRegistry()` keeps tests explicit and isolated.
- Registering concrete configs at package init is safe here because adapters have no external side effects; all file writes remain behind `Write` / `Restore`.

## Implementation notes

- Do not modify `cmd/omnitoken-adopt/main.go` in T-040.
- Move `nonEmptyStrings` from `claude_code.go` to `fileio.go`.
- Add `registry.go` for `AgentType`, `Options`, `RestoreOptions`, `AgentConfig`, and `Registry`.
- Refactor each adapter minimally: introduce `ClaudeCodeConfig`, `CodexConfig`, `OpenCodeConfig` with `Type`, `Write`, and `Restore`; keep exported function wrappers.
- Add `registry_test.go` for register/get/list/duplicate behavior and table-driven default registry parity.
