package store

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"time"

	"rollic-leaderboard/internal/domain"
)

type PostgresScoreRepository struct {
	db *sql.DB
}

func (r *PostgresScoreRepository) Upsert(ctx context.Context, score domain.Score) (domain.Score, error) {
	query := `
		INSERT INTO scores (board_id, user_id, score)
		VALUES ($1, $2, $3)
		ON CONFLICT (board_id, user_id)
		DO UPDATE SET score = EXCLUDED.score, scored_at = now()
		RETURNING scored_at
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query,
		score.BoardID,
		score.UserID,
		score.Score,
	).Scan(&score.ScoredAt)
	if err != nil {
		return domain.Score{}, err
	}

	return score, nil
}

func (r *PostgresScoreRepository) GetTopScores(ctx context.Context, boardID int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error) {
	query := `
		SELECT user_id, score
		FROM scores
		WHERE board_id = $1
		  AND scored_at >= $2
		ORDER BY score DESC, scored_at ASC
		LIMIT $3
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, boardID, periodStart, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scores := make([]domain.TopScoreEntry, 0, n)
	for rows.Next() {
		var s domain.TopScoreEntry
		if err := rows.Scan(&s.UserID, &s.Score); err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return scores, nil
}

const cleanerBatchSize = 10_000

func (r *PostgresScoreRepository) Populate(ctx context.Context, boardID int64, n int) error {
	ctx, cancel := context.WithTimeout(ctx, BulkTimeoutDuration)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO scores (board_id, user_id, score)
		VALUES ($1, $2, $3)
		ON CONFLICT (board_id, user_id) DO UPDATE SET score = EXCLUDED.score, scored_at = now()
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := 1; i <= n; i++ {
		userID := fmt.Sprintf("mock_user_%d", i)
		score := rand.Int63n(1_000_000)
		if _, err := stmt.ExecContext(ctx, boardID, userID, score); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *PostgresScoreRepository) DeleteOldScores(ctx context.Context, boardID int64, periodStart time.Time) error {
	for {
		res, err := r.db.ExecContext(ctx, `
			DELETE FROM scores
			WHERE id IN (
				SELECT id FROM scores
				WHERE board_id = $1 AND scored_at < $2
				LIMIT $3
			)
		`, boardID, periodStart, cleanerBatchSize)
		if err != nil {
			return err
		}

		deleted, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if deleted < int64(cleanerBatchSize) {
			break
		}
	}
	return nil
}

func (r *PostgresScoreRepository) GetSurroundings(ctx context.Context, boardID int64, userID string, periodStart time.Time, n int) (domain.Surroundings, error) {
	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var user domain.TopScoreEntry
	var userScoredAt time.Time

	err := r.db.QueryRowContext(ctx, `
		SELECT user_id, score, scored_at FROM scores
		WHERE board_id = $1 AND user_id = $2 AND scored_at >= $3
	`, boardID, userID, periodStart).Scan(&user.UserID, &user.Score, &userScoredAt)
	if err == sql.ErrNoRows {
		return domain.Surroundings{}, ErrNotFound
	}
	if err != nil {
		return domain.Surroundings{}, err
	}

	aboveRows, err := r.db.QueryContext(ctx, `
		SELECT user_id, score FROM (
			SELECT user_id, score, scored_at FROM scores
			WHERE board_id = $1
			  AND scored_at >= $2
			  AND (score > $3 OR (score = $3 AND scored_at < $4))
			ORDER BY score ASC, scored_at DESC
			LIMIT $5
		) sub
		ORDER BY score DESC, scored_at ASC
	`, boardID, periodStart, user.Score, userScoredAt, n)
	if err != nil {
		return domain.Surroundings{}, err
	}
	defer aboveRows.Close()

	above := make([]domain.TopScoreEntry, 0, n)
	for aboveRows.Next() {
		var e domain.TopScoreEntry
		if err := aboveRows.Scan(&e.UserID, &e.Score); err != nil {
			return domain.Surroundings{}, err
		}
		above = append(above, e)
	}
	if err := aboveRows.Err(); err != nil {
		return domain.Surroundings{}, err
	}

	belowRows, err := r.db.QueryContext(ctx, `
		SELECT user_id, score FROM scores
		WHERE board_id = $1
		  AND scored_at >= $2
		  AND (score < $3 OR (score = $3 AND scored_at > $4))
		ORDER BY score DESC, scored_at ASC
		LIMIT $5
	`, boardID, periodStart, user.Score, userScoredAt, n)
	if err != nil {
		return domain.Surroundings{}, err
	}
	defer belowRows.Close()

	below := make([]domain.TopScoreEntry, 0, n)
	for belowRows.Next() {
		var e domain.TopScoreEntry
		if err := belowRows.Scan(&e.UserID, &e.Score); err != nil {
			return domain.Surroundings{}, err
		}
		below = append(below, e)
	}
	if err := belowRows.Err(); err != nil {
		return domain.Surroundings{}, err
	}

	return domain.Surroundings{
		User:  user,
		Above: above,
		Below: below,
	}, nil
}
