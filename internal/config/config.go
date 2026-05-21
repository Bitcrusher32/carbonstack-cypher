package config

import "os"

type Config struct {
	Addr          string
	DBPath        string
	MigrationsDir string
	DevInviteCode string
}

func Load() Config {
	return Config{
		Addr:          getEnv("CYPHER_ADDR", ":8080"),
		DBPath:        getEnv("CYPHER_DB", "cypher.db"),
		MigrationsDir: getEnv("CYPHER_MIGRATIONS", "migrations"),
		DevInviteCode: getEnv("CYPHER_DEV_INVITE", "dev-invite"),
	}
}

func getEnv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
