package server

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/room"
	"github.com/rx3lixir/laba_zis/internal/user"
	"github.com/rx3lixir/laba_zis/internal/voice"
	"github.com/rx3lixir/laba_zis/internal/websocket"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type RouterConfig struct {
	UserHandler      *user.Handler
	RoomHandler      *room.Handler
	VoiceHandler     *voice.Handler
	WebSocketHandler *websocket.Handler
	Log              logger.Logger
	AuthService      *auth.Service
}

func NewRouter(config RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Middleware block
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*"}, // SvelteKit dev/preview ports
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		// Public auth routes
		r.Route("/auth", func(r chi.Router) {
			config.UserHandler.RegisterAuthRoutes(r)
		})

		// Chat rooms logic routes
		r.Route("/rooms", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))
			config.RoomHandler.RegisterRoutes(r)
		})

		// Voice messages logic routes
		r.Route("/messages", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))
			config.VoiceHandler.RegisterRoutes(r)
		})

		// User logic routes
		r.Route("/user", func(r chi.Router) {
			r.Use(auth.Middleware(config.AuthService))
			config.UserHandler.RegisterUserRoutes(r)
		})

		// WebSocket routes - NEW
		r.Route("/ws", func(r chi.Router) {
			// Note: WebSocket handles auth via token query param, not middleware
			config.WebSocketHandler.RegisterRoutes(r)
		})
	})

	return r
}
