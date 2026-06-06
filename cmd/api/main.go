package main

import (
	"log"
	"time"

	"rollic-leaderboard/internal/config"
	"rollic-leaderboard/internal/db"
	"rollic-leaderboard/internal/server"
	"rollic-leaderboard/internal/store"
	"rollic-leaderboard/internal/worker"

	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	database, err := db.New(cfg.DB.Addr)
	if err != nil {
		log.Fatalf("db connection failed: %v", err)
	}
	defer database.Close()

	storage := store.NewStorage(database)

	worker.StartCleaner(storage, 2*time.Hour)

	app := server.New(cfg, storage)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
