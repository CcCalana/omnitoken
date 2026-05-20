# T-043 OpenCode adapter proposal

refs T-043.

## Decision 1: managed OpenCode fields

Use OpenCode's schema spelling: top-level `provider`, singular. The T-043 task body says `providers.omnitoken`; implementation should treat that as the intended managed provider object but write `provider.omnitoken` so OpenCode accepts the file.

Managed root field:

```json
"$schema": "https://opencode.ai/config.json"
```

Managed provider object:

```json
"provider": {
  "omnitoken": {
    "name": "OmniToken",
    "npm": "@ai-sdk/openai-compatible",
    "options": {
      "baseURL": "<gateway-url>/v1",
      "apiKey": "<virtual_key>"
    },
    "models": {
      "<model>": {
        "name": "<model>"
      }
    }
  }
}
```

Rationale:

- The official OpenCode config docs and schema use top-level `provider`, where provider entries support `npm`, `options`, and `models`; the docs show provider packages such as `@ai-sdk/openai-compatible` and API keys under provider `options`.
- The local tingly-box reference uses `provider` with `options.baseURL` and `options.apiKey`; token_proxy's summary writes a separate auth root, but the task only owns `opencode.json` and current OpenCode docs do not expose a separate stable secret store for this provider key.
- Therefore T-043 should write the token into `provider.omnitoken.options.apiKey`, never echo it to stdout/stderr, and cover leakage with the same CLI-output assertions used in T-041/T-042.

Managed key list for stdout:

```text
managed_provider provider.omnitoken.name,provider.omnitoken.npm,provider.omnitoken.options.baseURL,provider.omnitoken.options.apiKey,provider.omnitoken.models.<model>.name
```

`provider.omnitoken` is replaced as a whole on each adopt. Other root keys and other `provider.*` entries are preserved. If an existing `providers` plural key exists, leave it untouched as user data and write the valid singular `provider` key.

## Decision 2: XDG fallback on Windows

Use the task's deterministic path order:

1. If `--home <path>` is passed: `<home>/.config/opencode/opencode.json`.
2. Else if `XDG_CONFIG_HOME` is non-empty: `$XDG_CONFIG_HOME/opencode/opencode.json`.
3. Else: `<home>/.config/opencode/opencode.json`, where `home` comes from `resolveHome("")`.

Do not use `%APPDATA%` for T-043.

Rationale:

- OpenCode's public config docs describe config files in `~/.config/opencode`, matching the tingly-box reference and token_proxy's `resolve_opencode_config_dir` fallback.
- token_proxy checks `XDG_CONFIG_HOME` before falling back to `home/.config/opencode`; it does not use `%APPDATA%`.
- Keeping Windows on `%USERPROFILE%\.config\opencode` makes `--home` tests and production behavior consistent across platforms and avoids another platform-specific override surface.

Implementation tests should use `t.TempDir()` and `t.Setenv()` only. Add explicit tests for `XDG_CONFIG_HOME` set, unset fallback via `HOME`/`USERPROFILE`, and `--home` overriding `XDG_CONFIG_HOME`.

## Decision 3: no `--config-home`

Do not add `--config-home`.

Rationale:

- T-041 and T-042 expose only `--home`; adding a second path flag only for OpenCode would make the CLI harder to explain and test.
- Advanced users can set `XDG_CONFIG_HOME` before running the command.
- For dev and smoke safety, `--home <temp>` remains sufficient and aligned with AGENTS.md §9.5.

T-043 should not extract `Registry` / `AgentConfig`; after R-043 approval, T-040 is the correct place to introduce that abstraction.
