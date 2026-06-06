package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"rollic-leaderboard/internal/domain"
)

const QueryTimeoutDuration = 5 * time.Second

var ErrNotFound = errors.New("not found")

type Storage struct {
	Boards BoardRepository
	Scores ScoreRepository
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Boards: &PostgresBoardRepository{db: db},
		Scores: &PostgresScoreRepository{db: db},
	}
}

type BoardRepository interface {
	Create(ctx context.Context, board domain.Board) (domain.Board, error)
	List(ctx context.Context) ([]domain.Board, error)
	GetByID(ctx context.Context, id int64) (domain.Board, error)
	GetScheduledBoards(ctx context.Context) ([]domain.Board, error)
}

type ScoreRepository interface {
	Upsert(ctx context.Context, score domain.Score) (domain.Score, error)
	GetTopScores(ctx context.Context, boardID int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error)
	GetSurroundings(ctx context.Context, boardID int64, userID string, periodStart time.Time, n int) (domain.Surroundings, error)
	DeleteOldScores(ctx context.Context, boardID int64, periodStart time.Time) error
	Populate(ctx context.Context, boardID int64, n int) error
}
