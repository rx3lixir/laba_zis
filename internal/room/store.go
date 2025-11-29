package room

import (
	"context"

	"github.com/google/uuid"
)

type Store interface {
	CreateRoom(ctx context.Context, room *Room) error
	GetRoomByID(ctx context.Context, roomID uuid.UUID) (*Room, error)
	DeleteRoom(ctx context.Context, roomID uuid.UUID) error

	AddParticipant(ctx context.Context, participant *RoomParticipant) error
	RemoveParticipant(ctx context.Context, roomID, userID uuid.UUID) error
	GetRoomParticipants(ctx context.Context, roomID uuid.UUID) ([]*RoomParticipant, error)
	IsUserInRoom(ctx context.Context, roomID, userID uuid.UUID) (bool, error)

	GetUserRooms(ctx context.Context, userID uuid.UUID) ([]*Room, error)
}
