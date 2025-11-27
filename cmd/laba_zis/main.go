package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/config"
	"github.com/rx3lixir/laba_zis/internal/server"
	"github.com/rx3lixir/laba_zis/internal/storage/postgres"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cm, err := config.NewConfigManager("internal/config/config.yaml")
	if err != nil {
		fmt.Printf("Error getting config file: %d", err)
		os.Exit(1)
	}

	c := cm.GetConfig()

	if err := c.Validate(); err != nil {
		fmt.Printf("Invalid configuration: %d", err)
		os.Exit(1)
	}

	log := logger.Must(logger.New(logger.Config{
		Env:              c.GeneralParams.Env,
		AddSource:        true,
		SourcePathLength: 0,
	}))

	log.Info(
		"Configuration loaded",
		"env", c.GeneralParams.Env,
		"http_server_address", c.HttpServerParams.GetAddress(),
		"database", c.MainDBParams.Name,
	)

	pool, err := postgres.NewPool(ctx, c.MainDBParams.GetDSN())
	if err != nil {
		log.Error(
			"Failed to create postgres pool",
			"error", err,
			"db", c.MainDBParams.Name,
		)
		os.Exit(1)
	}
	defer pool.Close()

	log.Info(
		"Database connection established",
		"db", c.MainDBParams.GetDSN(),
	)

	// Create stores
	userStore := user.NewPostgresStore(pool)

	// Create auth service
	authService := auth.NewService(
		c.GeneralParams.SecretKey,
		15*time.Minute, // access token
		7*24*time.Hour, // refresh token
	)

	// Create Handlers
	userHandler := user.NewHandler(userStore, authService, *log)

	// Setup router
	router := server.NewRouter(server.RouterConfig{
		UserHandler: userHandler,
		AuthService: authService,
		Log:         *log,
	})

	srv := server.New(c.HttpServerParams.GetAddress(), router, *log)

	// Start server
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- srv.Start()
	}()

	// Wait for shutdown signal
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Error("server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		log.Info("shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}

		log.Info("server stopped")
	}
}
