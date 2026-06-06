package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"rollic-leaderboard/internal/config"
	"rollic-leaderboard/internal/domain"
	"rollic-leaderboard/internal/store"
)

type mockBoardRepo struct {
	createFn  func(context.Context, domain.Board) (domain.Board, error)
	listFn    func(context.Context) ([]domain.Board, error)
	getByIDFn func(context.Context, int64) (domain.Board, error)
}

func (m *mockBoardRepo) Create(ctx context.Context, b domain.Board) (domain.Board, error) {
	return m.createFn(ctx, b)
}
func (m *mockBoardRepo) List(ctx context.Context) ([]domain.Board, error) {
	return m.listFn(ctx)
}
func (m *mockBoardRepo) GetByID(ctx context.Context, id int64) (domain.Board, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockBoardRepo) GetScheduledBoards(ctx context.Context) ([]domain.Board, error) {
	return []domain.Board{}, nil
}

type mockScoreRepo struct {
	upsertFn          func(context.Context, domain.Score) (domain.Score, error)
	getTopScoresFn    func(context.Context, int64, time.Time, int) ([]domain.TopScoreEntry, error)
	getSurroundingsFn func(context.Context, int64, string, time.Time, int) (domain.Surroundings, error)
}

func (m *mockScoreRepo) Upsert(ctx context.Context, s domain.Score) (domain.Score, error) {
	return m.upsertFn(ctx, s)
}
func (m *mockScoreRepo) GetTopScores(ctx context.Context, boardID int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error) {
	return m.getTopScoresFn(ctx, boardID, periodStart, n)
}
func (m *mockScoreRepo) GetSurroundings(ctx context.Context, boardID int64, userID string, periodStart time.Time, n int) (domain.Surroundings, error) {
	return m.getSurroundingsFn(ctx, boardID, userID, periodStart, n)
}
func (m *mockScoreRepo) DeleteOldScores(ctx context.Context, boardID int64, periodStart time.Time) error {
	return nil
}
func (m *mockScoreRepo) Populate(ctx context.Context, boardID int64, n int) error {
	return nil
}

func newTestApp(boards store.BoardRepository, scores store.ScoreRepository) *application {
	return &application{
		config: &config.Config{Port: "8081"},
		store:  store.Storage{Boards: boards, Scores: scores},
	}
}

func doRequest(app *application, method, path, body string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	app.mount().ServeHTTP(w, req)
	return w
}

func fixedBoard(id int64, withSchedule bool) domain.Board {
	b := domain.Board{
		ID:          id,
		Name:        "Test Board",
		Description: "desc",
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if withSchedule {
		b.Schedule = &domain.Schedule{Type: "interval", IntervalSeconds: 604800}
	}
	return b
}

func noopScoreRepo() *mockScoreRepo {
	return &mockScoreRepo{
		upsertFn: func(ctx context.Context, s domain.Score) (domain.Score, error) {
			return s, nil
		},
		getTopScoresFn: func(ctx context.Context, boardID int64, periodStart time.Time, n int) ([]domain.TopScoreEntry, error) {
			return []domain.TopScoreEntry{}, nil
		},
		getSurroundingsFn: func(ctx context.Context, boardID int64, userID string, periodStart time.Time, n int) (domain.Surroundings, error) {
			return domain.Surroundings{}, nil
		},
	}
}

func boardRepoReturning(b domain.Board) *mockBoardRepo {
	return &mockBoardRepo{
		createFn:  func(ctx context.Context, in domain.Board) (domain.Board, error) { return b, nil },
		listFn:    func(ctx context.Context) ([]domain.Board, error) { return []domain.Board{b}, nil },
		getByIDFn: func(ctx context.Context, id int64) (domain.Board, error) { return b, nil },
	}
}

func boardRepoNotFound() *mockBoardRepo {
	return &mockBoardRepo{
		createFn:  func(ctx context.Context, in domain.Board) (domain.Board, error) { return domain.Board{}, nil },
		listFn:    func(ctx context.Context) ([]domain.Board, error) { return []domain.Board{}, nil },
		getByIDFn: func(ctx context.Context, id int64) (domain.Board, error) { return domain.Board{}, store.ErrNotFound },
	}
}

func statusOf(w *httptest.ResponseRecorder) int {
	return w.Result().StatusCode
}

func bodyOf(w *httptest.ResponseRecorder) string {
	b, _ := io.ReadAll(w.Result().Body)
	return string(b)
}

func get(app *application, path string) *httptest.ResponseRecorder {
	return doRequest(app, http.MethodGet, path, "")
}

func post(app *application, path, body string) *httptest.ResponseRecorder {
	return doRequest(app, http.MethodPost, path, body)
}
