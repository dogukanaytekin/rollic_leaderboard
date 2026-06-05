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

// ---- Create Board ----

func TestCreateBoardHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		mockCreate func(context.Context, domain.Board) (domain.Board, error)
		wantStatus int
		wantFields []string
		wantAbsent []string
	}{
		{
			name: "schedule'sız geçerli board",
			body: `{"name":"Weekly","description":"desc"}`,
			mockCreate: func(_ context.Context, b domain.Board) (domain.Board, error) {
				b.ID = 1
				b.CreatedAt = time.Now()
				return b, nil
			},
			wantStatus: http.StatusCreated,
			wantFields: []string{`"boardId"`, `"name"`, `"description"`},
			wantAbsent: []string{`"schedule"`, `"createdAt"`},
		},
		{
			name: "schedule'lı geçerli board",
			body: `{"name":"Weekly","schedule":{"type":"interval","intervalSeconds":604800}}`,
			mockCreate: func(_ context.Context, b domain.Board) (domain.Board, error) {
				b.ID = 2
				b.CreatedAt = time.Now()
				return b, nil
			},
			wantStatus: http.StatusCreated,
			wantFields: []string{`"boardId"`, `"schedule"`, `"intervalSeconds"`},
			wantAbsent: []string{`"createdAt"`},
		},
		{
			name:       "boş name → 400",
			body:       `{"name":""}`,
			wantStatus: http.StatusBadRequest,
			wantFields: []string{`"error"`},
		},
		{
			name:       "name alanı yok → 400",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "schedule type geçersiz → 400",
			body:       `{"name":"x","schedule":{"type":"cron","intervalSeconds":100}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "intervalSeconds = 0 → 400",
			body:       `{"name":"x","schedule":{"type":"interval","intervalSeconds":0}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "intervalSeconds negatif → 400",
			body:       `{"name":"x","schedule":{"type":"interval","intervalSeconds":-1}}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "geçersiz JSON → 400",
			body:       `{invalid}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "repository hatası → 500",
			body: `{"name":"x"}`,
			mockCreate: func(_ context.Context, b domain.Board) (domain.Board, error) {
				return domain.Board{}, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boardRepo := &mockBoardRepo{
				createFn: tt.mockCreate,
			}
			app := newTestApp(boardRepo, noopScoreRepo())
			w := post(app, "/boards", tt.body)

			assert.Equal(t, tt.wantStatus, statusOf(w))
			body := bodyOf(w)
			for _, f := range tt.wantFields {
				assert.Contains(t, body, f)
			}
			for _, f := range tt.wantAbsent {
				assert.NotContains(t, body, f)
			}
		})
	}
}

// ---- List Boards ----

func TestListBoardsHandler(t *testing.T) {
	tests := []struct {
		name       string
		mockList   func(context.Context) ([]domain.Board, error)
		wantStatus int
		wantBody   string
	}{
		{
			name: "board yok → boş array (null değil)",
			mockList: func(_ context.Context) ([]domain.Board, error) {
				return []domain.Board{}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   "[]",
		},
		{
			name: "boardlar var → boardId ve name döner",
			mockList: func(_ context.Context) ([]domain.Board, error) {
				return []domain.Board{
					{ID: 1, Name: "Board A"},
					{ID: 2, Name: "Board B"},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"boardId"`,
		},
		{
			name: "description ve schedule dönmez",
			mockList: func(_ context.Context) ([]domain.Board, error) {
				return []domain.Board{{ID: 1, Name: "A", Description: "desc"}}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"boardId"`,
		},
		{
			name: "repository hatası → 500",
			mockList: func(_ context.Context) ([]domain.Board, error) {
				return nil, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boardRepo := &mockBoardRepo{listFn: tt.mockList}
			app := newTestApp(boardRepo, noopScoreRepo())
			w := get(app, "/boards")

			assert.Equal(t, tt.wantStatus, statusOf(w))
			if tt.wantBody != "" {
				assert.Contains(t, bodyOf(w), tt.wantBody)
			}
			if tt.name == "description ve schedule dönmez" {
				assert.NotContains(t, bodyOf(w), `"description"`)
				assert.NotContains(t, bodyOf(w), `"schedule"`)
			}
		})
	}
}

// ---- Get Board ----

func TestGetBoardHandler(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		path        string
		mockGetByID func(context.Context, int64) (domain.Board, error)
		wantStatus  int
		wantFields  []string
		wantAbsent  []string
	}{
		{
			name: "schedule'sız board → nextResetAt yok",
			path: "/boards/1",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{ID: 1, Name: "x", CreatedAt: createdAt}, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{`"boardId"`, `"createdAt"`},
			wantAbsent: []string{`"nextResetAt"`, `"schedule"`},
		},
		{
			name: "schedule'lı board → nextResetAt var",
			path: "/boards/2",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{
					ID:        2,
					Name:      "Weekly",
					CreatedAt: createdAt,
					Schedule:  &domain.Schedule{Type: "interval", IntervalSeconds: 604800},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantFields: []string{`"nextResetAt"`, `"schedule"`, `"intervalSeconds"`},
		},
		{
			name: "olmayan board → 404",
			path: "/boards/99999",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{}, store.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
			wantFields: []string{`"error"`},
		},
		{
			name:       "geçersiz boardId → 404",
			path:       "/boards/abc",
			wantStatus: http.StatusNotFound,
		},
		{
			name: "repository hatası → 500",
			path: "/boards/1",
			mockGetByID: func(_ context.Context, id int64) (domain.Board, error) {
				return domain.Board{}, errors.New("db error")
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
			app := newTestApp(boardRepo, noopScoreRepo())
			w := get(app, tt.path)

			require.Equal(t, tt.wantStatus, statusOf(w))
			body := bodyOf(w)
			for _, f := range tt.wantFields {
				assert.Contains(t, body, f)
			}
			for _, f := range tt.wantAbsent {
				assert.NotContains(t, body, f)
			}
		})
	}
}
