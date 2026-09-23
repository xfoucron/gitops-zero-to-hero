package config

import "os"

type Config struct {
	ApplicationURL  string
	ApplicationPort string

	RedisURL    string
	PostgresURL string
}

func Load() *Config {
	return &Config{
		ApplicationURL:  getEnv("APPLICATION_URL", "http://localhost:80"),
		ApplicationPort: getEnv("APPLICATION_PORT", "80"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379/0"),
		PostgresURL:     getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/url_shortener?sslmode=disable"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
