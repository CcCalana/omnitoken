# PROPOSAL: T-045 Anthropic to OpenAI Protocol Conversion

refs T-045.

## Decision 1: conversion layer placement

Choose **an Anthropic `http.Handler` wrapper around the existing OpenAI proxy path**.

`POST /v1/messages` should use the same outer gateway stack as `/v1/chat/completions`: virtual key auth, monthly budget enforcement, and virtual model resolution. Inside that stack, the Anthropic handler converts the incoming Messages request into an OpenAI chat completions request, calls the existing Ark/OpenAI proxy handler, then converts the OpenAI response back to Anthropic format.

The important ordering detail is usage capture. For `/v1/messages`, build the handler as:

```text
protectGatewayRoute
  -> enforceMonthlyBudget
  -> resolveVirtualModel
  -> anthropic.MessagesHandler
       -> usage.Middleware
            -> proxy.ArkChatProxy
```

The Anthropic handler supplies a transforming `ResponseWriter` to the inner usage-wrapped OpenAI handler. That writer lets `usage.Middleware` capture the OpenAI-format upstream response while the client receives Anthropic-format JSON or SSE. This preserves the existing credential selector, retry behavior, upstream credential id recording, OpenAI usage parser, and cost ledger path without adding a second upstream client.

Reject a direct Anthropic handler that calls upstream itself. It would duplicate credential selection, retry, timeout, thinking-disable, request-id, and upstream error handling logic that already exists in `internal/proxy`.

## Decision 2: streaming SSE conversion strategy

Choose **real-time chunk-by-chunk conversion**.

For `stream: true`, the Anthropic response writer should parse OpenAI SSE frames as they are written by the inner proxy and emit Anthropic SSE events immediately. It should maintain a small state machine:

- before first OpenAI chunk: emit `message_start`
- first content block of a kind: emit `content_block_start`
- each text delta: emit `content_block_delta` with `text_delta`
- each reasoning delta: emit `content_block_delta` with `thinking_delta`
- finish chunk: emit any open `content_block_stop`, then `message_delta`
- `[DONE]`: emit `message_stop`

Do not buffer the full stream before emitting. Claude Code is latency-sensitive, and buffering would erase the existing proxy's first-byte behavior. The state machine should tolerate fragmented network writes by buffering incomplete SSE frames until a blank-line frame boundary is seen.

If the upstream returns `stream: true` but the content type is not event-stream, fall back to buffered non-stream conversion when the body is valid OpenAI JSON; otherwise return an Anthropic error envelope.

## Decision 3: usage parsing path

Choose **existing OpenAI usage parsing**.

The usage middleware must continue to receive OpenAI-format captured bytes, not Anthropic output bytes. That keeps `internal/usage/parser.go` unchanged and avoids introducing a second provider parser for data that originated from OpenAI-compatible upstream responses.

Implementation consequence: the Anthropic transforming writer must sit outside the inner usage capture, so `captureResponseWriter.Write` stores the original OpenAI bytes before forwarding them into the Anthropic converter. Non-streaming responses can be buffered in the converter before writing Anthropic JSON to the real client; streaming responses are converted frame-by-frame after the usage capture has seen the OpenAI frame.

Do not add Anthropic usage parsing in T-045. The Anthropic usage fields returned to clients are response-shape compatibility, not the ledger source of truth.

## Decision 4: request metadata snapshot

Choose **context pre-injection by the Anthropic converter**, with a minimal middleware fallback.

Before calling the inner handler, the Anthropic converter should parse the original Messages request once, validate the basic shape, and put request metadata into context:

- requested model
- stream flag
- protocol marker `anthropic_messages`

Then it should replace `r.Body` with the converted OpenAI request body and pass the request to the inner usage-wrapped OpenAI handler. Since the inner middleware sees an OpenAI-format body after conversion, the existing `snapshotRequestMetadata` behavior remains correct for model and stream.

Add a narrow context override in `usage.Middleware` only if implementation shows the requested model would otherwise be overwritten by routed/default model during proxy rewrite. The fallback should be generic request metadata from `httpx`, not Anthropic-specific JSON parsing in `internal/usage`.

## Decision 5: content block index management

Choose **Anthropic block indexes derived from output block boundaries, independent of OpenAI choice index**.

T-045 supports only a single OpenAI choice for Anthropic conversion. If multiple choices are returned, convert `choices[0]` and ignore additional choices with a WARN log containing request id and choice count only.

Maintain converter state:

```text
next_index = 0
open_block_kind = none | thinking | text
open_block_index = -1
```

When a reasoning delta arrives and no thinking block is open, open a thinking block at `next_index`, then increment `next_index`. When a text delta arrives and no text block is open, close any open thinking block, open a text block at `next_index`, then increment `next_index`. Repeated deltas of the same kind reuse the current block index. On finish, close the open block if any.

This produces stable Anthropic indexes:

- reasoning only: index `0`
- text only: index `0`
- reasoning then text: thinking index `0`, text index `1`

Do not use OpenAI `choices[].index` as the Anthropic content block index. It identifies the completion choice, not the ordered content block inside a message.

## Request conversion notes

- `system` string becomes one OpenAI `system` message prepended before user/assistant messages.
- `system` array is flattened into one `system` message by joining supported text blocks with newlines; unsupported blocks are ignored with no secret-bearing log fields.
- user/assistant text content maps directly.
- content block arrays map supported `text` blocks to text. Unsupported v1 blocks become empty text or are omitted according to whether omitting would leave the message empty.
- `max_tokens`, `temperature`, `top_p`, `stream`, and `stop_sequences` map to OpenAI `max_tokens`, `temperature`, `top_p`, `stream`, and `stop`.
- `thinking` may pass through unchanged for providers that understand it; no semantic mapping to `reasoning_effort` in T-045.
- `tools` should be ignored for v1 compatibility, not rejected and not forwarded.

Invalid input such as an empty `messages` array should return HTTP 400 with Anthropic error format:

```json
{"type":"error","error":{"type":"invalid_request_error","message":"messages must not be empty"}}
```

## Response conversion notes

Non-stream OpenAI responses map to Anthropic `type: "message"` responses. `choices[0].message.reasoning_content`, when present, becomes a leading `thinking` content block. `choices[0].message.content` becomes a `text` content block. Finish reasons map as specified in T-045: `stop` to `end_turn`, `length` to `max_tokens`, and `tool_calls` to `tool_use`; unknown finish reasons should pass through as `end_turn` with a WARN log.

Upstream error envelopes from the inner proxy should be converted to Anthropic error envelopes. Preserve the HTTP status code, map 400 to `invalid_request_error`, 401/403 to `authentication_error`, 429/503 rate-limit cases to `rate_limit_error`, and other 5xx cases to `api_error`. Do not expose upstream headers, credential ids, stack traces, request bodies, prompts, or API keys in error messages.

## Test plan

- Unit tests for Anthropic request conversion: system string, system array, text blocks, empty messages, unsupported blocks, stream flag, and stop sequences.
- Unit tests for non-stream response conversion: text, reasoning plus text, usage mapping, finish reason mapping, upstream error mapping.
- Unit tests for streaming conversion: message start/stop ordering, text-only, thinking-only, thinking-then-text, final usage chunk, malformed SSE frame tolerance, and `[DONE]`.
- Handler tests proving `/v1/messages` reuses auth/budget/model-resolution context and records usage from OpenAI-format captured bytes.
- Golden tests for `testdata/golden/ark/anthropic_nonstream_default.json` and OpenAI SSE fixtures.
- E2E tests remain behind the existing e2e gate and require explicit local secrets; normal `go test ./...` must not call real upstreams.

## Dependencies

No new third-party dependencies are needed.
