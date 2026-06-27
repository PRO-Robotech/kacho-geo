// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package handler_test

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	operationpb "github.com/PRO-Robotech/kacho-corelib/proto/gen/go/kacho/cloud/operation"
	geov1 "github.com/PRO-Robotech/kacho-geo/proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/handler"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

// awaitOpFromHandler поллит OperationHandler.Get до done=true (тот же контракт,
// что у клиента после async-мутации).
func awaitOpFromHandler(t *testing.T, oh *handler.OperationHandler, opID string) *operationpb.Operation {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		op, err := oh.Get(context.Background(), &operationpb.GetOperationRequest{OperationId: opID})
		if err == nil && op.GetDone() {
			return op
		}
		if time.Now().After(deadline) {
			t.Fatalf("operation %s did not finish", opID)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestRegionHandler_Get_notFound_mapsToNotFound(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Region, error) {
			return nil, geoerrors.Wrap(stderrors.New("x"), "Region", id) // путь ErrInternal
		},
	}, repomock.NewOpsRepo())
	h := handler.NewRegionHandler(uc)
	_, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: "x"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want INTERNAL for raw error, got %v", err)
	}
}

func TestRegionHandler_Get_emptyID_invalidArgument(t *testing.T) {
	h := handler.NewRegionHandler(region.New(&repomock.RegionRepo{}, repomock.NewOpsRepo()))
	_, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want INVALID_ARGUMENT for empty id, got %v", err)
	}
}

func TestRegionHandler_Get_happy(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Region, error) {
			return &domain.Region{ID: id, Name: "Region 1"}, nil
		},
	}, repomock.NewOpsRepo())
	h := handler.NewRegionHandler(uc)
	resp, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: "region-1"})
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if resp.GetId() != "region-1" || resp.GetName() != "Region 1" {
		t.Fatalf("Get resp = %+v", resp)
	}
}

func TestZoneHandler_Get_happy_statusMapped(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Zone, error) {
			return &domain.Zone{ID: id, RegionID: "region-1", Status: domain.ZoneStatusUp}, nil
		},
	}, repomock.NewOpsRepo())
	h := handler.NewZoneHandler(uc)
	resp, err := h.Get(context.Background(), &geov1.GetZoneRequest{ZoneId: "region-1-a"})
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if resp.GetRegionId() != "region-1" || resp.GetStatus() != geov1.Zone_UP {
		t.Fatalf("Get resp = %+v", resp)
	}
}

// TestInternalRegionHandler_Delete_emptyID_syncInvalidArgument — пустой id
// отвергается синхронно (InvalidArgument), Operation не создаётся.
func TestInternalRegionHandler_Delete_emptyID_syncInvalidArgument(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{}, repomock.NewOpsRepo())
	h := handler.NewInternalRegionHandler(uc)
	_, err := h.Delete(context.Background(), &geov1.DeleteRegionRequest{RegionId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want INVALID_ARGUMENT for empty id, got %v", err)
	}
}

// TestInternalRegionHandler_Delete_failedPrecondition — FK RESTRICT (есть зоны)
// доезжает как Operation.error FAILED_PRECONDITION (async).
func TestInternalRegionHandler_Delete_failedPrecondition(t *testing.T) {
	ops := repomock.NewOpsRepo()
	uc := region.New(&repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { return geoerrors.ErrFailedPrecondition },
	}, ops)
	h := handler.NewInternalRegionHandler(uc)
	oh := handler.NewOperationHandler(ops)
	op, err := h.Delete(context.Background(), &geov1.DeleteRegionRequest{RegionId: "region-1"})
	if err != nil {
		t.Fatalf("Delete accept err = %v", err)
	}
	if op.GetDone() {
		t.Fatalf("op must start done=false")
	}
	done := awaitOpFromHandler(t, oh, op.GetId())
	if done.GetError() == nil || done.GetError().GetCode() != int32(codes.FailedPrecondition) {
		t.Fatalf("op error = %v, want FAILED_PRECONDITION", done.GetError())
	}
}

// TestInternalZoneHandler_Create_happy — Create возвращает Operation; полл до
// done → response=Zone.
func TestInternalZoneHandler_Create_happy(t *testing.T) {
	ops := repomock.NewOpsRepo()
	uc := zone.New(&repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	}, ops)
	h := handler.NewInternalZoneHandler(uc)
	oh := handler.NewOperationHandler(ops)
	op, err := h.Create(context.Background(), &geov1.CreateZoneRequest{
		Id: "region-1-a", RegionId: "region-1", Status: geov1.Zone_UP, Name: "Region 1 A",
	})
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	done := awaitOpFromHandler(t, oh, op.GetId())
	if done.GetError() != nil {
		t.Fatalf("op error = %v", done.GetError())
	}
	msg, err := done.GetResponse().UnmarshalNew()
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	z, ok := msg.(*geov1.Zone)
	if !ok || z.GetId() != "region-1-a" || z.GetStatus() != geov1.Zone_UP {
		t.Fatalf("response = %+v", msg)
	}
}

// TestOperationHandler_Get_notFound — несуществующий operation_id → NOT_FOUND.
func TestOperationHandler_Get_notFound(t *testing.T) {
	oh := handler.NewOperationHandler(repomock.NewOpsRepo())
	_, err := oh.Get(context.Background(), &operationpb.GetOperationRequest{OperationId: "geo-absent"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("want NOT_FOUND, got %v", err)
	}
}
