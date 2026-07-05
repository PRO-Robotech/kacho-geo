// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package dberr — repo-adjacent adapter: трансляция ошибок pgx/pgconn в чистые
// sentinel'ы internal/errors. Зависит от pgx (Postgres-driver), поэтому вынесен
// из leaf-пакета internal/errors — так use-case/domain тянут только sentinel'ы,
// а pgx остаётся в dependency-closure одного лишь repo-слоя (ports/adapters).
package dberr

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// Wrap транслирует ошибку pgx/pgconn в sentinel kacho-geo, прикрепляя стабильное
// сообщение без утечек ("<Resource> <id> not found"). Маппинг SQLSTATE:
//
//	pgx.ErrNoRows → ErrNotFound
//	23505 UNIQUE  → ErrAlreadyExists
//	23503 FK      → ErrFailedPrecondition
//	23514 CHECK   → ErrInvalidArg
//	все остальное → ErrInternal
//
// resource — человекочитаемый ярлык ("Region" / "Zone"); id — id ресурса (может быть "").
func Wrap(err error, resource, id string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: %s %s not found", geoerrors.ErrNotFound, resource, id)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s %s already exists", geoerrors.ErrAlreadyExists, resource, id)
		case "23503": // foreign_key_violation — direction-neutral: 23503 летит и на
			// parent-delete (Region.Delete с зонами), и на child-insert/update (Zone
			// с несуществующим region_id). Текст не привязан к направлению.
			return fmt.Errorf("%w: %s %s violates a reference constraint", geoerrors.ErrFailedPrecondition, resource, id)
		case "23514": // check_violation
			return fmt.Errorf("%w: invalid %s", geoerrors.ErrInvalidArg, resource)
		}
	}
	// Защитно: сырой pgx-текст наружу не отдаем — фиксированный sentinel.
	return geoerrors.ErrInternal
}
