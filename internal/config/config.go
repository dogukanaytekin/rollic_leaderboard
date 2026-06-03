package config

import "os"

type Config struct {
	Port string
	DB   DBConfig
}

type DBConfig struct {
	Addr string
}

func Load() *Config {
	return &Config{
		Port: os.Getenv("PORT"),
		DB: DBConfig{
			Addr: os.Getenv("DB_ADDR"),
		},
	}
}
