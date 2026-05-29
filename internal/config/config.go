package config

import (
	"log"
	"os"
	"strings"
)

type Config struct {
	DatabaseUrl   string
	JWTSecret     string
	Port          string
	JWTIssuer     string
	JWTAudience   string
	AllowedEmails string
}

func LoadConfig() *Config {
	cfg := &Config{
		DatabaseUrl:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		Port:          os.Getenv("PORT"),
		JWTIssuer:     os.Getenv("JWT_ISSUER"),
		JWTAudience:   os.Getenv("JWT_AUDIENCE"),
		AllowedEmails: os.Getenv("ALLOWED_EMAIL_DOMAINS"),
	}

	if cfg.DatabaseUrl == "" {
		log.Fatalf("Config error: database is required")
	}

	if !strings.HasPrefix(cfg.DatabaseUrl, "postgres://") && !strings.HasPrefix(cfg.DatabaseUrl, "host=") {
		log.Fatalf("Config error: DATABASE_URL must be a valid connection string")
	}

	if cfg.JWTSecret == "" {
		log.Fatal("Config error: JWT_SECRET is required")
	}

	if cfg.Port == "" {
		log.Fatalf("Config error: PORT not provided")
	}
	if strings.TrimSpace(cfg.JWTIssuer) == "" {
		log.Fatalf("JWT issuer is empty")
	}
	if strings.TrimSpace(cfg.JWTAudience) == "" {
		log.Fatalf("JWT audience is empty")
	}
	if strings.TrimSpace(cfg.AllowedEmails) == "" {
		log.Fatalf("allowed emails string is empty")
	}
	return cfg
}

func ParseDomains(raw string) []string {
	parts := strings.Split(raw, ",")
	domains := make([]string, 0, len(parts))

	for _, part := range parts {
		domain := strings.TrimSpace(strings.ToLower(part))
		if domain != "" {
			domains = append(domains, domain)
		}
	}

	return domains
}
