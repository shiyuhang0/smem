package app

import (
	"github.com/gin-gonic/gin"

	"smem/apps/server/internal/config"
	"smem/apps/server/internal/handler"
)

func NewRouter(_ config.Config, handlers ...*handler.MemoryHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	handler.RegisterHealthRoutes(r)
	if len(handlers) > 0 && handlers[0] != nil {
		handlers[0].Register(r.Group("/api/v1"))
	}
	return r
}
