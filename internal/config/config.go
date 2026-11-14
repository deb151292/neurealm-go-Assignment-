package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	Env                 string
	APIBasePath         string
	DatabaseURL         string
	DBMaxConns          int32
	DBMinConns          int32
	DBMaxConnLifetime   time.Duration
	DBMaxConnIdleTime   time.Duration
	DBHealthCheckPeriod time.Duration
	JWTSecret           string
	JWTAccessExpiry     time.Duration
	JWTRefreshExpiry    time.Duration
	CacheTTL            time.Duration
	CacheCleanupPeriod  time.Duration
}

func Load() (*Config, error) {
	// Attempt to load .env if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		Port:                getEnv("PORT", "8080"),
		Env:                 getEnv("ENV", "development"),
		APIBasePath:         getEnv("API_BASE_PATH", "/api/v1"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/multitenant_db?sslmode=disable"),
		DBMaxConns:          int32(getEnvAsInt("DB_MAX_CONNS", 25)),
		DBMinConns:          int32(getEnvAsInt("DB_MIN_CONNS", 5)),
		DBMaxConnLifetime:   getEnvAsDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		DBMaxConnIdleTime:   getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 15*time.Minute),
		DBHealthCheckPeriod: getEnvAsDuration("DB_HEALTH_CHECK_PERIOD", time.Minute),
		JWTSecret:           getEnv("JWT_SECRET", "super-secret-default-jwt-key-minimum-32-bytes!"),
		JWTAccessExpiry:     getEnvAsDuration("JWT_ACCESS_EXPIRY", 15*time.Minute),
		JWTRefreshExpiry:    getEnvAsDuration("JWT_REFRESH_EXPIRY", 168*time.Hour), // 7 days
		CacheTTL:            getEnvAsDuration("CACHE_TTL", 5*time.Minute),
		CacheCleanupPeriod:  getEnvAsDuration("CACHE_CLEANUP_INTERVAL", 10*time.Minute),
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := getEnv(key, "")
	if val, err := time.ParseDuration(valStr); err == nil {
		return val
	}
	return defaultVal
}
