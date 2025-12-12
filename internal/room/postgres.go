package room

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool}
}

// CreateRoom creates a new room
func (s *PostgresStore) CreateRoom(ctx context.Context, room *Room) error {
	query := `
		INSERT INTO rooms (id, created_at, updated_at)
		VALUES ($1, $2, $3)
	`

	room.ID = uuid.New()
	now := time.Now()
	room.CreatedAt = now
	room.UpdatedAt = now

	_, err := s.pool.Exec(ctx, query, room.ID, room.CreatedAt, room.UpdatedAt)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("failed to create room: %w", err)
	}

	return nil
}

// GetRoomByID retrieves a room by its ID
func (s *PostgresStore) GetRoomByID(ctx context.Context, roomID uuid.UUID) (*Room, error) {
	query := `
		SELECT id, created_at, updated_at
		FROM rooms
		WHERE id = $1
	`

	room := &Room{}
	err := s.pool.QueryRow(ctx, query, roomID).Scan(
		&room.ID,
		&room.CreatedAt,
		&room.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("room not found")
		}
		return nil, fmt.Errorf("failed to get room: %w", err)
	}

	return room, nil
}

// DeleteRoom deletes a room (cascades to participants and messages)
func (s *PostgresStore) DeleteRoom(ctx context.Context, roomID uuid.UUID) error {
	query := `DELETE FROM rooms WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, roomID)
	if err != nil {
		return fmt.Errorf("failed to delete room: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("room not found")
	}

	return nil
}

// AddParticipant adds a user to a room
func (s *PostgresStore) AddParticipant(ctx context.Context, participant *RoomParticipant) error {
	query := `
		INSERT INTO room_participants (id, room_id, user_id, joined_at)
		VALUES ($1, $2, $3, $4)
	`

	participant.ID = uuid.New()
	participant.JoinedAt = time.Now()

	_, err := s.pool.Exec(ctx, query,
		participant.ID,
		participant.RoomID,
		participant.UserID,
		participant.JoinedAt,
	)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("failed to add participant: %w", err)
	}

	return nil
}

// RemoveParticipant removes a user from a room
func (s *PostgresStore) RemoveParticipant(ctx context.Context, roomID, userID uuid.UUID) error {
	query := `
		DELETE FROM room_participants
		WHERE room_id = $1 AND user_id = $2
	`

	result, err := s.pool.Exec(ctx, query, roomID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove participant: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("participant not found in room")
	}

	return nil
}

// GetRoomParticipants gets all participants in a room
func (s *PostgresStore) GetRoomParticipants(ctx context.Context, roomID uuid.UUID) ([]*RoomParticipant, error) {
	query := `
		SELECT id, room_id, user_id, joined_at
		FROM room_participants
		WHERE room_id = $1
		ORDER BY joined_at ASC
	`

	rows, err := s.pool.Query(ctx, query, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to get participants: %w", err)
	}
	defer rows.Close()

	participants := []*RoomParticipant{}
	for rows.Next() {
		p := &RoomParticipant{}
		err := rows.Scan(&p.ID, &p.RoomID, &p.UserID, &p.JoinedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan participant: %w", err)
		}
		participants = append(participants, p)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating participants: %w", err)
	}

	return participants, nil
}

// IsUserInRoom checks if a user is a participant in a room
func (s *PostgresStore) IsUserInRoom(ctx context.Context, roomID, userID uuid.UUID) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM room_participants
			WHERE room_id = $1 AND user_id = $2
		)
	`

	var exists bool
	err := s.pool.QueryRow(ctx, query, roomID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user in room: %w", err)
	}

	return exists, nil
}

// GetUserRooms gets all rooms a user is participating in
func (s *PostgresStore) GetUserRooms(ctx context.Context, userID uuid.UUID) ([]*Room, error) {
	query := `
		SELECT r.id, r.created_at, r.updated_at
		FROM rooms r
		INNER JOIN room_participants rp ON r.id = rp.room_id
		WHERE rp.user_id = $1
		ORDER BY r.updated_at DESC
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user rooms: %w", err)
	}
	defer rows.Close()

	rooms := []*Room{}
	for rows.Next() {
		room := &Room{}
		err := rows.Scan(&room.ID, &room.CreatedAt, &room.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan room: %w", err)
		}
		rooms = append(rooms, room)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rooms: %w", err)
	}

	return rooms, nil
}
