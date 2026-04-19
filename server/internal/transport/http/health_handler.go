package http

import "github.com/gin-gonic/gin"

func RegisterHealthRoutes(router gin.IRoutes) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
