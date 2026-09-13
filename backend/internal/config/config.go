package config

import "os"

type Config struct {
	Host string
	Port string
}

func Load() Config {
	host := os.Getenv("SWAPPY_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("SWAPPY_PORT")
	if port == "" {
		port = "8080"
	}

	return Config{Host: host, Port: port}
}

func (config Config) Address() string {
	return config.Host + ":" + config.Port
}
