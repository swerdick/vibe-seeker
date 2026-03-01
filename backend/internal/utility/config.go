package utility

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Environment string
	LogLevel    string
	Port        int
	AppName     string
	CORSOrigin  string
}

func NewConfig() Config {
	return Config{
		Environment: readEnvironmentVariable("ENVIRONMENT", "local"),
		LogLevel:    readEnvironmentVariable("LOG_LEVEL", "info"),
		Port:        readIntEnvironmentVariable("PORT", 8080),
		AppName:     readEnvironmentVariable("APP_NAME", "vibe-seeker-api"),
		CORSOrigin:  readEnvironmentVariable("CORS_ORIGIN", "http://localhost:5173"),
	}
}

func readEnvironmentVariable(key string, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	slog.Warn("environment variable not set", "key", key)

	return defaultValue
}

func readIntEnvironmentVariable(key string, defaultValue int) int {
	raw, ok := os.LookupEnv(key)
	if !ok {
		slog.Warn("environment variable not set", "key", key)
		return defaultValue
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		slog.Error(fmt.Sprintf("environment variable %s is not a valid integer", key), "value", raw)
		return defaultValue
	}

	return value
}
