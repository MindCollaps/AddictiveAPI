package v1

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"addictiveapi/internal/auth"
)

func RegisterRoutes(group *gin.RouterGroup, db *gorm.DB, authService *auth.Service) {
	group.GET("/health", HealthHandler(db))
	authHandler := NewAuthHandler(db, authService)
	group.POST("/auth/register", authHandler.Register)
	group.POST("/auth/login", authHandler.Login)
}
