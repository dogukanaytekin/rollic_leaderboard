package store

import (
	"context"
	"database/sql"

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
