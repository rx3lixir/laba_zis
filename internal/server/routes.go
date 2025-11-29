package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/room"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/internal/voice"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type RouterConfig struct {
	UserHandler  *user.Handler
	RoomHandler  *room.Handler
	VoiceHandler *voice.Handler
	Log          logger.Logger
	AuthService  *auth.Service
}

func NewRouter(config RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Middleware block
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Route("/api", func(r chi.Router) {
		// Public auth routes
		r.Route("/auth", func(r chi.Router) {
			config.UserHandler.RegisterAuthRoutes(r)
		})

		// Chat rooms logic routes
		r.Route("/rooms", func(r chi.Router) {
			config.RoomHandler.RegisterRoutes(r)
		})

		// Voice messages logic routes
		r.Route("/messages", func(r chi.Router) {
			config.VoiceHandler.RegisterRoutes(r)
		})

		// User logic routes
		r.Route("/user", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))
			config.UserHandler.RegisterUserRoutes(r)
		})
	})

	return r
}
