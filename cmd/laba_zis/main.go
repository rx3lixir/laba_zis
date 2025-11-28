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
	"github.com/rx3lixir/laba_zis/internal/storage/s3"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/internal/voice"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

func main() {
	// Creating and validating config
	cm, err := config.NewConfigManager("internal/config/config.yaml")
	if err != nil {
		fmt.Printf("Error getting config file: %v", err)
		os.Exit(1)
	}

	c := cm.GetConfig()

	if err := c.Validate(); err != nil {
		fmt.Printf("Invalid configuration: %v", err)
		os.Exit(1)
	}

	// Logger initializaion
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

	// Context to initialize stores
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)

	// Initializing Postgres connections pool
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

	minioClient, err := s3.NewClient(
		c.S3Params.Endpoint,
		c.S3Params.AccessKeyID,
		c.S3Params.SecretAccessKey,
		c.S3Params.UseSSL,
	)
	if err != nil {
		log.Error("Failed to create MinIO client", "error", err)
		os.Exit(1)
	}

	if err := s3.EnsureBucket(ctx, minioClient, c.S3Params.BucketName); err != nil {
		log.Error("Failed to ensure bucket exists", "error", err, "bucket", c.S3Params.BucketName)
		os.Exit(1)
	}
	cancel()

	log.Info("MinIO client initialized", "bucket", c.S3Params.BucketName)

	// Create stores
	userStore := user.NewPostgresStore(pool)
	voiceMessageStore := voice.NewMinIOVoiceStore(minioClient, c.S3Params.BucketName)

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
