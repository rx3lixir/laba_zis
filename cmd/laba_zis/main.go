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
	"github.com/rx3lixir/laba_zis/internal/room"
	"github.com/rx3lixir/laba_zis/internal/server"
	"github.com/rx3lixir/laba_zis/internal/storage/postgres"
	"github.com/rx3lixir/laba_zis/internal/storage/s3"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/internal/voice"
	"github.com/rx3lixir/laba_zis/internal/websocket"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

func main() {
	// Creating and validating config
	cm, err := config.NewConfigManager("internal/config/config.yaml")
	if err != nil {
		fmt.Printf("error getting config file: %v", err)
		os.Exit(1)
	}

	c := cm.GetConfig()

	if err := c.Validate(); err != nil {
		fmt.Printf("invalid configuration: %v", err)
		os.Exit(1)
	}

	// Logger initializaion
	log := logger.New(logger.Config{
		Env:    c.GeneralParams.Env,
		Output: os.Stdout,
	})

	log.Info(
		"configuration loaded",
		"env", c.GeneralParams.Env,
		"http_server_address", c.HttpServerParams.GetAddress(),
		"database", c.MainDBParams.Name,
	)

	// Initializing Postgres connections pool
	pool, err := postgres.NewPool(context.Background(), c.MainDBParams.GetDSN())
	if err != nil {
		log.Error(
			"failed to create postgres pool",
			"error", err,
			"db", c.MainDBParams.Name,
		)
		os.Exit(1)
	}
	defer pool.Close()

	log.Info(
		"database connection established",
		"db", c.MainDBParams.GetDSN(),
	)

	// Creating S3 storage
	minioClient, err := s3.NewClient(
		c.S3Params.Endpoint,
		c.S3Params.AccessKeyID,
		c.S3Params.SecretAccessKey,
		c.S3Params.UseSSL,
	)
	if err != nil {
		log.Error("failed to create MinIO client", "error", err)
		os.Exit(1)
	}

	// Making sure it has a bucket that we need
	if err := s3.EnsureBucket(context.Background(), minioClient, c.S3Params.BucketName); err != nil {
		log.Error("failed to ensure bucket exists", "error", err, "bucket", c.S3Params.BucketName)
		os.Exit(1)
	}

	log.Info("minIO client initialized", "bucket", c.S3Params.BucketName)

	// Create stores
	userStore := user.NewPostgresStore(pool)
	roomStore := room.NewPostgresStore(pool)
	voiceMessageDBStore := voice.NewPostgresStore(pool)
	voiceMessageFileStore := voice.NewMinIOVoiceStore(minioClient, c.S3Params.BucketName)

	// Create auth service
	authService := auth.NewService(
		c.GeneralParams.SecretKey,
		time.Duration(c.GeneralParams.AccessTokenTTL)*time.Minute,
		time.Duration(c.GeneralParams.RefreshTokenTTL)*24*time.Hour,
	)

	// Creating websocket manager
	wsManager := websocket.NewConnectionManager(log)

	// Converting database timeout from config to actual time
	dbTimeout := time.Duration(c.MainDBParams.Timeout) * time.Second

	// Create Handlers
	roomHandler := room.NewHandler(roomStore, log, dbTimeout)
	userHandler := user.NewHandler(userStore, authService, log, dbTimeout)
	wsHandler := websocket.NewHandler(wsManager, authService, roomStore, dbTimeout, log)
	voiceHandler := voice.NewHandler(
		voiceMessageDBStore,
		voiceMessageFileStore,
		roomStore,
		wsManager,
		log,
		dbTimeout,
	)

	// Setup router
	router := server.NewRouter(server.RouterConfig{
		UserHandler:  userHandler,
		RoomHandler:  roomHandler,
		VoiceHandler: voiceHandler,
		AuthService:  authService,
		WsHandler:    wsHandler,
		Log:          log,
	})

	// Create server with all passed parameters
	srv := server.New(c.HttpServerParams.GetAddress(), router, log)

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

		// Start graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Shutdown websocket connections first
		log.Info("shutting down websocket conections...")
		wsManager.Shutdown()
		log.Info("websocket connections closed")

		// Shutdown HTTP server
		log.Info("shutting down http server...")

		if err := srv.Shutdown(ctx); err != nil {
			log.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}

		log.Info("server stopped gracefully")
	}
}
