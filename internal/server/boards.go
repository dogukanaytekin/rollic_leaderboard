package server

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"rollic-leaderboard/internal/domain"
	"rollic-leaderboard/internal/store"
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

func calcNextResetAt(createdAt time.Time, intervalSeconds int64) time.Time {
	interval := time.Duration(intervalSeconds) * time.Second
	elapsed := time.Since(createdAt)
	n := int64(elapsed/interval) + 1
	return createdAt.Add(time.Duration(n) * interval)
}

func (app *application) getBoardHandler(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("boardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	board, err := app.store.Boards.GetByID(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	resp := getBoardResponse{
		BoardID:     board.ID,
		Name:        board.Name,
		Description: board.Description,
		CreatedAt:   board.CreatedAt,
		Schedule:    board.Schedule,
	}

	if board.Schedule != nil {
		t := calcNextResetAt(board.CreatedAt, board.Schedule.IntervalSeconds)
		resp.NextResetAt = &t
	}

	c.JSON(http.StatusOK, resp)
}

func (app *application) listBoardsHandler(c *gin.Context) {
	boards, err := app.store.Boards.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, createBoardResponse{
		BoardID:     board.ID,
		Name:        board.Name,
		Description: board.Description,
		Schedule:    board.Schedule,
	})
}
