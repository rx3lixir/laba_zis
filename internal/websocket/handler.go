package websocket

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type Handler struct {
	manager     *Manager
	authService *auth.Service
	log         logger.Logger
}

func NewHandler(wsManager *Manager, authService *auth.Service, log logger.Logger) *Handler {
	return &Handler{
		manager:     wsManager,
		authService: authService,
		log:         log,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.HandleConnection)
}

func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Get room_id from query params
	roomIDStr := r.URL.Query().Get("room_id")
	if roomIDStr == "" {
		http.Error(w, "room_id parameter required", http.StatusBadRequest)
		return
	}

	roomID, err := uuid.Parse(roomIDStr)
	if err != nil {
		http.Error(w, "Invalid room_id format", http.StatusBadRequest)
		return
	}

	// Try to get token from Authorization header first
	token := r.Header.Get("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	// If not in header, try query param (for browsers that don't support headers in WebSocket)
	if token == "" || token == r.Header.Get("Authorization") {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		http.Error(w, "Missing authorization token", http.StatusUnauthorized)
		return
	}

	claims, err := h.authService.ValidateAccessToken(token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	h.log.Info(
		"Establishing websocket connection",
		"user_id", claims.UserID,
		"room_id", roomID,
		"username", claims.Username,
	)

	// Upgrade and register the connection
	h.manager.ServeWS(w, r, claims.UserID, roomID)
}
