package config

import (
	"os"
	"testing"
)

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

func TestLoadDefaults(t *testing.T) {
	t.Setenv("OMNITOKEN_GATEWAY_ADDR", "")
	t.Setenv("OMNITOKEN_ADMIN_ADDR", "")
	t.Setenv("OMNITOKEN_ADMIN_CORS_ORIGINS", "")
	t.Setenv("OMNITOKEN_ARK_API_KEY", "")
	t.Setenv("OMNITOKEN_ARK_OPENAI_BASE_URL", "")
	t.Setenv("OMNITOKEN_ARK_ANTHROPIC_BASE_URL", "")
	t.Setenv("OMNITOKEN_ARK_DEFAULT_MODEL", "")
	t.Setenv("OMNITOKEN_ARK_DISABLE_THINKING", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Gateway.Addr != ":8080" {
		t.Fatalf("gateway addr = %q", cfg.Gateway.Addr)
	}
	if cfg.Admin.Addr != ":8081" {
		t.Fatalf("admin addr = %q", cfg.Admin.Addr)
	}
	if len(cfg.Admin.CORSOrigins) != 0 {
		t.Fatalf("expected env-provided empty CORS list, got %#v", cfg.Admin.CORSOrigins)
	}
	if cfg.Ark.Enabled() {
		t.Fatal("expected Ark to be disabled without API key")
	}
	if cfg.Ark.OpenAIBaseURL != "" || cfg.Ark.DefaultModel != "" {
		t.Fatalf("expected zero Ark config, got %#v", cfg.Ark)
	}
}

func TestLoadDefaultCORSWhenUnset(t *testing.T) {
	unsetEnv(t, "OMNITOKEN_ADMIN_CORS_ORIGINS")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Admin.CORSOrigins) != 1 || cfg.Admin.CORSOrigins[0] != "http://localhost:3000" {
		t.Fatalf("unexpected default CORS origins: %#v", cfg.Admin.CORSOrigins)
	}
}

func TestLoadParsesAdminAndArkConfig(t *testing.T) {
	t.Setenv("OMNITOKEN_GATEWAY_ADDR", ":18080")
	t.Setenv("OMNITOKEN_ADMIN_ADDR", ":18081")
	t.Setenv("OMNITOKEN_ADMIN_CORS_ORIGINS", "http://localhost:3000, https://admin.example.com ")
	t.Setenv("OMNITOKEN_ARK_API_KEY", "secret")
	t.Setenv("OMNITOKEN_ARK_OPENAI_BASE_URL", "https://ark.example.com/v3")
	t.Setenv("OMNITOKEN_ARK_ANTHROPIC_BASE_URL", "https://ark.example.com")
	t.Setenv("OMNITOKEN_ARK_DEFAULT_MODEL", "ark-code-latest")
	t.Setenv("OMNITOKEN_ARK_DISABLE_THINKING", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Gateway.Addr != ":18080" || cfg.Admin.Addr != ":18081" {
		t.Fatalf("unexpected service addrs: %#v", cfg)
	}
	if got := cfg.Admin.CORSOrigins; len(got) != 2 || got[1] != "https://admin.example.com" {
		t.Fatalf("unexpected CORS origins: %#v", got)
	}
	if !cfg.Ark.Enabled() {
		t.Fatal("expected Ark to be enabled")
	}
	if cfg.Ark.OpenAIBaseURL != "https://ark.example.com/v3" {
		t.Fatalf("unexpected Ark OpenAI URL: %q", cfg.Ark.OpenAIBaseURL)
	}
	if cfg.Ark.AnthropicBaseURL != "https://ark.example.com" {
		t.Fatalf("unexpected Ark Anthropic URL: %q", cfg.Ark.AnthropicBaseURL)
	}
	if cfg.Ark.DefaultModel != "ark-code-latest" {
		t.Fatalf("unexpected Ark model: %q", cfg.Ark.DefaultModel)
	}
	if !cfg.Ark.DisableThinking {
		t.Fatal("expected Ark disable thinking to be true")
	}
}

func TestLoadRejectsInvalidBool(t *testing.T) {
	t.Setenv("OMNITOKEN_ARK_DISABLE_THINKING", "sometimes")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid bool error")
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	oldValue, hadValue := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if hadValue {
			_ = os.Setenv(key, oldValue)
			return
		}
		_ = os.Unsetenv(key)
	})
}
