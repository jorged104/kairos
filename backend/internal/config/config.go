package config

import (
	"errors"
	"os"
)

const defaultPort = "8080"

type Config struct {
	DatabaseURL string
	Port        string
}

// Load reads the API configuration from environment variables.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("API_PORT"),
	}
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("falta la variable de entorno DATABASE_URL")
	}
	if cfg.Port == "" {
		cfg.Port = defaultPort
	}
	return cfg, nil
}
