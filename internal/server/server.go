package server

import (
	"context"
	"net/http"
	"time"

	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type Server struct {
	httpServer *http.Server
	log        logger.Logger
}

func New(addr string, handler http.Handler, log logger.Logger) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      handler,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		log: log,
	}
}

func (s *Server) Start() error {
	s.log.Info("starting http server", "addr", s.httpServer.Addr)
	// return s.httpServer.ListenAndServeTLS("internal/server/cert.pem", "internal/server/key.pem")
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("shutting down server")
	return s.httpServer.Shutdown(ctx)
}
