// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package errors — sentinel-ошибки repo-слоя + трансляция SQLSTATE→sentinel для
// kacho-geo. Живет в leaf-пакете (без import-цикла pgx/grpc): repo заворачивает
// pgx-ошибки в эти sentinel'ы, а use-case маппит sentinel → gRPC status.
package errors

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Sentinel-ошибки repo-слоя. Сырой pgx-текст наружу не утекает (use-case маппит
// их в фиксированный gRPC INTERNAL "internal database error").
var (
	// ErrNotFound — строки не существует (pgx.ErrNoRows).
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists — нарушение UNIQUE / PK (SQLSTATE 23505).
	ErrAlreadyExists = errors.New("already exists")
	// ErrFailedPrecondition — нарушение FK / конфликт состояния (SQLSTATE 23503).
	ErrFailedPrecondition = errors.New("failed precondition")
	// ErrInvalidArg — нарушение CHECK (SQLSTATE 23514).
	ErrInvalidArg = errors.New("invalid argument")
	// ErrInternal — некатегоризированная ошибка БД (без утечки pgx-текста).
	ErrInternal = errors.New("internal database error")
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
		return fmt.Errorf("%w: %s %s not found", ErrNotFound, resource, id)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return fmt.Errorf("%w: %s %s already exists", ErrAlreadyExists, resource, id)
		case "23503": // foreign_key_violation — direction-neutral: 23503 летит и на
			// parent-delete (Region.Delete с зонами), и на child-insert/update (Zone
			// с несуществующим region_id). Текст не привязан к направлению.
			return fmt.Errorf("%w: %s %s violates a reference constraint", ErrFailedPrecondition, resource, id)
		case "23514": // check_violation
			return fmt.Errorf("%w: invalid %s", ErrInvalidArg, resource)
		}
	}
	// Защитно: сырой pgx-текст наружу не отдаем — фиксированный sentinel.
	return ErrInternal
}
