package configs

import (
"os"
"strconv"
"time"
)

type Config struct {
AppEnv              string
Port                string
DatabaseURL         string
JWTSecret           string
JWTRefreshSecret    string
KafkaEnabled        bool
KafkaBrokers        string
AutomationWorkers   int
WebhookTimeout      time.Duration
WebsocketBufferSize int
}

func Load() Config {
return Config{
AppEnv:              getEnv("APP_ENV", "dev"),
Port:                getEnv("PORT", "8080"),
DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/scrumx?sslmode=disable"),
JWTSecret:           getEnv("JWT_SECRET", "change-me-access"),
JWTRefreshSecret:    getEnv("JWT_REFRESH_SECRET", "change-me-refresh"),
KafkaEnabled:        getEnvBool("KAFKA_ENABLED", false),
KafkaBrokers:        getEnv("KAFKA_BROKERS", "kafka:9092"),
AutomationWorkers:   getEnvInt("AUTOMATION_WORKERS", 2),
WebhookTimeout:      time.Duration(getEnvInt("WEBHOOK_TIMEOUT_SECONDS", 5)) * time.Second,
WebsocketBufferSize: getEnvInt("WEBSOCKET_BUFFER_SIZE", 64),
}
}

func getEnv(key, fallback string) string {
if v := os.Getenv(key); v != "" {
return v
}
return fallback
}

func getEnvInt(key string, fallback int) int {
if v := os.Getenv(key); v != "" {
if parsed, err := strconv.Atoi(v); err == nil {
return parsed
}
}
return fallback
}

func getEnvBool(key string, fallback bool) bool {
if v := os.Getenv(key); v != "" {
if parsed, err := strconv.ParseBool(v); err == nil {
return parsed
}
}
return fallback
}
