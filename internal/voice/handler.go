package voice

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/internal/auth"
	"github.com/rx3lixir/laba_zis/internal/room"
	"github.com/rx3lixir/laba_zis/internal/websocket"
	"github.com/rx3lixir/laba_zis/pkg/audio"
	"github.com/rx3lixir/laba_zis/pkg/httputil"
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
	roomStore room.Store
	wsManager *websocket.ConnectionManager
	log       *slog.Logger
	dbTimeout time.Duration
}

func NewHandler(
	dbStore VoiceMessageDBStore,
	fileStore VoiceMessageStore,
	roomStore room.Store,
	wsManager *websocket.ConnectionManager,
	log *slog.Logger,
	dbTimeout time.Duration,
) *Handler {
	return &Handler{
		dbStore,
		fileStore,
		roomStore,
		wsManager,
		log,
		dbTimeout,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/", httputil.Handler(h.HandleUploadVoiceMessage, h.log))
	r.Get("/room/{roomID}", httputil.Handler(h.HandleGetRoomMessages, h.log))
	r.Get("/{messageID}", httputil.Handler(h.HandleGetVoiceMessage, h.log))
	r.Delete("/{messageID}", httputil.Handler(h.HandleDeleteVoiceMessage, h.log))
}

func (h *Handler) dbCtx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.dbTimeout)
}

// HandleUploadVoiceMessage uploads a voice message to S3 and creates a DB record
func (h *Handler) HandleUploadVoiceMessage(w http.ResponseWriter, r *http.Request) error {
	// Extract user from context
	senderID := auth.GetUserID(r.Context())
	if senderID == uuid.Nil {
		h.log.Debug("voice message upload attempt without authentication")
		return httputil.Unauthorized("Unauthorized")
	}

	// Parse multipart form
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		h.log.Debug("failed to parse multipart form",
			"sender_id", senderID,
			"error", err)
		return httputil.BadRequest("Invalid multipart form data")
	}

	// Extract and validate parameters
	roomIDStr := r.FormValue("room_id")
	durationStr := r.FormValue("duration_seconds")

	h.log.Debug("voice message upload request received",
		"sender_id", senderID,
		"room_id", roomIDStr,
		"duration", durationStr)

	if roomIDStr == "" || durationStr == "" {
		return httputil.BadRequest("room_id and duration_seconds parameters required")
	}

	roomID, err := uuid.Parse(roomIDStr)
	if err != nil {
		return httputil.BadRequest("Invalid room_id format")
	}

	duration, err := strconv.Atoi(durationStr)
	if err != nil || duration <= 0 || duration > maxDuration {
		return httputil.BadRequest("duration_seconds must be between 1 and 15")
	}

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, roomID, senderID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"sender_id", senderID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("voice message upload blocked - user not in room",
			"sender_id", senderID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	// Get the audio file from form
	fileHandler, fileHeader, err := r.FormFile("audio")
	if err != nil {
		return httputil.BadRequest("Audio file is required")
	}
	defer fileHandler.Close()

	// Read file data
	data, err := io.ReadAll(fileHandler)
	if err != nil {
		h.log.Error("failed to read uploaded audio file",
			"sender_id", senderID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	if len(data) == 0 {
		return httputil.BadRequest("Empty audio file")
	}

	// Detect audio format
	contentType := fileHeader.Header.Get("Content-Type")
	filename := fileHeader.Filename
	audioFormat := audio.DetectAudioFormat(contentType, filename)

	h.log.Debug("audio file parsed",
		"sender_id", senderID,
		"room_id", roomID,
		"size_bytes", len(data),
		"format", audioFormat,
		"filename", filename)

	// Create message record
	message := &VoiceMessage{
		ID:              uuid.New(),
		RoomID:          roomID,
		SenderID:        senderID,
		DurationSeconds: duration,
	}

	// Upload to S3
	s3Key, err := h.fileStore.UploadVoiceMessage(ctx, message.ID, data, audioFormat)
	if err != nil {
		h.log.Error("failed to upload voice message to S3",
			"message_id", message.ID,
			"sender_id", senderID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	message.S3Key = s3Key

	// Save to database
	if err := h.dbStore.CreateVoiceMessage(ctx, message); err != nil {
		h.log.Error("failed to create voice message in database",
			"message_id", message.ID,
			"sender_id", senderID,
			"room_id", roomID,
			"s3_key", s3Key,
			"error", err)

		// Cleanup S3 file
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		if cleanupErr := h.fileStore.DeleteVoiceMessage(cleanupCtx, s3Key); cleanupErr != nil {
			h.log.Error("failed to cleanup S3 after database error",
				"s3_key", s3Key,
				"error", cleanupErr)
		}

		return httputil.Internal(err)
	}

	// Generate presigned URL
	url, err := h.fileStore.GetPresignedURL(ctx, s3Key, urlExpiryTime)
	if err != nil {
		h.log.Warn("failed to generate presigned URL, continuing without it",
			"message_id", message.ID,
			"s3_key", s3Key,
			"error", err)
		url = ""
	}

	// Broadcast websocket event
	event := websocket.ServerMessage{
		Type: websocket.TypeNewVoiceMessage,
		Data: websocket.VoiceMessageData{
			MessageID: message.ID,
			SenderID:  message.SenderID,
			Duration:  message.DurationSeconds,
			URL:       url,
		},
	}
	h.wsManager.BroadcastToRoom(message.RoomID, event)

	h.log.Info("voice message uploaded successfully",
		"message_id", message.ID,
		"sender_id", senderID,
		"room_id", roomID,
		"duration_seconds", duration,
		"size_bytes", len(data))

	response := UploadVoiceMessageResponse{
		Message: *message,
		URL:     url,
	}

	return httputil.RespondJSON(w, http.StatusCreated, response)
}

// HandleGetRoomMessages retrieves all voice messages in a room
func (h *Handler) HandleGetRoomMessages(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	roomID, err := uuid.Parse(chi.URLParam(r, "roomID"))
	if err != nil {
		return httputil.BadRequest("Invalid room ID")
	}

	// Parse pagination params
	limit := defaultLimit
	offset := defaultOffset

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 100 {
				limit = 100
			}
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	h.log.Debug("get room messages request",
		"user_id", userID,
		"room_id", roomID,
		"limit", limit,
		"offset", offset)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, roomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("get room messages blocked - user not in room",
			"user_id", userID,
			"room_id", roomID)
		return httputil.Forbidden("You are not a member of this room")
	}

	messages, err := h.dbStore.GetRoomMessages(ctx, roomID, limit, offset)
	if err != nil {
		h.log.Error("failed to get room messages from database",
			"room_id", roomID,
			"error", err)
		return httputil.Internal(err)
	}

	// Generate presigned URLs for each message
	messagesWithURLs := make([]VoiceMessageWithURL, 0, len(messages))
	for _, msg := range messages {
		url, err := h.fileStore.GetPresignedURL(ctx, msg.S3Key, urlExpiryTime)
		if err != nil {
			h.log.Warn("failed to generate presigned URL for message",
				"message_id", msg.ID,
				"s3_key", msg.S3Key,
				"error", err)
			url = ""
		}

		messagesWithURLs = append(messagesWithURLs, VoiceMessageWithURL{
			VoiceMessage: *msg,
			URL:          url,
		})
	}

	h.log.Debug("room messages retrieved",
		"room_id", roomID,
		"count", len(messages))

	response := GetRoomMessagesResponse{
		Messages: messagesWithURLs,
		Count:    len(messagesWithURLs),
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleGetVoiceMessage retrieves a single voice message
func (h *Handler) HandleGetVoiceMessage(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
	if err != nil {
		return httputil.BadRequest("Invalid message ID")
	}

	h.log.Debug("get voice message request",
		"user_id", userID,
		"message_id", messageID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	message, err := h.dbStore.GetVoiceMessageByID(ctx, messageID)
	if err != nil {
		h.log.Debug("voice message not found",
			"message_id", messageID,
			"error", err)
		return httputil.NotFound("Message not found")
	}

	// Verify user is in the room
	isInRoom, err := h.roomStore.IsUserInRoom(ctx, message.RoomID, userID)
	if err != nil {
		h.log.Error("failed to verify room membership",
			"user_id", userID,
			"room_id", message.RoomID,
			"error", err)
		return httputil.Internal(err)
	}
	if !isInRoom {
		h.log.Warn("get voice message blocked - user not in room",
			"user_id", userID,
			"room_id", message.RoomID,
			"message_id", messageID)
		return httputil.Forbidden("You are not a member of this room")
	}

	// Generate presigned URL
	url, err := h.fileStore.GetPresignedURL(ctx, message.S3Key, urlExpiryTime)
	if err != nil {
		h.log.Warn("failed to generate presigned URL",
			"message_id", messageID,
			"s3_key", message.S3Key,
			"error", err)
		url = ""
	}

	response := VoiceMessageWithURL{
		VoiceMessage: *message,
		URL:          url,
	}

	return httputil.RespondJSON(w, http.StatusOK, response)
}

// HandleDeleteVoiceMessage deletes a voice message (only by sender)
func (h *Handler) HandleDeleteVoiceMessage(w http.ResponseWriter, r *http.Request) error {
	userID := auth.GetUserID(r.Context())
	messageID, err := uuid.Parse(chi.URLParam(r, "messageID"))
	if err != nil {
		return httputil.BadRequest("Invalid message ID")
	}

	h.log.Debug("delete voice message request",
		"user_id", userID,
		"message_id", messageID)

	ctx, cancel := h.dbCtx(r)
	defer cancel()

	// Get the message
	message, err := h.dbStore.GetVoiceMessageByID(ctx, messageID)
	if err != nil {
		h.log.Debug("voice message not found for deletion",
			"message_id", messageID,
			"error", err)
		return httputil.NotFound("Message not found")
	}

	// Only sender can delete their own messages
	if message.SenderID != userID {
		h.log.Warn("delete voice message blocked - not message owner",
			"user_id", userID,
			"message_id", messageID,
			"owner_id", message.SenderID)
		return httputil.Forbidden("You can only delete your messages")
	}

	// Delete from S3 first
	if err := h.fileStore.DeleteVoiceMessage(ctx, message.S3Key); err != nil {
		h.log.Error("failed to delete voice message from S3",
			"message_id", messageID,
			"s3_key", message.S3Key,
			"error", err)
		// Continue to delete from DB anyway
	}

	// Delete from database
	if err := h.dbStore.DeleteVoiceMessage(ctx, messageID); err != nil {
		h.log.Error(
			"failed to delete voice message from database",
			"message_id", messageID,
			"error", err)
		return httputil.Internal(err)
	}

	h.log.Info(
		"voice message deleted successfully",
		"message_id", messageID,
		"deleted_by", userID,
		"room_id", message.RoomID)

	return httputil.RespondJSON(w, http.StatusOK, "Message deleted successfully")
}
