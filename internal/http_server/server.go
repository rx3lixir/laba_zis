package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/rx3lixir/laba_zis/internal/storage/main_db"
	"github.com/rx3lixir/laba_zis/pkg/jwt"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type Server struct {
	userStore  maindb.UserStore
	jwtService *jwt.Service
	log        *logger.Logger
	httpServer *http.Server
}

func New(addr string, userStore maindb.UserStore, jwtService *jwt.Service, logger *logger.Logger) *Server {
	s := &Server{
		userStore:  userStore,
		jwtService: jwtService,
		log:        logger,
	}

	router := s.setupRoutes()

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening fot HTTP requests
func (s *Server) Start() error {
	s.log.Info(
		"Starting HTTP server",
		"addr", s.httpServer.Addr,
	)

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info(
		"Server shutting down gracefully...",
		"addr", s.httpServer.Addr,
	)
	return s.httpServer.Shutdown(ctx)
}
