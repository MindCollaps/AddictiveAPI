package main

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"

	"addictiveapi/internal/api"
	"addictiveapi/internal/auth"
	"addictiveapi/internal/config"
	"addictiveapi/internal/database"
	"addictiveapi/internal/logger"
	"addictiveapi/internal/models"
	"addictiveapi/internal/ws"
)

func main() {
	cfg := config.Load()
	appLogger := logger.New(cfg.Environment)

	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		appLogger.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		appLogger.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(&models.FriendRequest{}, &models.Follow{}); err != nil {
		appLogger.Error("failed to migrate social tables", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(&models.Notification{}); err != nil {
		appLogger.Error("failed to migrate notification tables", "error", err)
		os.Exit(1)
	}

	authService := auth.NewService(cfg.JWTSecret, cfg.JWTIssuer)
	wsHandler := ws.NewHandler(appLogger, db, authService)

	router := api.NewRouter(appLogger, db, authService, wsHandler)
	address := fmt.Sprintf(":%s", cfg.Port)

	appLogger.Info("starting server", "address", address, "database", cfg.DatabasePath)
	if err := router.Run(address); err != nil {
		appLogger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}
