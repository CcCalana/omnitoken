# T-041 Claude Code adapter proposal

refs T-041.

## Decision 1: CLI binary layout

Choose a standalone `cmd/omnitoken-adopt` binary.

Rationale: the adopter is an employee-side setup tool, not an admin control-plane command. Keeping it standalone avoids pulling admin concerns, server config, or future admin authentication into a local config writer. The command surface stays exactly scoped to:

```text
omnitoken-adopt adopt claude-code --gateway-url <URL> --token <virtual_key> [--model chat-balanced] [--home <path>]
omnitoken-adopt restore claude-code [--home <path>]
```

Use the standard `flag` package only. Do not add `cobra`, `urfave/cli`, or a shared Registry abstraction in this task.

## Decision 2: virtual key source

Choose explicit `--token`.

Rationale: T-041 should remain a local file-write operation. Auto-pulling a token from an admin URL would add network behavior, admin credentials, and key lifecycle policy to a task whose acceptance criteria are config write, backup, restore, and CLI parsing. The admin plane can create the virtual key through existing flows; this tool receives it as input and writes it into Claude Code's expected `ANTHROPIC_AUTH_TOKEN` env slot.

Operational guardrail: CLI output must never print the full token. Results may show the settings path, backup path, gateway URL, model, and env key names only.

## Decision 3: backup retention

Choose keep all backups.

Rationale: `settings.json` is small, and preserving every pre-write copy gives users a simple audit and recovery trail. Retention pruning can be added later if real usage shows directory growth.

Use:

```text
~/.omnitoken/backups/claude-code/settings.json.<timestamp>.bak
```

Implementation nuance: raw RFC3339 contains `:` characters, which are invalid in Windows filenames. Use an RFC3339-derived UTC timestamp with filename-safe separators, for example `2026-05-19T10-02-00.000000000Z`. Lexicographic sort must match chronological order so `restore claude-code` can pick the latest backup deterministically.

## Implementation notes

- Merge `~/.claude/settings.json` semantically with Go's JSON support: preserve all existing non-`env` fields and existing unrelated `env` keys, while normalizing formatting on write.
- If the root JSON or `env` field is not an object, fail without writing rather than overwriting user data.
- On first write when no settings file exists, create the file without a backup because there is no original to preserve. Existing-file writes always create a backup before replacement.
- Write only Claude Code config. Do not write `.claude.json`, status-line config, Codex/OpenCode config, or protocol conversion code in T-041.
- `ANTHROPIC_BASE_URL` should be the exact `--gateway-url` value supplied by the operator; T-045 owns the future Anthropic-compatible endpoint behavior.
