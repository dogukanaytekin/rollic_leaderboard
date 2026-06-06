package worker

import (
	"context"
	"log"
	"time"

	"rollic-leaderboard/internal/store"
)

func StartCleaner(s store.Storage, interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if err := RunCleanup(s); err != nil {
				log.Printf("cleaner error: %v", err)
			}
		}
	}()
}

func RunCleanup(s store.Storage) error {
	return runCleanupAt(s, time.Now())
}

func runCleanupAt(s store.Storage, now time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	boards, err := s.Boards.GetScheduledBoards(ctx)
	if err != nil {
		return err
	}

	for _, b := range boards {
		interval := time.Duration(b.Schedule.IntervalSeconds) * time.Second
		elapsed := now.Sub(b.CreatedAt)
		n := int64(elapsed / interval)
		periodStart := b.CreatedAt.Add(time.Duration(n) * interval)

		if err := s.Scores.DeleteOldScores(ctx, b.ID, periodStart); err != nil {
			log.Printf("cleaner: board %d cleanup error: %v", b.ID, err)
		}
	}

	return nil
}
