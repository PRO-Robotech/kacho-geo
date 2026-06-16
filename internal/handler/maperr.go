package handler

import (
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// mapErr translates a use-case/repo error into a gRPC status, stripping the
// sentinel prefix so the client sees the stable Kachō message ("Region %s not
// found"). Unclassified errors → fixed INTERNAL "internal database error" (no
// pgx-text leak — also on the cluster-internal listener).
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
	// Already a gRPC status (e.g. validate.PageSize) — pass through.
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err
	}
	// Defensive: raw error → fixed INTERNAL (no leak).
	return status.Error(codes.Internal, "internal database error")
}

// strip removes the "<sentinel>: " prefix so the client sees the stable message.
func strip(err, sentinel error) string {
	msg := err.Error()
	prefix := sentinel.Error() + ": "
	if rest, ok := strings.CutPrefix(msg, prefix); ok {
		return rest
	}
	return msg
}
