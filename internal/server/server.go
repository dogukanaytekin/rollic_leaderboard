package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"rollic-leaderboard/internal/config"
	"rollic-leaderboard/internal/store"
)

type application struct {
	config *config.Config
	store  store.Storage
}

func New(cfg *config.Config, store store.Storage) *application {
	return &application{config: cfg, store: store}
}

func (app *application) mount() *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/docs", app.docsHandler)
	r.GET("/openapi.yaml", app.openapiHandler)

	boards := r.Group("/boards")
	{
		boards.GET("", app.listBoardsHandler)
		boards.POST("", app.createBoardHandler)

		board := boards.Group("/:boardId", app.boardMiddleware)
		{
			board.GET("", app.getBoardHandler)
			board.POST("/scores", app.setScoreHandler)
			board.GET("/scores", app.getTopScoresHandler)
			board.GET("/scores/:userId/surroundings", app.getScoreSurroundingsHandler)
			board.POST("/populate", app.populateBoardHandler)
		}
	}

	return r
}

func (app *application) Run() error {
	return app.mount().Run(":" + app.config.Port)
}
