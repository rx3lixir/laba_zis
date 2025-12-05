package room

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type Handler struct {
	store Store
	log   logger.Logger
}

func NewHandler(store Store, log logger.Logger) *Handler {
	return &Handler{
		store: store,
		log:   log,
	}
}

func writeJson(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, status int, error string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": error})
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleCreateRoom)
	r.Get("/", h.HandleGetUserRooms)
	r.Get("/{roomID}", h.HandleGetRoom)
	r.Delete("/{roomID}", h.HandleDeleteRoom)
	r.Post("/{roomID}/participants", h.HandleAddParticipant)
	r.Delete("/{roomID}/participants/{userID}", h.HandleRemoveParticipant)
	r.Get("/{roomID}/participants", h.HandleGetParticipants)
}

// HandleCreateRoom creates a new room with initial participants
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	creatorID := auth.GetUserID(r.Context())
	if creatorID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	req := new(CreateRoomRequest)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusInternalServerError, "Invalid JSON")
		return
	}

	h.log.Debug(
		"Creating room",
		"creator_id", creatorID,
		"participant_count", len(req.ParticipantIDs),
	)

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	room := &Room{}

	if err := h.store.CreateRoom(ctx, room); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create room")
		h.log.Error("Failed to create room", "error", err)
		return
	}

	// Add creator as participant
	participants := []*RoomParticipant{
		{RoomID: room.ID, UserID: creatorID},
	}

	// Add other participants
	for _, userID := range req.ParticipantIDs {
		if userID != creatorID {
			participants = append(participants, &RoomParticipant{
				RoomID: room.ID,
				UserID: userID,
			})
		}
	}

	addedParticipants := []RoomParticipant{}

	// Adding all participants into database
	for _, p := range participants {
		if err := h.store.AddParticipant(ctx, p); err != nil {
			h.log.Warn("Failed to add participant", "user_id", p.UserID, "error", err)
			continue
		}
		addedParticipants = append(addedParticipants, *p)
	}

	response := CreateRoomResponse{
		Room:         *room,
		Participants: addedParticipants,
	}

	h.log.Debug("Room created",
		"room_id", room.ID,
		"participants", len(addedParticipants),
	)

	writeJson(w, http.StatusCreated, response)
}

// HandleGetRoom gets room details with participants
func (h *Handler) HandleGetRoom(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Room ID is invalid")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to verify access")
		return
	}

	if !isInRoom {
		writeError(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	room, err := h.store.GetRoomByID(ctx, roomID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Room not found")
		return
	}

	participants, err := h.store.GetRoomParticipants(ctx, roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get participants")
		return
	}

	participantsList := make([]RoomParticipant, len(participants))
	for i, p := range participants {
		participantsList[i] = *p
	}

	response := RoomResponse{
		Room:         *room,
		Participants: participantsList,
	}

	writeJson(w, http.StatusOK, response)
}

// HandleGetUserRooms gets all rooms the authenticated user is part of
func (h *Handler) HandleGetUserRooms(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	rooms, err := h.store.GetUserRooms(ctx, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get rooms")
		h.log.Error("Failed to get user rooms", "user_id", userID, "error", err)
		return
	}

	roomResponses := make([]RoomResponse, 0, len(rooms))

	// Get participants for each room
	for _, room := range rooms {
		participants, err := h.store.GetRoomParticipants(ctx, room.ID)
		if err != nil {
			h.log.Warn("Failed to get participants for room", "room_id", room.ID, "error", err)
			continue
		}

		participantsList := make([]RoomParticipant, len(participants))
		for i, p := range participants {
			participantsList[i] = *p
		}

		roomResponses = append(roomResponses, RoomResponse{
			Room:         *room,
			Participants: participantsList,
		})
	}

	response := GetUserRoomsResponse{
		Rooms: roomResponses,
		Count: len(roomResponses),
	}

	h.log.Debug("Retrieved user rooms", "user_id", userID, "count", len(roomResponses))

	writeJson(w, http.StatusOK, response)
}

// HandleDeleteRoom deletes a room (only if user is a participant)
func (h *Handler) HandleDeleteRoom(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	// Check if user is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		writeError(w, http.StatusForbidden, "Failed to verify access")
		return
	}

	if !isInRoom {
		writeError(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	if err := h.store.DeleteRoom(ctx, roomID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete room")
		return
	}

	h.log.Debug("Room deleted", "room_id", roomID, "by_user", userID)

	writeJson(w, http.StatusOK, "Room deleted successfully")
}

// HandleAddParticipant adds a user to the room
func (h *Handler) HandleAddParticipant(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	req := new(AddParticipantRequest)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	// Check if requester is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil || !isInRoom {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "You are not a member of this room"})
		return
	}

	participant := &RoomParticipant{
		RoomID: roomID,
		UserID: req.UserID,
	}

	if err := h.store.AddParticipant(ctx, participant); err != nil {
		writeJson(w, http.StatusInternalServerError, "Failed to add participant")
		return
	}

	h.log.Debug("Participant added", "room_id", roomID, "user_id", req.UserID)

	writeJson(w, http.StatusOK, participant)
}

// HandleRemoveParticipant removes a user from the room
func (h *Handler) HandleRemoveParticipant(w http.ResponseWriter, r *http.Request) {
	requestingUserID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}

	userIDToRemove, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	// Check if requester is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, requestingUserID)
	if err != nil || !isInRoom {
		writeError(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	// Users can only remove themselves (for now - you could add admin logic later)
	if userIDToRemove != requestingUserID {
		writeError(w, http.StatusForbidden, "You can only remove yourself from rooms")
		return
	}

	if err := h.store.RemoveParticipant(ctx, roomID, userIDToRemove); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to remove participant")
		return
	}

	h.log.Debug("Participant removed", "room_id", roomID, "user_id", userIDToRemove)

	writeJson(w, http.StatusOK, "Participant removed successfully")
}

// HandleGetParticipants gets all participants in a room
func (h *Handler) HandleGetParticipants(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	// Check if user is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil || !isInRoom {
		writeError(w, http.StatusForbidden, "Your are not a member of this room")
		return
	}

	participants, err := h.store.GetRoomParticipants(ctx, roomID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get participants")
		return
	}

	// Convert to response format
	participantsList := make([]RoomParticipant, len(participants))
	for i, p := range participants {
		participantsList[i] = *p
	}

	writeJson(w, http.StatusOK, map[string]any{
		"participants": participantsList,
		"count":        len(participantsList),
	})
}
