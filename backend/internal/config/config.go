package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr            string
	DatabaseURL     string
	DBMaxOpenConns  int
	DBMaxIdleConns  int
	DBConnMaxIdle   time.Duration
	DBConnMaxLife   time.Duration
	MigrationsPath  string
	EnableDBStorage bool
}

func Load() Config {
	addr := getenvDefault("SYNAPSE_ADDR", ":8080")
	databaseURL := strings.TrimSpace(os.Getenv("SYNAPSE_DATABASE_URL"))
	return Config{
		Addr:            addr,
		DatabaseURL:     databaseURL,
		DBMaxOpenConns:  getenvInt("SYNAPSE_DB_MAX_OPEN_CONNS", 10),
		DBMaxIdleConns:  getenvInt("SYNAPSE_DB_MAX_IDLE_CONNS", 5),
		DBConnMaxIdle:   getenvDuration("SYNAPSE_DB_CONN_MAX_IDLE", 15*time.Minute),
		DBConnMaxLife:   getenvDuration("SYNAPSE_DB_CONN_MAX_LIFE", time.Hour),
		MigrationsPath:  getenvDefault("SYNAPSE_MIGRATIONS_PATH", "migrations"),
		EnableDBStorage: databaseURL != "",
	}
}

func (c Config) ValidateDB() error {
	if !c.EnableDBStorage {
		return fmt.Errorf("database storage disabled: SYNAPSE_DATABASE_URL not set")
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
