package config

import "testing"

func TestEnvReturnsFallback(t *testing.T) {
	t.Parallel()

	if got := Env("OMNITOKEN_TEST_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestEnvReturnsValue(t *testing.T) {
	t.Setenv("OMNITOKEN_TEST_VALUE", "configured")

	if got := Env("OMNITOKEN_TEST_VALUE", "fallback"); got != "configured" {
		t.Fatalf("expected configured value, got %q", got)
	}
}
