package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type RouterConfig struct {
	UserHandler *user.Handler
	Log         logger.Logger
	AuthService *auth.Service
}

func NewRouter(config RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Middleware block
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Routes
	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			config.UserHandler.RegisterUserRoutes(r)
		})
		r.Route("/user", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))
			config.UserHandler.RegisterUserRoutes(r)
		})
	})

	return r
}
