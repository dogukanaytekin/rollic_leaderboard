package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rollic-leaderboard/internal/domain"
	"rollic-leaderboard/internal/store"
)

var fixedNow = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

const weekSeconds = int64(604800)

type mockBoardRepo struct {
	getScheduledBoardsFn func(context.Context) ([]domain.Board, error)
}

func (m *mockBoardRepo) Create(_ context.Context, b domain.Board) (domain.Board, error) {
	return b, nil
}
func (m *mockBoardRepo) List(_ context.Context) ([]domain.Board, error) { return nil, nil }
func (m *mockBoardRepo) GetByID(_ context.Context, _ int64) (domain.Board, error) {
	return domain.Board{}, nil
}
func (m *mockBoardRepo) GetScheduledBoards(ctx context.Context) ([]domain.Board, error) {
	return m.getScheduledBoardsFn(ctx)
}

type mockScoreRepo struct {
	deleteOldScoresFn func(context.Context, int64, time.Time) error
}

func (m *mockScoreRepo) Upsert(_ context.Context, s domain.Score) (domain.Score, error) {
	return s, nil
}
func (m *mockScoreRepo) GetTopScores(_ context.Context, _ int64, _ time.Time, _ int) ([]domain.TopScoreEntry, error) {
	return nil, nil
}
func (m *mockScoreRepo) GetSurroundings(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
	return domain.Surroundings{}, nil
}
func (m *mockScoreRepo) DeleteOldScores(ctx context.Context, boardID int64, periodStart time.Time) error {
	return m.deleteOldScoresFn(ctx, boardID, periodStart)
}
func (m *mockScoreRepo) Populate(_ context.Context, _ int64, _ int) error { return nil }

func newStorage(boards store.BoardRepository, scores store.ScoreRepository) store.Storage {
	return store.Storage{Boards: boards, Scores: scores}
}

func TestRunCleanupAt_NoScheduledBoards(t *testing.T) {
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return []domain.Board{}, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, _ time.Time) error {
			t.Fatal("DeleteOldScores should not be called")
			return nil
		}},
	)

	err := runCleanupAt(s, fixedNow)
	assert.NoError(t, err)
}

func TestRunCleanupAt_BoardInFirstPeriod_NothingDeleted(t *testing.T) {

	createdAt := fixedNow.Add(-3 * 24 * time.Hour)
	board := domain.Board{
		ID:        1,
		CreatedAt: createdAt,
		Schedule:  &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds},
	}

	var capturedPeriodStart time.Time
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return []domain.Board{board}, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, ps time.Time) error {
			capturedPeriodStart = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, createdAt, capturedPeriodStart, "1. periyotta periodStart createdAt olmalı")
}

func TestRunCleanupAt_BoardInSecondPeriod_CorrectPeriodStart(t *testing.T) {

	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	expectedPeriodStart := createdAt.Add(time.Duration(weekSeconds) * time.Second)

	board := domain.Board{
		ID:        1,
		CreatedAt: createdAt,
		Schedule:  &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds},
	}

	var capturedPeriodStart time.Time
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return []domain.Board{board}, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, ps time.Time) error {
			capturedPeriodStart = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, expectedPeriodStart, capturedPeriodStart)
}

func TestRunCleanupAt_ThirdPeriod_CorrectPeriodStart(t *testing.T) {

	createdAt := fixedNow.Add(-20 * 24 * time.Hour)
	expectedPeriodStart := createdAt.Add(2 * time.Duration(weekSeconds) * time.Second)

	board := domain.Board{
		ID:        1,
		CreatedAt: createdAt,
		Schedule:  &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds},
	}

	var capturedPeriodStart time.Time
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return []domain.Board{board}, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, ps time.Time) error {
			capturedPeriodStart = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, expectedPeriodStart, capturedPeriodStart)
}

func TestRunCleanupAt_PeriodBoundary_ExactlyAtInterval(t *testing.T) {

	createdAt := fixedNow.Add(-time.Duration(weekSeconds) * time.Second)
	expectedPeriodStart := fixedNow

	board := domain.Board{
		ID:        1,
		CreatedAt: createdAt,
		Schedule:  &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds},
	}

	var capturedPeriodStart time.Time
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return []domain.Board{board}, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, ps time.Time) error {
			capturedPeriodStart = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, expectedPeriodStart, capturedPeriodStart)
}

func TestRunCleanupAt_MultipleBoards_EachGetsCorrectPeriodStart(t *testing.T) {
	createdAt1 := fixedNow.Add(-10 * 24 * time.Hour)
	createdAt2 := fixedNow.Add(-20 * 24 * time.Hour)

	expectedPS1 := createdAt1.Add(time.Duration(weekSeconds) * time.Second)
	expectedPS2 := createdAt2.Add(2 * time.Duration(weekSeconds) * time.Second)

	boards := []domain.Board{
		{ID: 1, CreatedAt: createdAt1, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds}},
		{ID: 2, CreatedAt: createdAt2, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds}},
	}

	calls := map[int64]time.Time{}
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return boards, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, boardID int64, ps time.Time) error {
			calls[boardID] = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, expectedPS1, calls[1])
	assert.Equal(t, expectedPS2, calls[2])
}

func TestRunCleanupAt_DifferentIntervals(t *testing.T) {
	dailyInterval := int64(86400)
	monthlyInterval := int64(2592000)

	createdAt1 := fixedNow.Add(-3 * 24 * time.Hour)
	createdAt2 := fixedNow.Add(-45 * 24 * time.Hour)
	expectedPS1 := createdAt1.Add(3 * time.Duration(dailyInterval) * time.Second)
	expectedPS2 := createdAt2.Add(time.Duration(monthlyInterval) * time.Second)

	boards := []domain.Board{
		{ID: 1, CreatedAt: createdAt1, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: dailyInterval}},
		{ID: 2, CreatedAt: createdAt2, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: monthlyInterval}},
	}

	calls := map[int64]time.Time{}
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return boards, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, boardID int64, ps time.Time) error {
			calls[boardID] = ps
			return nil
		}},
	)

	require.NoError(t, runCleanupAt(s, fixedNow))
	assert.Equal(t, expectedPS1, calls[1])
	assert.Equal(t, expectedPS2, calls[2])
}

func TestRunCleanupAt_FetchError_ReturnsError(t *testing.T) {
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return nil, errors.New("db connection lost")
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, _ int64, _ time.Time) error {
			return nil
		}},
	)

	err := runCleanupAt(s, fixedNow)
	assert.Error(t, err)
}

func TestRunCleanupAt_DeleteError_LogsAndContinues(t *testing.T) {
	createdAt := fixedNow.Add(-10 * 24 * time.Hour)
	boards := []domain.Board{
		{ID: 1, CreatedAt: createdAt, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds}},
		{ID: 2, CreatedAt: createdAt, Schedule: &domain.Schedule{Type: "interval", IntervalSeconds: weekSeconds}},
	}

	deletedBoards := []int64{}
	s := newStorage(
		&mockBoardRepo{getScheduledBoardsFn: func(_ context.Context) ([]domain.Board, error) {
			return boards, nil
		}},
		&mockScoreRepo{deleteOldScoresFn: func(_ context.Context, boardID int64, _ time.Time) error {
			if boardID == 1 {
				return errors.New("timeout")
			}
			deletedBoards = append(deletedBoards, boardID)
			return nil
		}},
	)

	err := runCleanupAt(s, fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, []int64{2}, deletedBoards, "board 2 hata olsa da temizlenmeli")
}
