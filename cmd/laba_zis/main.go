package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rx3lixir/laba_zis/internal/config"
	httpserver "github.com/rx3lixir/laba_zis/internal/http_server"
	maindb "github.com/rx3lixir/laba_zis/internal/storage/main_db"
	"github.com/rx3lixir/laba_zis/pkg/jwt"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

func main() {
	// Initializing and validating config
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

	// Initializing logger
	log, err := logger.New(logger.Config{
		Env:       c.GeneralParams.Env,
		AddSource: false,
	})

	log.Info(
		"Config loaded successfully!",
		"env", c.GeneralParams.Env,
		"http_server_port", c.HttpServerParams.Port,
		"http_server_addresss", c.HttpServerParams.Address,
		"database", c.MainDBParams.Name,
	)

	// Global context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Creating database connection and init Postgres
	pool, err := maindb.CreatePostgresPool(ctx, c.MainDBParams.GetDSN())
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

	maindbStore := maindb.NewPostgresStore(pool)

	// JWT Service intialization
	jwtService := jwt.NewService(
		c.GeneralParams.SecretKey,
		time.Minute*15,
		time.Hour*24*7,
	)

	// Creates HTTP server
	HTTPserver := httpserver.New(
		c.HttpServerParams.GetAddress(),
		maindbStore,
		jwtService,
		log,
	)

	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- HTTPserver.Start()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Block until we recieve a signal or error
	select {
	case err := <-serverErrors:
		log.Error("Server error", "error", err)
		os.Exit(1)

	case sig := <-shutdown:
		log.Info("Shutdown signal received", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Info("Shutting down HTTP server...")
		if err := HTTPserver.Shutdown(ctx); err != nil {
			log.Error("Graceful shutdown failed", "error", err)
		}
	}
}
