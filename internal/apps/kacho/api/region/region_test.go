// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package region_test

import (
	"context"
	stderrors "errors"
	"testing"

	"google.golang.org/grpc/codes"

	geov1 "github.com/PRO-Robotech/kacho-geo/proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

func TestGet_emptyID_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{}, repomock.NewOpsRepo())
	_, err := uc.Get(context.Background(), "")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Get('') err = %v, want ErrInvalidArg", err)
	}
}

func TestGet_happy(t *testing.T) {
	mock := &repomock.RegionRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Region, error) {
			return &domain.Region{ID: id, Name: "Region 1"}, nil
		},
	}
	uc := region.New(mock, repomock.NewOpsRepo())
	r, err := uc.Get(context.Background(), "region-1")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if r.ID != "region-1" || r.Name != "Region 1" {
		t.Fatalf("Get = %+v", r)
	}
}

// TestCreate_emptyID_invalidArg — пустой id отвергается СИНХРОННО (операция не пишется).
func TestCreate_emptyID_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{}, repomock.NewOpsRepo())
	_, err := uc.Create(context.Background(), "", "x")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Create('') err = %v, want ErrInvalidArg", err)
	}
}

// TestCreate_happy — валидный вход → Operation(done=false) → worker → response=Region.
func TestCreate_happy(t *testing.T) {
	ops := repomock.NewOpsRepo()
	mock := &repomock.RegionRepo{
		InsertFunc: func(_ context.Context, r *domain.Region) (*domain.Region, error) { return r, nil },
	}
	uc := region.New(mock, ops)
	op, err := uc.Create(context.Background(), "region-1", "Region 1")
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if op.ID == "" || op.Done {
		t.Fatalf("Create op = %+v, want non-empty id and done=false", op)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error != nil {
		t.Fatalf("happy create op.Error = %v", done.Error)
	}
	msg, err := done.Response.UnmarshalNew()
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	r, ok := msg.(*geov1.Region)
	if !ok || r.GetId() != "region-1" {
		t.Fatalf("response = %+v", msg)
	}
}

// TestDelete_repoFKViolation_failedPrecondition — FK RESTRICT (есть зоны) →
// repo.Delete возвращает ErrFailedPrecondition → Operation.error FAILED_PRECONDITION.
func TestDelete_repoFKViolation_failedPrecondition(t *testing.T) {
	ops := repomock.NewOpsRepo()
	mock := &repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { return geoerrors.ErrFailedPrecondition },
	}
	uc := region.New(mock, ops)
	op, err := uc.Delete(context.Background(), "region-1")
	if err != nil {
		t.Fatalf("Delete accept err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error == nil || done.Error.GetCode() != int32(codes.FailedPrecondition) {
		t.Fatalf("op.Error = %v, want FAILED_PRECONDITION", done.Error)
	}
}

// TestDelete_noZones_ok — успешное удаление → Operation done, response=Empty (без тела).
func TestDelete_noZones_ok(t *testing.T) {
	ops := repomock.NewOpsRepo()
	deleted := false
	mock := &repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { deleted = true; return nil },
	}
	uc := region.New(mock, ops)
	op, err := uc.Delete(context.Background(), "region-1")
	if err != nil {
		t.Fatalf("Delete err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error != nil {
		t.Fatalf("op.Error = %v", done.Error)
	}
	if !deleted {
		t.Fatal("Delete repo not called")
	}
}

// TestUpdate_name_passesPointer — name задан → передаётся в repo указателем;
// response несёт обновлённый Region.
func TestUpdate_name_passesPointer(t *testing.T) {
	ops := repomock.NewOpsRepo()
	var gotName *string
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, id string, name *string) (*domain.Region, error) {
			gotName = name
			return &domain.Region{ID: id, Name: "New Name"}, nil
		},
	}
	uc := region.New(mock, ops)
	op, err := uc.Update(context.Background(), "region-1", "New Name")
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error != nil {
		t.Fatalf("op.Error = %v", done.Error)
	}
	if gotName == nil || *gotName != "New Name" {
		t.Fatalf("repo name = %v, want &New Name", gotName)
	}
}

// TestUpdate_emptyName_noChange — name="" → use-case передаёт nil (COALESCE-апдейт).
func TestUpdate_emptyName_noChange(t *testing.T) {
	ops := repomock.NewOpsRepo()
	gotName := new(string)
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, id string, name *string) (*domain.Region, error) {
			gotName = name
			return &domain.Region{ID: id, Name: "unchanged"}, nil
		},
	}
	uc := region.New(mock, ops)
	op, err := uc.Update(context.Background(), "region-1", "")
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	_ = repomock.AwaitOpDone(t, ops, op.ID)
	if gotName != nil {
		t.Fatalf("repo name = %v, want nil (empty name must not change)", *gotName)
	}
}

// TestUpdate_notFound — repo.Update возвращает ErrNotFound → Operation.error NOT_FOUND.
func TestUpdate_notFound(t *testing.T) {
	ops := repomock.NewOpsRepo()
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, _ string, _ *string) (*domain.Region, error) {
			return nil, geoerrors.ErrNotFound
		},
	}
	uc := region.New(mock, ops)
	op, err := uc.Update(context.Background(), "no-such-region", "New Name")
	if err != nil {
		t.Fatalf("Update accept err = %v", err)
	}
	done := repomock.AwaitOpDone(t, ops, op.ID)
	if done.Error == nil || done.Error.GetCode() != int32(codes.NotFound) {
		t.Fatalf("op.Error = %v, want NOT_FOUND", done.Error)
	}
}

func TestList_garbagePageSize_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{}, repomock.NewOpsRepo())
	_, _, err := uc.List(context.Background(), region.Pagination{PageSize: 1_000_000})
	if err == nil {
		t.Fatal("List(page_size too large) err = nil, want validation error")
	}
}
