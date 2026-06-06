package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port string
	DB   DBConfig
}

type DBConfig struct {
	Addr string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		return nil, fmt.Errorf("PORT is required")
	}

	dbAddr := os.Getenv("DB_ADDR")
	if dbAddr == "" {
		return nil, fmt.Errorf("DB_ADDR is required")
	}

	return &Config{
		Port: port,
		DB:   DBConfig{Addr: dbAddr},
	}, nil
}
