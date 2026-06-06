package server

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"rollic-leaderboard/internal/domain"
)

type createBoardRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Schedule    *domain.Schedule `json:"schedule"`
}

type createBoardResponse struct {
	BoardID     int64            `json:"boardId"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Schedule    *domain.Schedule `json:"schedule,omitempty"`
}

type listBoardResponse struct {
	BoardID int64  `json:"boardId"`
	Name    string `json:"name"`
}

type getBoardResponse struct {
	BoardID     int64            `json:"boardId"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	Schedule    *domain.Schedule `json:"schedule,omitempty"`
	NextResetAt *time.Time       `json:"nextResetAt,omitempty"`
}

func calcPeriodStart(createdAt time.Time, intervalSeconds int64, now time.Time) time.Time {
	interval := time.Duration(intervalSeconds) * time.Second
	elapsed := now.Sub(createdAt)
	n := int64(elapsed / interval)
	return createdAt.Add(time.Duration(n) * interval)
}

func calcNextResetAt(createdAt time.Time, intervalSeconds int64, now time.Time) time.Time {
	return calcPeriodStart(createdAt, intervalSeconds, now).Add(time.Duration(intervalSeconds) * time.Second)
}

func (app *application) getBoardHandler(c *gin.Context) {
	board := c.MustGet("board").(domain.Board)

	resp := getBoardResponse{
		BoardID:     board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
		Schedule:    board.Schedule,
	}

	if board.Schedule != nil {
		t := calcNextResetAt(board.CreatedAt, board.Schedule.IntervalSeconds, time.Now())
		resp.NextResetAt = &t
	}

	c.JSON(http.StatusOK, resp)
}

func (app *application) listBoardsHandler(c *gin.Context) {
	boards, err := app.store.Boards.List(c.Request.Context())
	if err != nil {
		serverError(c, err)
		return
	}

	resp := make([]listBoardResponse, len(boards))
	for i, b := range boards {
		resp[i] = listBoardResponse{BoardID: b.ID, Name: b.Name}
	}

	c.JSON(http.StatusOK, resp)
}

func (app *application) createBoardHandler(c *gin.Context) {
	var req createBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid board name"})
		return
	}

	if req.Schedule != nil {
		if req.Schedule.Type != "interval" || req.Schedule.IntervalSeconds <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid schedule"})
			return
		}
	}

	board, err := app.store.Boards.Create(c.Request.Context(), domain.Board{
		Name:        req.Name,
		Description: req.Description,
		Schedule:    req.Schedule,
	})
	if err != nil {
		serverError(c, err)
		return
	}

	c.JSON(http.StatusCreated, createBoardResponse{
		BoardID:     board.ID,
		Name:        board.Name,
		Description: board.Description,
		Schedule:    board.Schedule,
	})
}
