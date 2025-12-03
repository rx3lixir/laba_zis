package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

const (
	maxUploadSize = 5 * 1024 * 1024 // 5MB max file size
	maxDuration   = 15              // 15 seconds max
	urlExpiryTime = 1 * time.Hour   // Presigned URLs expire after 1 hour
	defaultLimit  = 50
	defaultOffset = 0
)

type Handler struct {
	dbStore   VoiceMessageDBStore
	fileStore VoiceMessageStore
	roomStore RoomStore // We need to check if user is in room
	wsHub     WebSocketHub
	log       logger.Logger
}

// RoomStore is a minimal interface for room verification
type RoomStore interface {
	IsUserInRoom(ctx context.Context, roomID, userID uuid.UUID) (bool, error)
}

// WebSocketHub is the interface for broadcasting messages
type WebSocketHub interface {
	BroadcastToRoom(roomID uuid.UUID, message any)
}

func NewHandler(
	dbStore VoiceMessageDBStore,
	fileStore VoiceMessageStore,
	roomStore RoomStore,
	wsHub WebSocketHub,
	log logger.Logger,
) *Handler {
	return &Handler{
		dbStore:   dbStore,
		fileStore: fileStore,
		roomStore: roomStore,
		wsHub:     wsHub,
		log:       log,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.HandleUploadVoiceMessage)
	r.Get("/room/{roomID}", h.HandleGetRoomMessages)
	r.Get("/{messageID}", h.HandleGetVoiceMessage)
	r.Delete("/{messageID}", h.HandleDeleteVoiceMessage)
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

// HandleUploadVoiceMessage uploads a voice message to S3 and creates a DB record
func (h *Handler) HandleUploadVoiceMessage(w http.ResponseWriter, r *http.Request) {
	senderID := auth.GetUserID(r.Context())
	if senderID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse multipart form with size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusUnauthorized, "File too large or data is invalid")
		return
	}

	// Get room_id and duration from form
	roomIDStr := r.FormValue("room_id")
	durationStr := r.FormValue("duration_seconds")

	if roomIDStr == "" || durationStr == "" {
		writeError(w, http.StatusBadRequest, "room_id and duration_seconds required")
		return
	}

	roomID, err := uuid.Parse(roomIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid room_id format")
		return
	}

	duration, err := strconv.Atoi(durationStr)
	if err != nil || duration <= 0 || duration > maxDuration {
		writeError(w, http.StatusBadRequest, "duration_seconds must be between 1 and 15")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
	defer cancel()

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, roomID, senderID)
	if err != nil || !isInRoom {
		writeJson(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	// Get the audio file from form
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeJson(w, http.StatusBadRequest, "Audio file is required")
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		writeJson(w, http.StatusInternalServerError, "Failed to read audio file")
		h.log.Error("Failed to read uploaded file", "error", err)
		return
	}

	if len(data) == 0 {
		writeJson(w, http.StatusBadRequest, "Empty audio file")
		return
	}

	// Determine audio format from content type or filename
	audioFormat := "webm" // default

	h.log.Debug(
		"Uploading voice message",
		"sender_id", senderID,
		"room_id", roomID,
		"duration", duration,
		"size_bytes", len(data),
		"format", audioFormat,
	)

	// Create message record (to get ID for S3 key)
	message := &VoiceMessage{
		RoomID:          roomID,
		SenderID:        senderID,
		DurationSeconds: duration,
	}
	// Generate ID for S3 upload
	message.ID = uuid.New()

	// Upload to S3
	s3Key, err := h.fileStore.UploadVoiceMessage(ctx, message.ID, data, audioFormat)
	if err != nil {
		writeJson(w, http.StatusInternalServerError, "Failed to upload audio file")
		h.log.Error("Failed to upload to S3", "error", err)
		return
	}

	message.S3Key = s3Key

	// Save to database
	if err := h.dbStore.CreateVoiceMessage(ctx, message); err != nil {
		// Try to clean up S3 file if DB insert fails
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cleanupCancel()
		_ = h.fileStore.DeleteVoiceMessage(cleanupCtx, s3Key)

		writeJson(w, http.StatusInternalServerError, "Failed to save message metadata")

		h.log.Error("Failed to create voice message in DB", "error", err)

		return
	}

	// Generate presigned URL for immediate playback
	url, err := h.fileStore.GetPresignedURL(ctx, s3Key, urlExpiryTime)
	if err != nil {
		h.log.Warn("Failed to generate presigned URL", "error", err)
		url = "" // Continue without URL
	}

	response := UploadVoiceMessageResponse{
		Message: *message,
		URL:     url,
	}

	h.log.Debug(
		"Voice message uploaded",
		"message_id", message.ID,
		"room_id", roomID,
		"s3_key", s3Key,
	)

	if h.wsHub != nil {
		wsMessage := map[string]any{
			"type": "voice_message",
			"data": map[string]any{
				"id":               message.ID,
				"room_id":          message.RoomID,
				"sender_id":        message.SenderID,
				"url":              url,
				"duration_seconds": message.DurationSeconds,
				"created_at":       message.CreatedAt,
			},
		}

		h.wsHub.BroadcastToRoom(roomID, wsMessage)

		h.log.Debug(
			"Broadcasted voice message to WebSocket clients",
			"room_id", roomID,
			"message_id", message.ID,
		)
	}

	writeJson(w, http.StatusCreated, response)
}

// HandleGetRoomMessages retrieves all voice messages in a room
func (h *Handler) HandleGetRoomMessages(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid room ID")
		return
	}

	// Parse pagination params
	limit := defaultLimit
	offset := defaultOffset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 100 {
				limit = 100 // Cap at 100
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, roomID, userID)
	if err != nil || !isInRoom {
		writeError(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	messages, err := h.dbStore.GetRoomMessages(ctx, roomID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve messages")
		h.log.Error("Failed to get room messages", "room_id", roomID, "error", err)
		return
	}

	// Generate presigned URLs for each message
	messagesWithURLs := make([]VoiceMessageWithURL, 0, len(messages))
	for _, msg := range messages {
		url, err := h.fileStore.GetPresignedURL(ctx, msg.S3Key, urlExpiryTime)
		if err != nil {
			h.log.Warn("Failed to generate presigned URL", "message_id", msg.ID, "error", err)
			url = ""
		}

		messagesWithURLs = append(messagesWithURLs, VoiceMessageWithURL{
			VoiceMessage: *msg,
			URL:          url,
		})
	}

	response := GetRoomMessagesResponse{
		Messages: messagesWithURLs,
		Count:    len(messagesWithURLs),
	}

	h.log.Debug("Retrieved room messages", "room_id", roomID, "count", len(messages))

	writeJson(w, http.StatusOK, response)
}

// HandleGetVoiceMessage retrieves a single voice message
func (h *Handler) HandleGetVoiceMessage(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*3)
	defer cancel()

	message, err := h.dbStore.GetVoiceMessageByID(ctx, messageID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Message not found")
		return
	}

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, message.RoomID, userID)
	if err != nil || !isInRoom {
		writeError(w, http.StatusForbidden, "You are not a member of this room")
		return
	}

	// Generate presigned URL
	url, err := h.fileStore.GetPresignedURL(ctx, message.S3Key, urlExpiryTime)
	if err != nil {
		h.log.Warn("Failed to generate presigned URL", "message_id", messageID, "error", err)
		url = ""
	}

	response := VoiceMessageWithURL{
		VoiceMessage: *message,
		URL:          url,
	}

	writeJson(w, http.StatusOK, response)
}

// HandleDeleteVoiceMessage deletes a voice message (only by sender)
func (h *Handler) HandleDeleteVoiceMessage(w http.ResponseWriter, r *http.Request) {
	userID := auth.GetUserID(r.Context())
	messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid message ID")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), time.Second*5)
	defer cancel()

	// Get the message
	message, err := h.dbStore.GetVoiceMessageByID(ctx, messageID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Message not found")
		return
	}

	// Only sender can delete their own messages
	if message.SenderID != userID {
		writeError(w, http.StatusForbidden, "You can only delete your messages")
		return
	}

	// Delete from S3 first
	if err := h.fileStore.DeleteVoiceMessage(ctx, message.S3Key); err != nil {
		h.log.Error("Failed to delete from S3", "s3_key", message.S3Key, "error", err)
		// Continue to delete from DB anyway
	}

	// Delete from database
	if err := h.dbStore.DeleteVoiceMessage(ctx, messageID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete message"})
		return
	}

	h.log.Debug("Voice message deleted", "message_id", messageID, "by_user", userID)

	writeJson(w, http.StatusOK, "Message deleted successfully")
}
