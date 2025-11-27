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

	// Public routes
	r.Route("/api", func(r chi.Router) {
		// Auth routes (no middleware)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/signup", config.UserHandler.HandleSignup)
			r.Post("/signin", config.UserHandler.HandleSignin)
			r.Post("/refresh", config.UserHandler.HandleRefreshToken)
		})

		// Protected routes
		r.Route("/user", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))

			config.UserHandler.RegisterRoutes(r)
		})
	})

	return r
}
