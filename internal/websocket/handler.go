package websocket

import (
	"context"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

// RoomStore is a minimal interface for room verification
type RoomStore interface {
	IsUserInRoom(ctx context.Context, roomID, userID uuid.UUID) (bool, error)
}

// UserStore is a minimal interface for getting user info
type UserStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*UserInfo, error)
}

// UserInfo holds minimal user information needed for WebSocket
type UserInfo struct {
	ID       uuid.UUID
	Username string
}

// Handler handles WebSocket connections
type Handler struct {
	hub         *Hub
	authService *auth.Service
	roomStore   RoomStore
	userStore   UserStore
	log         logger.Logger
}

// NewHandler creates a new WebSocket handler
func NewHandler(
	hub *Hub,
	authService *auth.Service,
	roomStore RoomStore,
	userStore UserStore,
	log logger.Logger,
) *Handler {
	return &Handler{
		hub:         hub,
		authService: authService,
		roomStore:   roomStore,
		userStore:   userStore,
		log:         log,
	}
}

// RegisterRoutes registers WebSocket routes
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/room/{roomID}", h.HandleConnection)
}

// HandleConnection upgrades HTTP to WebSocket and manages the connection
func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Get room ID from URL
	roomIDStr := chi.URLParam(r, "roomID")
	if roomIDStr == "" {
		h.log.Warn("WebSocket connection attempt without room ID")
		http.Error(w, "Room ID required", http.StatusBadRequest)
		return
	}

	roomID, err := uuid.Parse(roomIDStr)
	if err != nil {
		h.log.Warn("Invalid room ID format", "room_id", roomIDStr)
		http.Error(w, "Invalid room ID", http.StatusBadRequest)
		return
	}

	// Get JWT token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		h.log.Warn(
			"WebSocket connection attempt without token",
			"room_id", roomID,
		)
		http.Error(w, "Authentication token required", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.authService.ValidateAccessToken(token)
	if err != nil {
		h.log.Warn(
			"Invalid WebSocket authentication token",
			"room_id", roomID,
			"error", err,
		)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID
	username := claims.Username

	// Verify user is a member of the room
	isInRoomCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	isInRoom, err := h.roomStore.IsUserInRoom(isInRoomCtx, roomID, userID)
	if err != nil {
		h.log.Error(
			"Failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err,
		)
		http.Error(w, "Failed to verify room membership", http.StatusInternalServerError)
		return
	}

	if !isInRoom {
		h.log.Warn("User attempted to connect to room they're not in",
			"user_id", userID,
			"room_id", roomID,
		)
		http.Error(w, "You are not a member of this room", http.StatusForbidden)
		return
	}

	// Upgrade connection to WebSocket
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"}, // For MVP; restrict in production
	})
	if err != nil {
		h.log.Error("Failed to upgrade WebSocket connection",
			"user_id", userID,
			"room_id", roomID,
			"error", err,
		)
		return
	}

	h.log.Info("WebSocket connection established",
		"user_id", userID,
		"username", username,
		"room_id", roomID,
	)

	// Create client
	client := NewClient(userID, username, roomID, conn, h.hub, h.log)

	// Register client with hub
	h.hub.register <- client

	// Send connection confirmation
	connectedMsg := NewConnected(roomID, userID)
	client.send <- connectedMsg

	hubCtx := h.hub.ctx

	// Start pumps
	go client.writePump(hubCtx)
	go client.readPump(hubCtx)
}
