package websocket

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type Handler struct {
	manager Manager
	log     logger.Logger
}

func NewHandler(wsManager Manager, log logger.Logger) *Handler {
	return &Handler{
		manager: wsManager,
		log:     log,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.HandleConnection)
}

func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	h.manager.ServeWS(w, r)
}
