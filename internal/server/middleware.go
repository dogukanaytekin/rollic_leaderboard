package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"rollic-leaderboard/internal/store"
)

func (app *application) boardMiddleware(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("boardId"), 10, 64)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}

	board, err := app.store.Boards.GetByID(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Board not found"})
		return
	}
	if err != nil {
		serverError(c, err)
		return
	}

	c.Set("board", board)
	c.Next()
}
