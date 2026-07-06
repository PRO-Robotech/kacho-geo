// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package zone_test

import (
	"context"
	stderrors "errors"
	"testing"

	"google.golang.org/grpc/codes"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/shared/serviceerr"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

func TestGet_emptyID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{}, &repomock.ZoneRepo{}, repomock.NewOpsRepo(), serviceerr.ToStatus)
	_, err := uc.Get(context.Background(), "")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Get('') err = %v, want ErrInvalidArg", err)
	}
}

func TestGet_happy(t *testing.T) {
	mock := &repomock.ZoneRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Zone, error) {
			return &domain.Zone{ID: id, RegionID: "region-1", Status: domain.ZoneStatusUp}, nil
		},
	}
	uc := zone.New(mock, mock, repomock.NewOpsRepo(), serviceerr.ToStatus)
	z, err := uc.Get(context.Background(), "region-1-a")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if z.ID != "region-1-a" || z.RegionID != "region-1" || z.Status != domain.ZoneStatusUp {
		t.Fatalf("Get = %+v", z)
	}
}

// TestCreate_emptyRegionID_invalidArg — пустой region_id отвергается синхронно.
func TestCreate_emptyRegionID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{}, &repomock.ZoneRepo{}, repomock.NewOpsRepo(), serviceerr.ToStatus)
	_, err := uc.Create(context.Background(), "region-1-a", "", "x", domain.ZoneStatusUp)
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Create(region_id='') err = %v, want ErrInvalidArg", err)
	}
}

// TestCreate_happy — валидный вход → Operation → worker → response=Zone.
func TestCreate_happy(t *testing.T) {
	ops := repomock.NewOpsRepo()
	mock := &repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	}
	uc := zone.New(mock, mock, ops, serviceerr.ToStatus)
	op, err := uc.Create(context.Background(), "region-1-a", "region-1", "Region 1 A", domain.ZoneStatusUp)
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if op.ID == "" || op.Done {
		t.Fatalf("Create op = %+v", op)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error != nil {
		t.Fatalf("op.Error = %v", done.Error)
	}
	msg, err := done.Response.UnmarshalNew()
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	z, ok := msg.(*geov1.Zone)
	if !ok || z.GetId() != "region-1-a" || z.GetRegionId() != "region-1" {
		t.Fatalf("response = %+v", msg)
	}
}

// TestUpdate_status — статус задан → передан в repo указателем; response несёт DOWN.
func TestUpdate_status(t *testing.T) {
	ops := repomock.NewOpsRepo()
	var got zone.UpdateParams
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, id string, p zone.UpdateParams) (*domain.Zone, error) {
			got = p
			return &domain.Zone{ID: id, RegionID: "region-1", Status: domain.ZoneStatusDown}, nil
		},
	}
	uc := zone.New(mock, mock, ops, serviceerr.ToStatus)
	op, err := uc.Update(context.Background(), "region-1-a", "", "", domain.ZoneStatusDown)
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error != nil {
		t.Fatalf("op.Error = %v", done.Error)
	}
	if got.Status == nil || *got.Status != domain.ZoneStatusDown {
		t.Fatalf("UpdateParams.Status = %v, want &DOWN", got.Status)
	}
	if got.RegionID != nil || got.Name != nil {
		t.Fatalf("UpdateParams = %+v, want regionID/name nil", got)
	}
}

// TestUpdate_unspecifiedStatus_keepsExisting — Update без статуса не затирает
// существующий (Status=nil → COALESCE в repo); меняется только name.
func TestUpdate_unspecifiedStatus_keepsExisting(t *testing.T) {
	ops := repomock.NewOpsRepo()
	var got zone.UpdateParams
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, id string, p zone.UpdateParams) (*domain.Zone, error) {
			got = p
			return &domain.Zone{ID: id, RegionID: "region-1", Name: "new-name", Status: domain.ZoneStatusUp}, nil
		},
	}
	uc := zone.New(mock, mock, ops, serviceerr.ToStatus)
	op, err := uc.Update(context.Background(), "region-1-a", "", "new-name", domain.ZoneStatusUnspecified)
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	_ = repomock.AwaitOpDone(t, ops, op.ID)
	if got.Status != nil {
		t.Fatalf("UpdateParams.Status = %v, want nil (unspecified must not overwrite)", got.Status)
	}
	if got.Name == nil || *got.Name != "new-name" {
		t.Fatalf("UpdateParams.Name = %v, want &new-name", got.Name)
	}
}

// TestUpdate_invalidStatus_invalidArg — out-of-range статус → синхронный
// ErrInvalidArg (репо не зовётся, операция не пишется).
func TestUpdate_invalidStatus_invalidArg(t *testing.T) {
	called := false
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, _ string, _ zone.UpdateParams) (*domain.Zone, error) {
			called = true
			return nil, nil
		},
	}
	uc := zone.New(mock, mock, repomock.NewOpsRepo(), serviceerr.ToStatus)
	_, err := uc.Update(context.Background(), "region-1-a", "", "", domain.ZoneStatus(99))
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Update(status=99) err = %v, want ErrInvalidArg", err)
	}
	if called {
		t.Fatal("repo.Update must not be called on invalid status")
	}
}

// TestUpdate_notFound — repo.Update возвращает ErrNotFound → Operation.error NOT_FOUND.
func TestUpdate_notFound(t *testing.T) {
	ops := repomock.NewOpsRepo()
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, _ string, _ zone.UpdateParams) (*domain.Zone, error) {
			return nil, geoerrors.ErrNotFound
		},
	}
	uc := zone.New(mock, mock, ops, serviceerr.ToStatus)
	op, err := uc.Update(context.Background(), "no-such-zone", "", "new-name", domain.ZoneStatusDown)
	if err != nil {
		t.Fatalf("Update accept err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error == nil || done.Error.GetCode() != int32(codes.NotFound) {
		t.Fatalf("op.Error = %v, want NOT_FOUND", done.Error)
	}
}

func TestDelete_emptyID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{}, &repomock.ZoneRepo{}, repomock.NewOpsRepo(), serviceerr.ToStatus)
	_, err := uc.Delete(context.Background(), "")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Delete('') err = %v, want ErrInvalidArg", err)
	}
}
