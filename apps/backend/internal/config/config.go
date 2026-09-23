package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ApplicationURL  string
	ApplicationPort string

	RedisURL    string
	PostgresURL string

	CacheTTL time.Duration

	CORSAllowedOrigins []string

	CreateLinkRateLimit       int
	CreateLinkRateLimitWindow time.Duration
}

func Load() *Config {
	return &Config{
		ApplicationURL:  getEnv("APPLICATION_URL", "http://localhost:80"),
		ApplicationPort: getEnv("APPLICATION_PORT", "80"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
		PostgresURL:     getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable"),

		CacheTTL: getEnvDuration("CACHE_TTL", 1*time.Hour),

		CORSAllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"*"}),

		CreateLinkRateLimit:       getEnvInt("CREATE_LINK_RATE_LIMIT", 20),
		CreateLinkRateLimitWindow: getEnvDuration("CREATE_LINK_RATE_LIMIT_WINDOW", 1*time.Minute),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}

	return d
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}

	return n
}

func getEnvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}

	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p := strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}

	return result
}
