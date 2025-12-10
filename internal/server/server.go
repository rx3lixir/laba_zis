package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
	log        *slog.Logger
}

func New(addr string, handler http.Handler, log *slog.Logger) *Server {
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
