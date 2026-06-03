# Ark (Volcano Engine) Golden Fixtures

Captured 2026-05-11 from `https://ark.cn-beijing.volces.com/api/coding(/v3)` using
the user-provided dev key against model alias `ark-code-latest` (resolves to
`glm-5.1`, a reasoning model).

> ⚠️ These fixtures contain **no API keys**. They are raw upstream responses only.
> Do not regenerate them inside the repo with a real key; record fixtures via
> `scripts/capture_ark_fixture.go` (TBD in T-002) which strips Authorization
> headers before writing.

Files:

- `openai_nonstream_default.json` — `POST /v3/chat/completions`, `stream:false`,
  default reasoning ON. Demonstrates `completion_tokens_details.reasoning_tokens`
  consuming the full `max_tokens` budget and `message.reasoning_content` carrying
  the non-standard reasoning trace.
- `openai_stream_no_thinking_no_usage.txt` — `stream:true`, `thinking.disabled`,
  no `stream_options`. Shows that **usage is `null` in every chunk** when
  `include_usage` is not requested. Gateway MUST inject
  `stream_options.include_usage=true` or compute usage via tokenizer.
- `openai_stream_no_thinking_with_usage.txt` — same as above but with
  `stream_options.include_usage=true`. Final chunk is `choices:[]` carrying
  `usage` (matches OpenAI). Use this as the primary regression baseline for
  T-007 SSE proxy.
- `anthropic_nonstream_default.json` — `POST /v1/messages`, default thinking ON.
  Content array contains a `type:"thinking"` block. Note Anthropic-style
  `cache_read_input_tokens` field.

Usage mapper requirements derived from these fixtures (input for T-010):

1. Detect `completion_tokens_details.reasoning_tokens` and route to
   `usage_token_breakdown.reasoning_tokens`.
2. Detect `prompt_tokens_details.cached_tokens` and route to
   `usage_token_breakdown.cached_tokens`.
3. Treat upstream `model` field (e.g. `glm-5.1`) as `model_actual`; the
   client-requested model (`ark-code-latest`) goes to `model_requested`.
4. When `thinking.disabled` is supported by provider, surface it as a per-route
   policy knob; otherwise prefer the OpenAI-compat endpoint for latency budget.
