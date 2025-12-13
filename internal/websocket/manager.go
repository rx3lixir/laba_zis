package websocket

import (
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // tighten in prod!
	},
}

type ConnectionManager struct {
	hubs sync.Map // map[uuid.UUID]*Hub
	log  *slog.Logger
}

func NewConnectionManager(log *slog.Logger) *ConnectionManager {
	return &ConnectionManager{log: log}
}

// GetOrCreateHub returns existing hub or creates new one
func (cm *ConnectionManager) GetOrCreateHub(roomID uuid.UUID) *Hub {
	if hub, ok := cm.hubs.Load(roomID); ok {
		return hub.(*Hub)
	}

	hub := NewHub(roomID, cm.log)
	actual, loaded := cm.hubs.LoadOrStore(roomID, hub)

	if !loaded {
		// We created a new hub, start it
		go hub.Run()
		cm.log.Info("Created new hub", "room_id", roomID)
	}

	return actual.(*Hub)
}

// BroadcastToRoom sends message to all clients in a room
func (cm *ConnectionManager) BroadcastToRoom(roomID uuid.UUID, message ServerMessage) {
	if hub, ok := cm.hubs.Load(roomID); ok {
		hub.(*Hub).Send(message)
	} else {
		cm.log.Warn("attempted to broadcast to non-existent room", "room_id", roomID)
	}
}

// HandleConnection upgrades HTTP to WebSocket
func (cm *ConnectionManager) HandleConnection(
	w http.ResponseWriter,
	r *http.Request,
	userID uuid.UUID,
	roomID uuid.UUID,
) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return err
	}

	hub := cm.GetOrCreateHub(roomID)
	client := NewClient(hub, conn, userID, cm.log)

	// Register with hub
	hub.register <- client

	// Start client pumps
	go client.writePump()
	go client.readPump()

	return nil
}

// Shutdown gracefully shuts down all hubs
func (cm *ConnectionManager) Shutdown() {
	cm.log.Info("shutting down all websocket hubs")
	cm.hubs.Range(func(key, value any) bool {
		hub := value.(*Hub)
		roomID := key.(uuid.UUID)

		cm.log.Debug("shutting down hub", "room_id", roomID)
		hub.Shutdown()

		return true
	})

	cm.log.Info("all websocket hubs shut down")
}

// GetMetrics returns metrics for monitoring (now thread-safe)
func (cm *ConnectionManager) GetMetrics() map[uuid.UUID]HubMetrics {
	metrics := make(map[uuid.UUID]HubMetrics)

	cm.hubs.Range(func(key, value any) bool {
		roomID := key.(uuid.UUID)
		hub := value.(*Hub)

		// Use the thread-safe snapshot method
		metrics[roomID] = hub.GetMetricsSnapshot()

		return true
	})

	return metrics
}

// GetHubCount returns the number of active hubs
func (cm *ConnectionManager) GetHubCount() int {
	count := 0
	cm.hubs.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// CleanupIdleHubs removes hubs with no clients (call periodically)
func (cm *ConnectionManager) CleanupIdleHubs() int {
	removed := 0

	cm.hubs.Range(func(key, value any) bool {
		hub := value.(*Hub)
		roomID := key.(uuid.UUID)

		metrics := hub.GetMetricsSnapshot()

		// If no clients, shut down and remove
		if metrics.ConnectedClients == 0 {
			cm.log.Debug("cleaning up idle hub", "room_id", roomID)
			hub.Shutdown()
			cm.hubs.Delete(roomID)
			removed++
		}

		return true
	})

	if removed > 0 {
		cm.log.Info("cleaned up idle hubs", "count", removed)
	}

	return removed
}
