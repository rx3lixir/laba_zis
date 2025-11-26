package maindb

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBTX is an interface for database operations
// it allows to swap between pool and transactions
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UserStore defines all user-related database operations
type UserStore interface {
	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUsers(ctx context.Context, limit, offset int) ([]*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

// PostgresStore is a main database store
type PostgresStore struct {
	db DBTX
}

// NewPostgresStore creates a new store
func NewPostgresStore(db DBTX) *PostgresStore {
	return &PostgresStore{
		db: db,
	}
}

// CreatePostgresPool creates and pings a connection pool
func CreatePostgresPool(parentCtx context.Context, dburl string) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(parentCtx, time.Second*3)
	defer cancel()

	pool, err := pgxpool.New(ctx, dburl)
	if err != nil {
		return nil, err
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
