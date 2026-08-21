// Package config reads service configuration from the environment, with .env
// loaded by the binaries at startup.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Getenv returns the variable's value, or fallback when unset or empty.
func Getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IntEnv returns the variable parsed as int, or fallback when unset or invalid.
func IntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// DatabaseURL assembles the Postgres connection string from DB_* variables.
func DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		Getenv("DB_USERNAME", "postgres"),
		Getenv("DB_PASSWORD", ""),
		Getenv("DB_HOST", "127.0.0.1"),
		Getenv("DB_PORT", "5432"),
		Getenv("DB_NAME", "mlbb_analyzer"),
	)
}
