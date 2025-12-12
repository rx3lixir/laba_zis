package room

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/httputil"
)

type Handler struct {
	store     Store
	log       *slog.Logger
	dbTimeout time.Duration
}

func NewHandler(store Store, log *slog.Logger, dbTimeout time.Duration) *Handler {
	if dbTimeout == 0 {
		dbTimeout = time.Second * 5
	}
	return &Handler{store, log, dbTimeout}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", httputil.Handler(h.HandleCreateRoom, h.log))
	r.Get("/", httputil.Handler(h.HandleGetUserRooms, h.log))
	r.Get("/{roomID}", httputil.Handler(h.HandleGetRoom, h.log))
	r.Delete("/{roomID}", httputil.Handler(h.HandleDeleteRoom, h.log))
	r.Post("/{roomID}/participants", httputil.Handler(h.HandleAddParticipant, h.log))
	r.Delete("/{roomID}/participants/{userID}", httputil.Handler(h.HandleRemoveParticipant, h.log))
	r.Get("/{roomID}/participants", httputil.Handler(h.HandleGetParticipants, h.log))
}

func (h *Handler) dbCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.dbTimeout)
}

// HandleCreateRoom creates a new room with initial participants
func (h *Handler) HandleCreateRoom(w http.ResponseWriter, r *http.Request) error {
	creatorID := auth.GetUserID(r.Context())
	if creatorID == uuid.Nil {
		h.log.Debug("room creation attempt without authentication")
		return httputil.Unauthorized("Unauthorized")
	}

	req := new(CreateRoomRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("room creation request received",
		"creator_id", creatorID,
		"participant_count", len(req.ParticipantIDs))

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	room := &Room{}

	if err := h.store.CreateRoom(ctx, room); err != nil {
		h.log.Error("failed to create room in database",
			"creator_id", creatorID,
			"error", err)
		return httputil.Internal(err)
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
			h.log.Error("failed to add participant during room creation",
				"room_id", room.ID,
				"participant_id", p.UserID,
				"creator_id", creatorID,
				"error", err)
			return httputil.Internal(err)
		}
		addedParticipants = append(addedParticipants, *p)
	}

	h.log.Info("room created successfully",
		"room_id", room.ID,
		"creator_id", creatorID,
		"participant_count", len(participants))

	response := CreateRoomResponse{
		Room:         *room,
		Participants: addedParticipants,
	}

	return httputil.RespondJSON(w, http.StatusCreated, response)
}

// HandleGetRoom gets room details with participants
func (h *Handler) HandleGetRoom(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	roomID, err := httputil.ParseUUID(r, "roomID")
	if err != nil {
		return err
	}

	h.log.Debug("get room request",
		"user_id", userID,
		"room_id", roomID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	if !isInRoom {
		h.log.Warn("get room blocked - user not in room",
			"user_id", userID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	room, err := h.store.GetRoomByID(ctx, roomID)
	if err != nil {
		h.log.Error("failed to retrieve room from database",
			"room_id", roomID,
			"error", err)
		return httputil.NotFound("Room not found")
	}

	participants, err := h.store.GetRoomParticipants(ctx, roomID)
	if err != nil {
		h.log.Error("failed to retrieve room participants",
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	participantsList := make([]RoomParticipant, len(participants))
	for i, p := range participants {
		participantsList[i] = *p
	}

	h.log.Debug("room retrieved",
		"room_id", roomID,
		"participant_count", len(participants))

	response := RoomResponse{
		Room:         *room,
		Participants: participantsList,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleGetUserRooms gets all rooms the authenticated user is part of
func (h *Handler) HandleGetUserRooms(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())

	h.log.Debug("get user rooms request",
		"user_id", userID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	rooms, err := h.store.GetUserRooms(ctx, userID)
	if err != nil {
		h.log.Error("failed to get user rooms from database",
			"user_id", userID,
			"error", err)
		return httputil.Internal(err)
	}

	// TODO: N+1 query problem – replace with batch loading when scaling
	// Consider adding GetRoomsWithParticipants(ctx, userID)

	roomResponses := make([]RoomResponse, 0, len(rooms))

	// Get participants for each room
	for _, room := range rooms {
		participants, err := h.store.GetRoomParticipants(ctx, room.ID)
		if err != nil {
			h.log.Warn("failed to load participants for room",
				"room_id", room.ID,
				"user_id", userID,
				"error", err)
			participants = nil
		}

		plist := make([]RoomParticipant, len(participants))
		for i, p := range participants {
			plist[i] = *p
		}

		roomResponses = append(roomResponses, RoomResponse{
			Room:         *room,
			Participants: plist,
		})
	}

	h.log.Debug("user rooms retrieved",
		"user_id", userID,
		"room_count", len(roomResponses))

	response := GetUserRoomsResponse{
		Rooms: roomResponses,
		Count: len(roomResponses),
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleDeleteRoom deletes a room (only if user is a participant)
func (h *Handler) HandleDeleteRoom(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	roomID, err := httputil.ParseUUID(r, "roomID")
	if err != nil {
		return err
	}

	h.log.Debug("delete room request",
		"user_id", userID,
		"room_id", roomID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Check if user is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	if !isInRoom {
		h.log.Warn("delete room blocked - user not in room",
			"user_id", userID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	if err := h.store.DeleteRoom(ctx, roomID); err != nil {
		h.log.Error("failed to delete room from database",
			"room_id", roomID,
			"user_id", userID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("room deleted successfully",
		"room_id", roomID,
		"deleted_by", userID)

	return httputil.RespondJSON(w, http.StatusNoContent, map[string]string{"message": "Room deleted successfully"})
}

// HandleAddParticipant adds a user to the room
func (h *Handler) HandleAddParticipant(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	roomID, err := httputil.ParseUUID(r, "roomID")
	if err != nil {
		return err
	}

	req := new(AddParticipantRequest)
	if err := httputil.DecodeJSON(r, req); err != nil {
		return err
	}

	h.log.Debug("add participant request",
		"requester_id", userID,
		"room_id", roomID,
		"participant_id", req.UserID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Check if requester is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("add participant blocked - requester not in room",
			"requester_id", userID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	participant := &RoomParticipant{
		RoomID: roomID,
		UserID: req.UserID,
	}

	if err := h.store.AddParticipant(ctx, participant); err != nil {
		h.log.Error("failed to add participant to room",
			"room_id", roomID,
			"participant_id", req.UserID,
			"added_by", userID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("participant added successfully",
		"room_id", roomID,
		"participant_id", req.UserID,
		"added_by", userID)

	return httputil.RespondJSON(w, http.StatusOK, participant)
}

// HandleRemoveParticipant removes a user from the room
func (h *Handler) HandleRemoveParticipant(w http.ResponseWriter, r *http.Request) error {
	requestingUserID := auth.GetUserID(r.Context())
	roomID, err := httputil.ParseUUID(r, "roomID")
	if err != nil {
		return err
	}

	userIDToRemove, err := uuid.Parse(chi.URLParam(r, "userID"))
	if err != nil {
		return httputil.BadRequest("Invalid user ID")
	}

	h.log.Debug("remove participant request",
		"requester_id", requestingUserID,
		"room_id", roomID,
		"participant_id", userIDToRemove)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Check if requester is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, requestingUserID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", requestingUserID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("remove participant blocked - requester not in room",
			"requester_id", requestingUserID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	// Users can only remove themselves (add admin logic later)
	if userIDToRemove != requestingUserID {
		h.log.Warn("remove participant blocked - can only remove self",
			"requester_id", requestingUserID,
			"target_id", userIDToRemove,
			"room_id", roomID)
		return httputil.Forbidden("You can only remove yourself from room")
	}

	if err := h.store.RemoveParticipant(ctx, roomID, userIDToRemove); err != nil {
		h.log.Error("failed to remove participant from room",
			"room_id", roomID,
			"participant_id", userIDToRemove,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info("participant removed successfully",
		"room_id", roomID,
		"participant_id", userIDToRemove)

	return httputil.RespondJSON(w, http.StatusNoContent, map[string]string{
		"message": "Participant removed successfully",
	})
}

// HandleGetParticipants gets all participants in a room
func (h *Handler) HandleGetParticipants(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	roomID, err := httputil.ParseUUID(r, "roomID")
	if err != nil {
		return err
	}

	h.log.Debug("get participants request",
		"user_id", userID,
		"room_id", roomID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Check if user is in the room
	isInRoom, err := h.store.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("get participants blocked - user not in room",
			"user_id", userID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	participants, err := h.store.GetRoomParticipants(ctx, roomID)
	if err != nil {
		h.log.Error("failed to retrieve room participants",
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	// Convert to response format
	participantsList := make([]RoomParticipant, len(participants))
	for i, p := range participants {
		participantsList[i] = *p
	}

	h.log.Debug("participants retrieved",
		"room_id", roomID,
		"participant_count", len(participantsList))

	response := map[string]any{
		"participants": participantsList,
		"count":        len(participantsList),
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}
