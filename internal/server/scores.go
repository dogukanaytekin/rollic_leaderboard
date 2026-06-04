package server

import (
	"errors"
	"net/http"
	"strconv"

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
	boardID, err := strconv.ParseInt(c.Param("boardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	_, err = app.store.Boards.GetByID(c.Request.Context(), boardID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nStr := c.Query("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value for n"})
		return
	}

	scores, err := app.store.Scores.GetTopScores(c.Request.Context(), boardID, n)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if scores == nil {
		scores = []domain.TopScoreEntry{}
	}

	c.JSON(http.StatusOK, scores)
}

func (app *application) setScoreHandler(c *gin.Context) {
	boardID, err := strconv.ParseInt(c.Param("boardId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	_, err = app.store.Boards.GetByID(c.Request.Context(), boardID)
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req setScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	score, err := app.store.Scores.Upsert(c.Request.Context(), domain.Score{
		BoardID: boardID,
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
