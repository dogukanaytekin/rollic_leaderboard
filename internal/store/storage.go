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
}

func NewStorage(db *sql.DB) Storage {
	return Storage{
		Boards: &PostgresBoardRepository{db: db},
	}
}

type BoardRepository interface {
	Create(ctx context.Context, board domain.Board) (domain.Board, error)
	List(ctx context.Context) ([]domain.Board, error)
	GetByID(ctx context.Context, id int64) (domain.Board, error)
}
