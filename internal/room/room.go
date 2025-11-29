package room

import (
	"time"

	"github.com/google/uuid"
)

type Room struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RoomParticipant struct {
	ID       uuid.UUID `json:"id"`
	RoomID   uuid.UUID `json:"room_id"`
	UserID   uuid.UUID `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type CreateRoomRequest struct {
	ParticipantIDs []uuid.UUID `json:"participants_ids"`
}

type CreateRoomResponse struct {
	Room         Room              `json:"room"`
	Participants []RoomParticipant `json:"participants"`
}

type AddParticipantRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

type RoomResponse struct {
	Room         Room              `json:"room"`
	Participants []RoomParticipant `json:"participants"`
}

type GetUserRoomsResponse struct {
	Rooms []RoomResponse `json:"rooms"`
	Count int            `json:"count"`
}
