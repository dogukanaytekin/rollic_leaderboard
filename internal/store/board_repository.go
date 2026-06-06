package store

import (
	"context"
	"database/sql"

	"rollic-leaderboard/internal/domain"
)

type PostgresBoardRepository struct {
	db *sql.DB
}

func (r *PostgresBoardRepository) Create(ctx context.Context, board domain.Board) (domain.Board, error) {
	query := `
		INSERT INTO boards (name, description, schedule_type, interval_seconds)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`

	var scheduleType sql.NullString
	var intervalSeconds sql.NullInt64

	if board.Schedule != nil {
		scheduleType = sql.NullString{String: board.Schedule.Type, Valid: true}
		intervalSeconds = sql.NullInt64{Int64: board.Schedule.IntervalSeconds, Valid: true}
	}

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	err := r.db.QueryRowContext(ctx, query,
		board.Name,
		board.Description,
		scheduleType,
		intervalSeconds,
	).Scan(&board.ID, &board.CreatedAt)
	if err != nil {
		return domain.Board{}, err
	}

	return board, nil
}

func (r *PostgresBoardRepository) List(ctx context.Context) ([]domain.Board, error) {
	query := `SELECT id, name FROM boards`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []domain.Board
	for rows.Next() {
		var b domain.Board
		if err := rows.Scan(&b.ID, &b.Name); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return boards, nil
}

func (r *PostgresBoardRepository) GetByID(ctx context.Context, id int64) (domain.Board, error) {
	query := `
		SELECT id, name, description, created_at, schedule_type, interval_seconds
		FROM boards
		WHERE id = $1
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	var b domain.Board
	var scheduleType sql.NullString
	var intervalSeconds sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.Name, &b.Description, &b.CreatedAt, &scheduleType, &intervalSeconds,
	)
	if err == sql.ErrNoRows {
		return domain.Board{}, ErrNotFound
	}
	if err != nil {
		return domain.Board{}, err
	}

	if scheduleType.Valid {
		b.Schedule = &domain.Schedule{
			Type:            scheduleType.String,
			IntervalSeconds: intervalSeconds.Int64,
		}
	}

	return b, nil
}

func (r *PostgresBoardRepository) GetScheduledBoards(ctx context.Context) ([]domain.Board, error) {
	query := `
		SELECT id, created_at, interval_seconds
		FROM boards
		WHERE interval_seconds IS NOT NULL
	`

	ctx, cancel := context.WithTimeout(ctx, QueryTimeoutDuration)
	defer cancel()

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []domain.Board
	for rows.Next() {
		var b domain.Board
		var intervalSeconds sql.NullInt64
		if err := rows.Scan(&b.ID, &b.CreatedAt, &intervalSeconds); err != nil {
			return nil, err
		}
		if intervalSeconds.Valid {
			b.Schedule = &domain.Schedule{
				Type:            "interval",
				IntervalSeconds: intervalSeconds.Int64,
			}
		}
		boards = append(boards, b)
	}
	return boards, rows.Err()
}
