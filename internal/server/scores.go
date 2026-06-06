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

type setScoreRequest struct {
	UserID string `json:"userId"`
	Score  int64  `json:"score"`
}

type setScoreResponse struct {
	BoardID int64  `json:"boardId"`
	UserID  string `json:"userId"`
	Score   int64  `json:"score"`
}

func (app *application) getTopScoresHandler(c *gin.Context) {
	board := c.MustGet("board").(domain.Board)

	nStr := c.Query("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value for n"})
		return
	}

	var periodStart time.Time
	if board.Schedule != nil {
		periodStart = calcPeriodStart(board.CreatedAt, board.Schedule.IntervalSeconds, time.Now())
	}

	scores, err := app.store.Scores.GetTopScores(c.Request.Context(), board.ID, periodStart, n)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if scores == nil {
		scores = []domain.TopScoreEntry{}
	}

	c.JSON(http.StatusOK, scores)
}

func (app *application) getScoreSurroundingsHandler(c *gin.Context) {
	board := c.MustGet("board").(domain.Board)
	userID := c.Param("userId")

	nStr := c.Query("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value for n"})
		return
	}

	var periodStart time.Time
	if board.Schedule != nil {
		periodStart = calcPeriodStart(board.CreatedAt, board.Schedule.IntervalSeconds, time.Now())
	}

	surroundings, err := app.store.Scores.GetSurroundings(c.Request.Context(), board.ID, userID, periodStart, n)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board or user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, surroundings)
}

func (app *application) setScoreHandler(c *gin.Context) {
	board := c.MustGet("board").(domain.Board)

	var req setScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	score, err := app.store.Scores.Upsert(c.Request.Context(), domain.Score{
		BoardID: board.ID,
		UserID:  req.UserID,
		Score:   req.Score,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, setScoreResponse{
		BoardID: score.BoardID,
		UserID:  score.UserID,
		Score:   score.Score,
	})
}
