package usage

import "testing"

func TestParseNonStreamUsage(t *testing.T) {
	body := []byte(`{
		"model":"glm-5.1",
		"choices":[{"message":{"content":"pong"}}],
		"usage":{
			"prompt_tokens":15,
			"completion_tokens":2,
			"total_tokens":19,
			"prompt_tokens_details":{"cached_tokens":3},
			"completion_tokens_details":{"reasoning_tokens":2}
		}
	}`)

	got, ok, err := ParseNonStream(body)
	if err != nil {
		t.Fatalf("parse nonstream: %v", err)
	}
	if !ok {
		t.Fatal("expected usage")
	}
	if got.ModelActual != "glm-5.1" {
		t.Fatalf("model actual = %q", got.ModelActual)
	}
	if got.Tokens.PromptTokens != 15 || got.Tokens.CompletionTokens != 2 || got.Tokens.ReasoningTokens != 2 || got.Tokens.CachedTokens != 3 || got.Tokens.TotalTokens != 19 {
		t.Fatalf("tokens mismatch: %#v", got.Tokens)
	}
}

func TestParseStreamFinalUsage(t *testing.T) {
	stream := []byte(`data: {"model":"glm-5.1","choices":[{"delta":{"content":"pong"}}]}

data: {"model":"glm-5.1","choices":[],"usage":{"prompt_tokens":15,"completion_tokens":2,"prompt_tokens_details":{"cached_tokens":4},"completion_tokens_details":{"reasoning_tokens":0}}}

data: [DONE]
`)

	got, ok, err := ParseStream(stream)
	if err != nil {
		t.Fatalf("parse stream: %v", err)
	}
	if !ok {
		t.Fatal("expected usage")
	}
	if got.ModelActual != "glm-5.1" {
		t.Fatalf("model actual = %q", got.ModelActual)
	}
	if got.Tokens.TotalTokens != 17 {
		t.Fatalf("total fallback = %d", got.Tokens.TotalTokens)
	}
	if got.Tokens.CachedTokens != 4 {
		t.Fatalf("cached tokens = %d", got.Tokens.CachedTokens)
	}
}

func TestParseNonStreamMissingUsageKeepsModel(t *testing.T) {
	got, ok, err := ParseNonStream([]byte(`{"model":"glm-5.1","choices":[]}`))
	if err != nil {
		t.Fatalf("parse missing usage: %v", err)
	}
	if ok {
		t.Fatal("unexpected usage")
	}
	if got.ModelActual != "glm-5.1" {
		t.Fatalf("model actual = %q", got.ModelActual)
	}
}

func TestParseNonStreamRejectsInvalidJSON(t *testing.T) {
	if _, _, err := ParseNonStream([]byte(`not-json`)); err == nil {
		t.Fatal("expected parse error")
	}
}
