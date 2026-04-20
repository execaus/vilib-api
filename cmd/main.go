package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vilib-api/config"
	"vilib-api/internal/handler"
	"vilib-api/internal/repository"
	"vilib-api/internal/saga"
	"vilib-api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		zap.L().Fatal("failed to run application", zap.Error(err))
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	gin.SetMode(string(cfg.Server.Mode))
	if cfg.Server.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	db, pool, err := repository.NewPostgresDB(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	executorProvider := repository.NewExecutorProvider(db)
	repo := repository.NewRepository(executorProvider)

	localMailBox := make(chan string, 1)
	svc := service.NewService(cfg, localMailBox, nil, repo)

	sagaRunner := saga.NewSagaRunner(svc, executorProvider)
	h := handler.NewHandler(sagaRunner)
	router := h.GetRouter()

	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	go func() {
		zap.L().Info("starting server", zap.String("port", cfg.Server.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Error("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	zap.L().Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server forced to shutdown: %w", err)
	}

	zap.L().Info("server exited properly")
	return nil
}
