package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CreatePostgresPool creates and pings a connection pool
func NewPool(parentCtx context.Context, dburl string) (*pgxpool.Pool, error) {
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
