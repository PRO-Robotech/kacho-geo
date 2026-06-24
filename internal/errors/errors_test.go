// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package errors_test

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

func TestWrap_noRows_notFound(t *testing.T) {
	err := geoerrors.Wrap(pgx.ErrNoRows, "Region", "region-1")
	if !stderrors.Is(err, geoerrors.ErrNotFound) {
		t.Fatalf("Wrap(ErrNoRows) = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "Region region-1 not found") {
		t.Fatalf("Wrap msg = %q, want stable not-found text", err.Error())
	}
}

// TestWrap_fkViolation_directionNeutral — 23503 летит и на parent-delete
// (Region.Delete с зонами), и на child-insert (Zone с несуществующим region_id).
// Сообщение обязано быть direction-neutral (не «referenced by», что верно только
// для parent-delete).
func TestWrap_fkViolation_directionNeutral(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23503"}
	err := geoerrors.Wrap(pgErr, "Zone", "region-1-a")
	if !stderrors.Is(err, geoerrors.ErrFailedPrecondition) {
		t.Fatalf("Wrap(23503) = %v, want ErrFailedPrecondition", err)
	}
	if !strings.Contains(err.Error(), "Zone region-1-a violates a reference constraint") {
		t.Fatalf("Wrap(23503) msg = %q, want direction-neutral reference-constraint text", err.Error())
	}
	if strings.Contains(err.Error(), "referenced by") {
		t.Fatalf("Wrap(23503) msg = %q, must not contain direction-specific 'referenced by'", err.Error())
	}
}

func TestWrap_checkViolation_invalidArg(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23514"}
	err := geoerrors.Wrap(pgErr, "Zone", "z-1")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Wrap(23514) = %v, want ErrInvalidArg", err)
	}
}
