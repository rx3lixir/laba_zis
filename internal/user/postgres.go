package user

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

// CreateUser creates a new user in Postgres
func (s *PostgresStore) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, username, email, password, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	user.ID = uuid.New()
	now := time.Now()

	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := s.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.Password,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByID retrieves a user with passed ID from Postgres
func (s *PostgresStore) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, username, email, password, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	user := &User{}
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by passed email from Postgres
func (s *PostgresStore) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, username, email, password, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	user := &User{}
	err := s.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetAllUsers retrieves all users with pagination from Postgres
func (s *PostgresStore) GetAllUsers(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, username, email, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// UpdateUser updates an existing user in Postgres
func (s *PostgresStore) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users
		SET username = $2, email = $3, updated_at = $4
		WHERE id = $1
	`
	user.UpdatedAt = time.Now()

	result, err := s.pool.Exec(ctx, query,
		user.ID,
		user.Username,
		user.Email,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user by ID from Postgres
func (s *PostgresStore) DeleteUser(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
