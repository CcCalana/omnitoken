package config

import "os"

// Env returns an environment variable or a fallback value.
func Env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
