package config

import "os"

type Config struct {
	Host        string
	Port        string
	DatabaseURL string
	TokenSecret string
	StorageDir  string
	// DevLogin exposes /v1/dev/login, which hands out a token for any display
	// name without a password. Local testing only.
	DevLogin bool
}

func Load() Config {
	return Config{
		Host:        getenv("SWAPPY_HOST", "localhost"),
		Port:        getenv("SWAPPY_PORT", "8080"),
		DatabaseURL: getenv("SWAPPY_DATABASE_URL", "postgres://swappy:swappy@localhost:5433/swappy?sslmode=disable"),
		TokenSecret: os.Getenv("SWAPPY_TOKEN_SECRET"),
		StorageDir:  getenv("SWAPPY_STORAGE_DIR", "data/images"),
		DevLogin:    os.Getenv("SWAPPY_DEV_LOGIN") == "true",
	}
}

func (config Config) Address() string {
	return config.Host + ":" + config.Port
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
