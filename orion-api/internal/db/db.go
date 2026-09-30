package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a connection pool to Postgres. A "pool" is a small set of
// already-open connections that handlers borrow and return, rather than
// opening a brand new TCP connection to Postgres on every request — much
// faster and is how every production Go service talks to a database.
func NewPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	// Ping proves the connection actually works right now, instead of
	// finding out on the first real request. Fail fast at startup.
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}