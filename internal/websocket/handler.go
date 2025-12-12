package websocket

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/room"
	"github.com/rx3lixir/laba_zis/pkg/httputil"
)

type Handler struct {
	connManager *ConnectionManager
	authService *auth.Service
	roomStore   room.Store
	dbTimeout   time.Duration
	log         *slog.Logger
}

func NewHandler(
	connManager *ConnectionManager,
	authService *auth.Service,
	roomStore room.Store,
	dbTimeout time.Duration,
	log *slog.Logger,
) *Handler {
	return &Handler{connManager, authService, roomStore, dbTimeout, log}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", httputil.Handler(h.HandleConnection, h.log))
}

func (h *Handler) dbCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.dbTimeout)
}

func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) error {
	roomID, err := httputil.ParseUUID(r, "room_id")
	if err != nil {
		return err
	}

	token := r.Header.Get("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	if token == "" || token == r.Header.Get("Authorization") {
		token = r.URL.Query().Get("token")
	}

	if token == "" {
		return httputil.Unauthorized("Missing authorization token")
	}

	claims, err := h.authService.ValidateAccessToken(token)
	if err != nil {
		return httputil.Unauthorized("Invalid or expired token")
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	isInRoom, err := h.roomStore.IsUserInRoom(ctx, roomID, claims.UserID)
	if err != nil || !isInRoom {
		return httputil.Forbidden("You are not a member of this room")
	}

	// Upgrade connection
	if err := h.connManager.HandleConnection(w, r, claims.UserID, roomID); err != nil {
		h.log.Error("webSocket upgrade failed", "error", err)
		return httputil.Internal(err)
	}

	h.log.Info("establishing websocket connection",
		"user_id", claims.UserID,
		"room_id", roomID,
		"username", claims.Username)

	return nil
}
