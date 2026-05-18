package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/godfreygan/ai-script/backend/internal/conf"
	"github.com/godfreygan/ai-script/backend/internal/server"
	"github.com/godfreygan/ai-script/backend/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	cfg, err := conf.Load("./configs/config.yaml")
	if err != nil {
		panic(fmt.Sprintf("load config failed: %v", err))
	}
	if err := cfg.Validate(); err != nil {
		panic(fmt.Sprintf("config validation failed: %v", err))
	}

	log, err := logger.New(cfg.App.LogLevel, cfg.App.Env)
	if err != nil {
		panic(err)
	}
	defer log.Sync()

	app, err := server.InitializeApp(cfg, log)
	if err != nil {
		log.Fatal("init app failed", zap.Error(err))
	}
	defer app.Close()

	srv := app.Server()

	go func() {
		log.Info("http server started", zap.Int("port", cfg.App.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("http server crashed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server forced shutdown", zap.Error(err))
	}
	log.Info("server exited")
}
