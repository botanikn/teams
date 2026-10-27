package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr      string
	MySQLDSN      string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	JWTSecret     string
	JWTTTL        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      env("HTTP_ADDR", "0.0.0.0:8080"),
		MySQLDSN:      os.Getenv("MYSQL_DSN"),
		RedisAddr:     env("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTTTL:        24 * time.Hour,
	}
	redisDB, err := strconv.Atoi(env("REDIS_DB", "0"))
	if err != nil || redisDB < 0 {
		return Config{}, fmt.Errorf("REDIS_DB must be a non-negative integer")
	}
	cfg.RedisDB = redisDB
	if ttl := os.Getenv("JWT_TTL"); ttl != "" {
		cfg.JWTTTL, err = time.ParseDuration(ttl)
		if err != nil || cfg.JWTTTL <= 0 {
			return Config{}, fmt.Errorf("JWT_TTL must be a positive duration")
		}
	}
	if cfg.MySQLDSN == "" {
		return Config{}, fmt.Errorf("MYSQL_DSN is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
