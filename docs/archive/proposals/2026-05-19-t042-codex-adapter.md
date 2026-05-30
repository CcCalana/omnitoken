# T-042 Codex adapter proposal

refs T-042.

## Decision 1: TOML editing

Choose a minimal line-based patcher for `~/.codex/config.toml`.

Rationale: T-042 only owns a small, explicit set of top-level keys and one managed provider table. `pelletier/go-toml/v2` is MIT and active enough, but decode/encode does not preserve user comments or formatting. `BurntSushi/toml` is MIT and stable, with the same rewrite problem. A Go equivalent of Rust `toml_edit` is not mature enough to justify a new dependency.

Implementation shape:

- Validate existing TOML with a small scanner for the managed surface: reject unterminated strings, malformed table headers, duplicate managed keys, or non-string/bool values under `[model_providers.omnitoken]`.
- Patch only these owned lines/tables; leave every unrelated byte, comment, blank line, and table untouched.
- Replace the full `[model_providers.omnitoken]` table body because OmniToken owns that table; do not edit other provider tables.
- Create a new file with the managed block when `config.toml` does not exist.

Do not add a TOML dependency in T-042. If Claude later wants full TOML AST validation, that should be a follow-up proposal with license ledger impact.

## Decision 2: config/auth split

`config.toml` owns routing and auth mode; `auth.json` owns the secret.

Managed TOML keys:

```text
model = "chat-balanced"
model_provider = "omnitoken"
preferred_auth_method = "apikey"
cli_auth_credentials_store = "file"
[model_providers.omnitoken]
name = "OmniToken"
base_url = "<gateway-url>/v1"
env_key = "OPENAI_API_KEY"
wire_api = "chat"
requires_openai_auth = true
```

Managed auth JSON keys:

```text
OPENAI_API_KEY = <virtual_key>
```

Notes: Codex official config documents `model`, `model_provider`, provider `base_url`, `env_key`, `wire_api`, and `requires_openai_auth`; current auth docs also describe API-key login via `OPENAI_API_KEY`. The `/v1` suffix is intentional because OmniToken currently exposes OpenAI-compatible `/v1/models` and `/v1/chat/completions`.

## Decision 3: helper scope

Do not extract `Registry` or `AgentConfig` yet.

T-042 should only extract file-level helpers shared by Claude Code and Codex:

- `writeAtomic(path, data, perm)` using sibling temp file, `fsync` best-effort where portable, then rename.
- JSON object read/merge/write helpers that marshal nested objects normally, fixing R-041 M-20.
- Backup helpers parameterized by agent name and filename, keeping compact UTC timestamps.

The registry/interface abstraction remains T-040 after T-043 gives a third concrete adapter.
