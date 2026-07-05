// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package handler_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PRO-Robotech/kacho-corelib/operations"
	operationpb "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/operation"
	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/shared/lro"
	"github.com/PRO-Robotech/kacho-geo/internal/handler"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

// seedOwnedOp кладёт в мок незавершённую операцию, созданную principal'ом owner
// (через ctx-WithPrincipal → NewFromContext переносит principal в op.Principal,
// мок сохраняет его). Возвращает op-id.
func seedOwnedOp(t *testing.T, ops *repomock.OpsRepo, owner operations.Principal) string {
	t.Helper()
	ctx := operations.WithPrincipal(context.Background(), owner)
	op, err := operations.NewFromContext(ctx, lro.OperationPrefix, "create region",
		&geov1.CreateRegionMetadata{RegionId: "region-1"})
	if err != nil {
		t.Fatalf("NewFromContext: %v", err)
	}
	if err := ops.Create(ctx, op); err != nil {
		t.Fatalf("ops.Create: %v", err)
	}
	return op.ID
}

var (
	adminA = operations.Principal{Type: "user", ID: "usr_adminA"}
	adminB = operations.Principal{Type: "user", ID: "usr_adminB"}
)

// TestOperationHandler_Get_foreignPrincipal_NotFound — BOLA-гейт: caller B не
// владелец операции A → NotFound (no-leak, неотличимо от «нет такой»), НЕ отдаёт
// чужую операцию.
func TestOperationHandler_Get_foreignPrincipal_NotFound(t *testing.T) {
	ops := repomock.NewOpsRepo()
	opID := seedOwnedOp(t, ops, adminA)
	oh := handler.NewOperationHandler(ops)

	ctxB := operations.WithPrincipal(context.Background(), adminB)
	_, err := oh.Get(ctxB, &operationpb.GetOperationRequest{OperationId: opID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign Get: want NOT_FOUND, got %v", err)
	}
}

// TestOperationHandler_Get_owner_ok — владелец A читает свою операцию.
func TestOperationHandler_Get_owner_ok(t *testing.T) {
	ops := repomock.NewOpsRepo()
	opID := seedOwnedOp(t, ops, adminA)
	oh := handler.NewOperationHandler(ops)

	ctxA := operations.WithPrincipal(context.Background(), adminA)
	op, err := oh.Get(ctxA, &operationpb.GetOperationRequest{OperationId: opID})
	if err != nil {
		t.Fatalf("owner Get err = %v", err)
	}
	if op.GetId() != opID {
		t.Fatalf("owner Get id = %q, want %q", op.GetId(), opID)
	}
}

// TestOperationHandler_Cancel_foreignPrincipal_NotFound — caller B не может
// отменить in-flight операцию A → NotFound (integrity-гейт control-plane).
func TestOperationHandler_Cancel_foreignPrincipal_NotFound(t *testing.T) {
	ops := repomock.NewOpsRepo()
	opID := seedOwnedOp(t, ops, adminA)
	oh := handler.NewOperationHandler(ops)

	ctxB := operations.WithPrincipal(context.Background(), adminB)
	_, err := oh.Cancel(ctxB, &operationpb.CancelOperationRequest{OperationId: opID})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("foreign Cancel: want NOT_FOUND, got %v", err)
	}
	// Операция A должна остаться НЕ отменённой (foreign cancel — no-op).
	ctxA := operations.WithPrincipal(context.Background(), adminA)
	op, err := oh.Get(ctxA, &operationpb.GetOperationRequest{OperationId: opID})
	if err != nil {
		t.Fatalf("owner Get after foreign cancel err = %v", err)
	}
	if op.GetDone() {
		t.Fatalf("operation was cancelled by foreign principal — BOLA integrity breach")
	}
}

// TestOperationHandler_Cancel_owner_ok — владелец A отменяет свою in-flight
// операцию (done=true, CANCELLED).
func TestOperationHandler_Cancel_owner_ok(t *testing.T) {
	ops := repomock.NewOpsRepo()
	opID := seedOwnedOp(t, ops, adminA)
	oh := handler.NewOperationHandler(ops)

	ctxA := operations.WithPrincipal(context.Background(), adminA)
	op, err := oh.Cancel(ctxA, &operationpb.CancelOperationRequest{OperationId: opID})
	if err != nil {
		t.Fatalf("owner Cancel err = %v", err)
	}
	if !op.GetDone() {
		t.Fatalf("owner Cancel: op must be done=true")
	}
}

// bareOpsRepo реализует ТОЛЬКО operations.Repo (не OwnedOperationRepo) — модель
// wiring-ошибки. Handler обязан fail-closed (INTERNAL), а не молча пропустить
// ownership-проверку.
type bareOpsRepo struct{ operations.Repo }

// TestOperationHandler_Get_repoWithoutOwned_failClosed — если repo не
// реализует OwnedOperationRepo → INTERNAL (не silent-bypass ownership).
func TestOperationHandler_Get_repoWithoutOwned_failClosed(t *testing.T) {
	oh := handler.NewOperationHandler(bareOpsRepo{Repo: repomock.NewOpsRepo()})
	_, err := oh.Get(context.Background(), &operationpb.GetOperationRequest{OperationId: "x"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("bare repo Get: want INTERNAL (fail-closed), got %v", err)
	}
	_, err = oh.Cancel(context.Background(), &operationpb.CancelOperationRequest{OperationId: "x"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("bare repo Cancel: want INTERNAL (fail-closed), got %v", err)
	}
}
