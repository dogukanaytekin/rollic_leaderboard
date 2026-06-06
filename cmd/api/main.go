package main

import (
	"log"

	"rollic-leaderboard/internal/config"
	"rollic-leaderboard/internal/db"
	"rollic-leaderboard/internal/server"
	"rollic-leaderboard/internal/store"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load() // optional: env vars can be set directly in production

	cfg := config.Load()

	database, err := db.New(cfg.DB.Addr)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer database.Close()

	storage := store.NewStorage(database)

	app := server.New(cfg, storage)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
