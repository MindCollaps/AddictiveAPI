package config

import "os"

type Config struct {
	Port         string
	DatabasePath string
	Environment  string
	JWTSecret    string
	JWTIssuer    string
}

func Load() Config {
	return Config{
		Port:         getenv("PORT", "8080"),
		DatabasePath: getenv("DATABASE_PATH", "./data/app.db"),
		Environment:  getenv("APP_ENV", "development"),
		JWTSecret:    getenv("JWT_SECRET", "change-me-in-production"),
		JWTIssuer:    getenv("JWT_ISSUER", "addictiveapi"),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
