package server

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rollic-leaderboard/internal/domain"
)

type populateResponse struct {
	BoardID   int64 `json:"boardId"`
	Populated int   `json:"populated"`
}

func (app *application) populateBoardHandler(c *gin.Context) {
	board := c.MustGet("board").(domain.Board)

	nStr := c.Query("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid value for n"})
		return
	}

	if err := app.store.Scores.Populate(c.Request.Context(), board.ID, n); err != nil {
		serverError(c, err)
		return
	}

	c.JSON(http.StatusOK, populateResponse{BoardID: board.ID, Populated: n})
}
