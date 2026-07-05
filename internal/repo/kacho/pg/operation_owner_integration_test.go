// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package pg_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PRO-Robotech/kacho-corelib/operations"
	operationpb "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/operation"
	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/shared/lro"
	"github.com/PRO-Robotech/kacho-geo/internal/handler"
)

// seedInFlightOwnedOp пишет незавершённую (done=false) LRO-строку через РЕАЛЬНЫЙ
// pgRepo с creator-principal owner (из ctx). Без async-worker'а — строка остаётся
// in-flight, чтобы проверить ownership-scoped Cancel.
func seedInFlightOwnedOp(t *testing.T, ops operations.Repo, owner operations.Principal) string {
	t.Helper()
	ctx := operations.WithPrincipal(context.Background(), owner)
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix, "create region",
		&geov1.CreateRegionMetadata{RegionId: "region-owner"})
	require.NoError(t, err)
	require.NoError(t, ops.Create(ctx, op))
	return op.ID
}

// geo-op-owner-01: BOLA-гейт на РЕАЛЬНОМ pgRepo (ownership-предикат в SQL WHERE).
// Операция создана principal A; caller B (другой principal) НЕ может её ни
// прочитать, ни отменить — оба → NotFound (no-leak), и in-flight операция A
// остаётся неотменённой. Владелец A — читает и отменяет свою.
func TestOperationOwnerScoping_pgRepo(t *testing.T) {
	pool := newTestPool(t)
	ops := operations.NewRepo(pool, "kacho_geo")
	oh := handler.NewOperationHandler(ops)

	adminA := operations.Principal{Type: "user", ID: "usr_owner_A"}
	adminB := operations.Principal{Type: "user", ID: "usr_owner_B"}
	ctxA := operations.WithPrincipal(context.Background(), adminA)
	ctxB := operations.WithPrincipal(context.Background(), adminB)

	opID := seedInFlightOwnedOp(t, ops, adminA)

	// B не владелец → Get NotFound (no-leak).
	_, err := oh.Get(ctxB, &operationpb.GetOperationRequest{OperationId: opID})
	require.Equal(t, codes.NotFound, status.Code(err), "foreign Get must be NOT_FOUND")

	// B не владелец → Cancel NotFound, операция НЕ тронута.
	_, err = oh.Cancel(ctxB, &operationpb.CancelOperationRequest{OperationId: opID})
	require.Equal(t, codes.NotFound, status.Code(err), "foreign Cancel must be NOT_FOUND")

	// A владелец → видит свою in-flight операцию (done=false).
	got, err := oh.Get(ctxA, &operationpb.GetOperationRequest{OperationId: opID})
	require.NoError(t, err)
	require.False(t, got.GetDone(), "operation must remain in-flight after foreign cancel (integrity)")

	// A владелец → отменяет свою операцию.
	cancelled, err := oh.Cancel(ctxA, &operationpb.CancelOperationRequest{OperationId: opID})
	require.NoError(t, err)
	require.True(t, cancelled.GetDone(), "owner Cancel must terminate the operation")
}
