package app

import (
	"github.com/gin-gonic/gin"

	"smem/apps/server/internal/config"
	httpmemory "smem/apps/server/internal/transport/http"
	httptransport "smem/apps/server/internal/transport/http"
)

func NewRouter(_ config.Config, handlers ...*httpmemory.MemoryHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	httptransport.RegisterHealthRoutes(r)
	if len(handlers) > 0 && handlers[0] != nil {
		handlers[0].Register(r.Group("/api/v1"))
	}
	return r
}
