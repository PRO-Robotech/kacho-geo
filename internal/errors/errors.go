// Package errors — repo-layer sentinel errors + SQLSTATE→sentinel translation
// for kacho-geo. Lives in a leaf package (no pgx/grpc import-cycle): repo wraps
// pgx errors into these sentinels; the use-case maps sentinels → gRPC status.
package errors

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel errors returned by the repo layer. Never leak raw pgx text outward
// (mapped to fixed gRPC INTERNAL "internal database error" by the use-case).
var (
	// ErrNotFound — row does not exist (pgx.ErrNoRows).
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists — UNIQUE / PK violation (SQLSTATE 23505).
	ErrAlreadyExists = errors.New("already exists")
	// ErrFailedPrecondition — FK violation / state conflict (SQLSTATE 23503).
	ErrFailedPrecondition = errors.New("failed precondition")
	// ErrInvalidArg — CHECK violation (SQLSTATE 23514).
	ErrInvalidArg = errors.New("invalid argument")
	// ErrInternal — uncategorised DB error (no pgx-text leak).
	ErrInternal = errors.New("internal database error")
)

// Wrap translates a pgx/pgconn error into a kacho-geo sentinel, attaching a
// stable, leak-free message ("<Resource> <id> not found"). SQLSTATE mapping:
//
//	pgx.ErrNoRows → ErrNotFound
//	23505 UNIQUE  → ErrAlreadyExists
//	23503 FK      → ErrFailedPrecondition
//	23514 CHECK   → ErrInvalidArg
//	anything else → ErrInternal
//
// resource is a human label ("Region" / "Zone"); id is the resource id (may be "").
func Wrap(err error, resource, id string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s %s not found", ErrNotFound, resource, id)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s %s already exists", ErrAlreadyExists, resource, id)
		case "23503": // foreign_key_violation
			return fmt.Errorf("%w: %s reference constraint", ErrFailedPrecondition, resource)
		case "23514": // check_violation
			return fmt.Errorf("%w: invalid %s", ErrInvalidArg, resource)
		}
	}
	// Defensive: never leak raw pgx text — fixed sentinel.
	return ErrInternal
}
