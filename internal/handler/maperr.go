// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package handler

import (
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// mapErr переводит ошибку use-case/repo в gRPC-статус, срезая sentinel-префикс,
// чтобы клиент видел стабильное сообщение Kachō ("Region %s not found").
// Неклассифицированные ошибки → фиксированный INTERNAL "internal database error"
// (без leak'а pgx-текста — в том числе на cluster-internal листенере).
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, geoerrors.ErrNotFound):
		return status.Error(codes.NotFound, strip(err, geoerrors.ErrNotFound))
	case errors.Is(err, geoerrors.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, strip(err, geoerrors.ErrAlreadyExists))
	case errors.Is(err, geoerrors.ErrFailedPrecondition):
		return status.Error(codes.FailedPrecondition, strip(err, geoerrors.ErrFailedPrecondition))
	case errors.Is(err, geoerrors.ErrInvalidArg):
		return status.Error(codes.InvalidArgument, strip(err, geoerrors.ErrInvalidArg))
	case errors.Is(err, geoerrors.ErrInternal):
		return status.Error(codes.Internal, "internal database error")
	}
	// Уже gRPC-статус (например, validate.PageSize) — пропускаем как есть.
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err
	}
	// На всякий случай: сырая ошибка → фиксированный INTERNAL (без leak'а).
	return status.Error(codes.Internal, "internal database error")
}

// strip убирает префикс "<sentinel>: ", чтобы клиент видел стабильное сообщение.
func strip(err, sentinel error) string {
	msg := err.Error()
	prefix := sentinel.Error() + ": "
	if rest, ok := strings.CutPrefix(msg, prefix); ok {
		return rest
	}
	return msg
}
