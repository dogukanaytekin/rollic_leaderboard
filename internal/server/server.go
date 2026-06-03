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

	r.GET("/boards", app.listBoardsHandler)
	r.POST("/boards", app.createBoardHandler)
	r.GET("/boards/:boardId", app.getBoardHandler)

	return r
}

func (app *application) Run() error {
	return app.mount().Run(":" + app.config.Port)
}
