package voice

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

// CreateVoiceMessage creates a voice message record in the database
func (s *PostgresStore) CreateVoiceMessage(ctx context.Context, message *VoiceMessage) error {
	query := `
		INSERT INTO voice_messages (id, room_id, sender_id, s3_key, duration_seconds, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	message.ID = uuid.New()
	message.CreatedAt = time.Now()

	_, err := s.pool.Exec(ctx, query,
		message.ID,
		message.RoomID,
		message.SenderID,
		message.S3Key,
		message.DurationSeconds,
		message.CreatedAt,
	)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("failed to create voice message: %w", err)
	}

	return nil
}

// GetVoiceMessageByID retrieves a voice message by ID
func (s *PostgresStore) GetVoiceMessageByID(ctx context.Context, messageID uuid.UUID) (*VoiceMessage, error) {
	query := `
		SELECT id, room_id, sender_id, s3_key, duration_seconds, created_at
		FROM voice_messages
		WHERE id = $1
	`

	message := &VoiceMessage{}
	err := s.pool.QueryRow(ctx, query, messageID).Scan(
		&message.ID,
		&message.RoomID,
		&message.SenderID,
		&message.S3Key,
		&message.DurationSeconds,
		&message.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("voice message not found")
		}
		return nil, fmt.Errorf("failed to get voice message: %w", err)
	}

	return message, nil
}

// GetRoomMessages retrieves all voice messages in a room with pagination
func (s *PostgresStore) GetRoomMessages(ctx context.Context, roomID uuid.UUID, limit, offset int) ([]*VoiceMessage, error) {
	query := `
		SELECT id, room_id, sender_id, s3_key, duration_seconds, created_at
		FROM voice_messages
		WHERE room_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.pool.Query(ctx, query, roomID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get room messages: %w", err)
	}
	defer rows.Close()

	messages := []*VoiceMessage{}
	for rows.Next() {
		msg := &VoiceMessage{}
		err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.SenderID,
			&msg.S3Key,
			&msg.DurationSeconds,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan voice message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating voice messages: %w", err)
	}

	return messages, nil
}

// DeleteVoiceMessage deletes a voice message record from the database
func (s *PostgresStore) DeleteVoiceMessage(ctx context.Context, messageID uuid.UUID) error {
	query := `DELETE FROM voice_messages WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, messageID)
	if err != nil {
		return fmt.Errorf("failed to delete voice message: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("voice message not found")
	}

	return nil
}

// GetMessagesBySender retrieves all messages sent by a specific user
func (s *PostgresStore) GetMessagesBySender(ctx context.Context, senderID uuid.UUID, limit, offset int) ([]*VoiceMessage, error) {
	query := `
		SELECT id, room_id, sender_id, s3_key, duration_seconds, created_at
		FROM voice_messages
		WHERE sender_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := s.pool.Query(ctx, query, senderID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get sender messages: %w", err)
	}
	defer rows.Close()

	messages := []*VoiceMessage{}
	for rows.Next() {
		msg := &VoiceMessage{}
		err := rows.Scan(
			&msg.ID,
			&msg.RoomID,
			&msg.SenderID,
			&msg.S3Key,
			&msg.DurationSeconds,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan voice message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating voice messages: %w", err)
	}

	return messages, nil
}
