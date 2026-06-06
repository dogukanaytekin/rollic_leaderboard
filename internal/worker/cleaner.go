package worker

import (
	"context"
	"database/sql"
	"log"
	"time"
)

const batchSize = 10_000

func StartCleaner(db *sql.DB, interval time.Duration) {
	go func() {
		for {
			time.Sleep(interval)
			if err := RunCleanup(db); err != nil {
				log.Printf("cleaner error: %v", err)
			}
		}
	}()
}

func RunCleanup(db *sql.DB) error {
	return runCleanupAt(db, time.Now())
}

func runCleanupAt(db *sql.DB, now time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	boards, err := fetchScheduledBoards(ctx, db)
	if err != nil {
		return err
	}

	for _, b := range boards {
		interval := time.Duration(b.intervalSeconds) * time.Second
		elapsed := now.Sub(b.createdAt)
		n := int64(elapsed / interval)
		periodStart := b.createdAt.Add(time.Duration(n) * interval)

		if err := deleteOldScores(ctx, db, b.id, periodStart); err != nil {
			log.Printf("cleaner: board %d cleanup error: %v", b.id, err)
		}
	}

	return nil
}

type scheduledBoard struct {
	id              int64
	createdAt       time.Time
	intervalSeconds int64
}

func fetchScheduledBoards(ctx context.Context, db *sql.DB) ([]scheduledBoard, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, created_at, interval_seconds
		FROM boards
		WHERE interval_seconds IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []scheduledBoard
	for rows.Next() {
		var b scheduledBoard
		if err := rows.Scan(&b.id, &b.createdAt, &b.intervalSeconds); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}

func deleteOldScores(ctx context.Context, db *sql.DB, boardID int64, periodStart time.Time) error {
	for {
		res, err := db.ExecContext(ctx, `
			DELETE FROM scores
			WHERE id IN (
				SELECT id FROM scores
				WHERE board_id = $1 AND scored_at < $2
				LIMIT $3
			)
		`, boardID, periodStart, batchSize)
		if err != nil {
			return err
		}

		deleted, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if deleted < int64(batchSize) {
			break
		}
	}
	return nil
}
