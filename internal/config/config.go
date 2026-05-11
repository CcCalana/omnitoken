package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultArkOpenAIBaseURL    = "https://ark.cn-beijing.volces.com/api/coding/v3"
	DefaultArkAnthropicBaseURL = "https://ark.cn-beijing.volces.com/api/coding"
	DefaultArkModel            = "ark-code-latest"
)

type Config struct {
	Gateway     GatewayConfig
	Admin       AdminConfig
	DatabaseURL string
	RedisAddr   string
	NATSURL     string
	LogBodyMode string
	MasterKey   string
	Ark         ArkConfig
}

type GatewayConfig struct {
	Addr string
}

type AdminConfig struct {
	Addr           string
	CORSOrigins    []string
	CORSMethods    []string
	BootstrapToken string
}

type ArkConfig struct {
	APIKey           string
	OpenAIBaseURL    string
	AnthropicBaseURL string
	DefaultModel     string
	DisableThinking  bool
}

func (c ArkConfig) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

func Load() (Config, error) {
	disableThinking, err := envBool("OMNITOKEN_ARK_DISABLE_THINKING", false)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Gateway: GatewayConfig{
			Addr: Env("OMNITOKEN_GATEWAY_ADDR", ":8080"),
		},
		Admin: AdminConfig{
			Addr:           Env("OMNITOKEN_ADMIN_ADDR", ":8081"),
			CORSOrigins:    envCSV("OMNITOKEN_ADMIN_CORS_ORIGINS", []string{"http://localhost:3000"}),
			CORSMethods:    envCSV("OMNITOKEN_ADMIN_CORS_METHODS", []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}),
			BootstrapToken: Env("OMNITOKEN_ADMIN_BOOTSTRAP_TOKEN", ""),
		},
		DatabaseURL: Env("OMNITOKEN_DATABASE_URL", ""),
		RedisAddr:   Env("OMNITOKEN_REDIS_ADDR", ""),
		NATSURL:     Env("OMNITOKEN_NATS_URL", ""),
		LogBodyMode: Env("OMNITOKEN_LOG_BODY_MODE", "off"),
		MasterKey:   Env("OMNITOKEN_MASTER_KEY", ""),
		Ark: ArkConfig{
			APIKey:           Env("OMNITOKEN_ARK_API_KEY", ""),
			OpenAIBaseURL:    Env("OMNITOKEN_ARK_OPENAI_BASE_URL", DefaultArkOpenAIBaseURL),
			AnthropicBaseURL: Env("OMNITOKEN_ARK_ANTHROPIC_BASE_URL", DefaultArkAnthropicBaseURL),
			DefaultModel:     Env("OMNITOKEN_ARK_DEFAULT_MODEL", DefaultArkModel),
			DisableThinking:  disableThinking,
		},
	}, nil
}

// Env returns an environment variable or a fallback value.
func Env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envCSV(key string, fallback []string) []string {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return append([]string(nil), fallback...)
	}
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func envBool(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", key, err)
	}
	return value, nil
}
