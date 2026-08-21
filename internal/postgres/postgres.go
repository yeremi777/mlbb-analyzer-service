// Package postgres holds every SQL statement in the service. Queries are plain
// functions taking a connection and domain arguments; the caller owns the
// transaction boundary and therefore owns what a partial failure means.
//
// Reads take Querier, writes take pgx.Tx. Where an orchestrator needs to
// substitute the database in tests, a small struct in this package adapts these
// functions to the interface that orchestrator declares.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the read-only surface shared by pgxpool.Pool, pgx.Conn, and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Executor is a handle that can query directly and also open a transaction:
// satisfied by pgxpool.Pool and pgx.Conn, but not by pgx.Tx.
type Executor interface {
	Querier
	Begin(ctx context.Context) (pgx.Tx, error)
}
