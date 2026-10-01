package config

import (
	"os"
)

type Config struct {
	Host               string
	Port               string
	DBPath             string
	JWTSecret          string
	UploadDir          string
	CORSAllowedOrigins string
}

func Load() Config {
	host := os.Getenv("SWAPPY_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := os.Getenv("SWAPPY_PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("SWAPPY_DB_PATH")
	if dbPath == "" {
		dbPath = "swappy.db"
	}

	jwtSecret := os.Getenv("SWAPPY_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "swappy-super-secure-dev-jwt-secret-key-32chars"
	}

	uploadDir := os.Getenv("SWAPPY_UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	corsOrigins := os.Getenv("SWAPPY_CORS_ORIGINS")
	if corsOrigins == "" {
		corsOrigins = "*"
	}

	return Config{
		Host:               host,
		Port:               port,
		DBPath:             dbPath,
		JWTSecret:          jwtSecret,
		UploadDir:          uploadDir,
		CORSAllowedOrigins: corsOrigins,
	}
}

func (config Config) Address() string {
	return config.Host + ":" + config.Port
}
