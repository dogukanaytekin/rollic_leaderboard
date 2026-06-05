package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"rollic-leaderboard/internal/domain"
	"rollic-leaderboard/internal/store"
)

// ---- Set Score ----

func TestSetScoreHandler(t *testing.T) {
	board := fixedBoard(1, false)

	tests := []struct {
		name        string
		path        string
		body        string
		mockGetByID func(context.Context, int64) (domain.Board, error)
		mockUpsert  func(context.Context, domain.Score) (domain.Score, error)
		wantStatus  int
		wantFields  []string
	}{
		{
			name:        "geçerli score → 200 + boardId, userId, score",
			path:        "/boards/1/scores",
			body:        `{"userId":"user_A","score":1500}`,
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockUpsert:  func(_ context.Context, s domain.Score) (domain.Score, error) { return s, nil },
			wantStatus:  http.StatusOK,
			wantFields:  []string{`"boardId"`, `"userId"`, `"score"`},
		},
		{
			name:        "overwrite — aynı user yeni score",
			path:        "/boards/1/scores",
			body:        `{"userId":"user_A","score":9999}`,
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockUpsert: func(_ context.Context, s domain.Score) (domain.Score, error) {
				return domain.Score{BoardID: 1, UserID: "user_A", Score: 9999}, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{"9999"},
		},
		{
			name: "olmayan board → 404",
			path: "/boards/99999/scores",
			body: `{"userId":"user_A","score":100}`,
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{}, store.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
			wantFields: []string{`"error"`},
		},
		{
			name:       "geçersiz boardId → 404",
			path:       "/boards/abc/scores",
			body:       `{"userId":"user_A","score":100}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:        "geçersiz JSON → 400",
			path:        "/boards/1/scores",
			body:        `{invalid}`,
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "repository upsert hatası → 500",
			path:        "/boards/1/scores",
			body:        `{"userId":"user_A","score":100}`,
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockUpsert: func(_ context.Context, s domain.Score) (domain.Score, error) {
				return domain.Score{}, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boardRepo := &mockBoardRepo{
				getByIDFn: func(ctx context.Context, id int64) (domain.Board, error) {
					if tt.mockGetByID != nil {
						return tt.mockGetByID(ctx, id)
					}
					return domain.Board{}, store.ErrNotFound
				},
			}
			scoreRepo := &mockScoreRepo{
				upsertFn: func(ctx context.Context, s domain.Score) (domain.Score, error) {
					if tt.mockUpsert != nil {
						return tt.mockUpsert(ctx, s)
					}
					return s, nil
				},
				getTopScoresFn:    noopScoreRepo().getTopScoresFn,
				getSurroundingsFn: noopScoreRepo().getSurroundingsFn,
			}
			app := newTestApp(boardRepo, scoreRepo)
			w := post(app, tt.path, tt.body)

			require.Equal(t, tt.wantStatus, statusOf(w))
			body := bodyOf(w)
			for _, f := range tt.wantFields {
				assert.Contains(t, body, f)
			}
		})
	}
}

// ---- Get Top Scores ----

func TestGetTopScoresHandler(t *testing.T) {
	board := fixedBoard(1, false)

	tests := []struct {
		name           string
		path           string
		mockGetByID    func(context.Context, int64) (domain.Board, error)
		mockGetTop     func(context.Context, int64, time.Time, int) ([]domain.TopScoreEntry, error)
		wantStatus     int
		wantBody       string
		wantNotContain string
	}{
		{
			name:        "skorlar var → sıralı döner",
			path:        "/boards/1/scores?n=3",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockGetTop: func(_ context.Context, _ int64, _ time.Time, n int) ([]domain.TopScoreEntry, error) {
				return []domain.TopScoreEntry{
					{UserID: "user_A", Score: 5000},
					{UserID: "user_B", Score: 3000},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"user_A"`,
		},
		{
			name:        "board boş → [] döner (null değil)",
			path:        "/boards/1/scores?n=5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockGetTop: func(_ context.Context, _ int64, _ time.Time, n int) ([]domain.TopScoreEntry, error) {
				return []domain.TopScoreEntry{}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
		{
			name:        "n > katılımcı sayısı → kaç varsa döner",
			path:        "/boards/1/scores?n=100",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockGetTop: func(_ context.Context, _ int64, _ time.Time, n int) ([]domain.TopScoreEntry, error) {
				return []domain.TopScoreEntry{{UserID: "user_A", Score: 100}}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"user_A"`,
		},
		{
			name: "schedule'lı board → periodStart hesaplanır",
			path: "/boards/1/scores?n=5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return fixedBoard(1, true), nil
			},
			mockGetTop: func(_ context.Context, _ int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error) {
				assert.False(t, periodStart.IsZero(), "schedule varsa periodStart sıfır olmamalı")
				return []domain.TopScoreEntry{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "schedule'sız board → periodStart zero value geçer",
			path:        "/boards/1/scores?n=5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockGetTop: func(_ context.Context, _ int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error) {
				assert.True(t, periodStart.IsZero(), "schedule yoksa periodStart sıfır olmalı")
				return []domain.TopScoreEntry{}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "n=0 → 400",
			path:        "/boards/1/scores?n=0",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "n negatif → 400",
			path:        "/boards/1/scores?n=-5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "n string → 400",
			path:        "/boards/1/scores?n=abc",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "n parametresi yok → 400",
			path:        "/boards/1/scores",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name: "olmayan board → 404",
			path: "/boards/99999/scores?n=5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{}, store.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "geçersiz boardId → 404",
			path:       "/boards/abc/scores?n=5",
			wantStatus: http.StatusNotFound,
		},
		{
			name:        "repository hatası → 500",
			path:        "/boards/1/scores?n=5",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) { return board, nil },
			mockGetTop: func(_ context.Context, _ int64, _ time.Time, _ int) ([]domain.TopScoreEntry, error) {
				return nil, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boardRepo := &mockBoardRepo{
				getByIDFn: func(ctx context.Context, id int64) (domain.Board, error) {
					if tt.mockGetByID != nil {
						return tt.mockGetByID(ctx, id)
					}
					return domain.Board{}, store.ErrNotFound
				},
			}
			scoreRepo := &mockScoreRepo{
				upsertFn: noopScoreRepo().upsertFn,
				getTopScoresFn: func(ctx context.Context, id int64, ps time.Time, n int) ([]domain.TopScoreEntry, error) {
					if tt.mockGetTop != nil {
						return tt.mockGetTop(ctx, id, ps, n)
					}
					return []domain.TopScoreEntry{}, nil
				},
				getSurroundingsFn: noopScoreRepo().getSurroundingsFn,
			}
			app := newTestApp(boardRepo, scoreRepo)
			w := get(app, tt.path)

			require.Equal(t, tt.wantStatus, statusOf(w))
			if tt.wantBody != "" {
				assert.Contains(t, bodyOf(w), tt.wantBody)
			}
		})
	}
}

// ---- Get Score Surroundings ----

func TestGetScoreSurroundingsHandler(t *testing.T) {
	board := fixedBoard(1, false)

	validSurroundings := domain.Surroundings{
		User:  domain.TopScoreEntry{UserID: "user_B", Score: 3000},
		Above: []domain.TopScoreEntry{{UserID: "user_A", Score: 5000}},
		Below: []domain.TopScoreEntry{{UserID: "user_C", Score: 1000}},
	}

	tests := []struct {
		name                string
		path                string
		mockGetByID         func(context.Context, int64) (domain.Board, error)
		mockGetSurroundings func(context.Context, int64, string, time.Time, int) (domain.Surroundings, error)
		wantStatus          int
		wantFields          []string
	}{
		{
			name:        "geçerli surroundings → user, above, below döner",
			path:        "/boards/1/scores/user_B/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
				return validSurroundings, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{`"user"`, `"above"`, `"below"`, `"user_B"`, `"user_A"`, `"user_C"`},
		},
		{
			name:        "en üstteki kullanıcı → above boş array",
			path:        "/boards/1/scores/user_A/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
				return domain.Surroundings{
					User:  domain.TopScoreEntry{UserID: "user_A", Score: 5000},
					Above: []domain.TopScoreEntry{},
					Below: []domain.TopScoreEntry{{UserID: "user_B", Score: 3000}},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{`"above":[]`},
		},
		{
			name:        "en alttaki kullanıcı → below boş array",
			path:        "/boards/1/scores/user_Z/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
				return domain.Surroundings{
					User:  domain.TopScoreEntry{UserID: "user_Z", Score: 100},
					Above: []domain.TopScoreEntry{{UserID: "user_B", Score: 3000}},
					Below: []domain.TopScoreEntry{},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{`"below":[]`},
		},
		{
			name: "schedule'lı board → periodStart sıfır değil",
			path: "/boards/1/scores/user_A/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) {
				return fixedBoard(1, true), nil
			},
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, periodStart time.Time, _ int) (domain.Surroundings, error) {
				assert.False(t, periodStart.IsZero(), "schedule varsa periodStart sıfır olmamalı")
				return validSurroundings, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:        "olmayan kullanıcı → 404",
			path:        "/boards/1/scores/nobody/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
				return domain.Surroundings{}, store.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
			wantFields: []string{`"Board or user not found"`},
		},
		{
			name: "olmayan board → 404",
			path: "/boards/99999/scores/user_A/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) {
				return domain.Board{}, store.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "geçersiz boardId → 404",
			path:       "/boards/abc/scores/user_A/surroundings?n=1",
			wantStatus: http.StatusNotFound,
		},
		{
			name:        "n=0 → 400",
			path:        "/boards/1/scores/user_A/surroundings?n=0",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "n negatif → 400",
			path:        "/boards/1/scores/user_A/surroundings?n=-1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "n yok → 400",
			path:        "/boards/1/scores/user_A/surroundings",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "repository hatası → 500",
			path:        "/boards/1/scores/user_A/surroundings?n=1",
			mockGetByID: func(_ context.Context, _ int64) (domain.Board, error) { return board, nil },
			mockGetSurroundings: func(_ context.Context, _ int64, _ string, _ time.Time, _ int) (domain.Surroundings, error) {
				return domain.Surroundings{}, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boardRepo := &mockBoardRepo{
				getByIDFn: func(ctx context.Context, id int64) (domain.Board, error) {
					if tt.mockGetByID != nil {
						return tt.mockGetByID(ctx, id)
					}
					return domain.Board{}, store.ErrNotFound
				},
			}
			scoreRepo := &mockScoreRepo{
				upsertFn:       noopScoreRepo().upsertFn,
				getTopScoresFn: noopScoreRepo().getTopScoresFn,
				getSurroundingsFn: func(ctx context.Context, boardID int64, userID string, ps time.Time, n int) (domain.Surroundings, error) {
					if tt.mockGetSurroundings != nil {
						return tt.mockGetSurroundings(ctx, boardID, userID, ps, n)
					}
					return domain.Surroundings{}, store.ErrNotFound
				},
			}
			app := newTestApp(boardRepo, scoreRepo)
			w := get(app, tt.path)

			require.Equal(t, tt.wantStatus, statusOf(w))
			body := bodyOf(w)
			for _, f := range tt.wantFields {
				assert.Contains(t, body, f)
			}
		})
	}
}
