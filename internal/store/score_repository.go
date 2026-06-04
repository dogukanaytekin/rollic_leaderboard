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

func (r *PostgresScoreRepository) GetTopScores(ctx context.Context, boardID int64, n int) ([]domain.TopScoreEntry, error) {
	query := `
		SELECT s.user_id, s.score
		FROM scores s
		JOIN boards b ON b.id = s.board_id
		WHERE s.board_id = $1
		  AND (
		      b.interval_seconds IS NULL
		      OR s.scored_at >= b.created_at
		          + floor(extract(epoch from now() - b.created_at) / b.interval_seconds)
		          * b.interval_seconds * interval '1 second'
		  )
		ORDER BY s.score DESC, s.scored_at ASC
		LIMIT $2
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query, boardID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scores []domain.TopScoreEntry
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
