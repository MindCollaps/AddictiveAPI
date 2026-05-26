package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	v1 "addictiveapi/internal/api/v1"
	"addictiveapi/internal/auth"
	"addictiveapi/internal/logger"
	"addictiveapi/internal/ws"
)

func NewRouter(log *slog.Logger, db *gorm.DB, authService *auth.Service, wsHandler *ws.Handler) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(logger.Middleware(log))

	router.GET("/health", v1.HealthHandler(db))
	router.GET("/ws", ws.JWTMiddleware(authService), wsHandler.ServeWS)

	v1Group := router.Group("/api/v1")
	v1.RegisterRoutes(v1Group, db, authService)

	return router
}
