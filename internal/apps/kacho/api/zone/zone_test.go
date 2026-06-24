// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package zone_test

import (
	"context"
	stderrors "errors"
	"testing"

	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

func TestGet_emptyID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{})
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
	uc := zone.New(mock)
	z, err := uc.Get(context.Background(), "region-1-a")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if z.ID != "region-1-a" || z.RegionID != "region-1" || z.Status != domain.ZoneStatusUp {
		t.Fatalf("Get = %+v", z)
	}
}

func TestCreate_emptyRegionID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{})
	_, err := uc.Create(context.Background(), "region-1-a", "", "x", domain.ZoneStatusUp)
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Create(region_id='') err = %v, want ErrInvalidArg", err)
	}
}

func TestCreate_happy(t *testing.T) {
	mock := &repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	}
	uc := zone.New(mock)
	z, err := uc.Create(context.Background(), "region-1-a", "region-1", "Region 1 A", domain.ZoneStatusUp)
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if z.ID != "region-1-a" || z.RegionID != "region-1" {
		t.Fatalf("Create = %+v", z)
	}
}

func TestUpdate_status(t *testing.T) {
	var got zone.UpdateParams
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, id string, p zone.UpdateParams) (*domain.Zone, error) {
			got = p
			// Repo возвращает результирующую строку (после атомарного UPDATE).
			return &domain.Zone{ID: id, RegionID: "region-1", Status: domain.ZoneStatusDown}, nil
		},
	}
	uc := zone.New(mock)
	z, err := uc.Update(context.Background(), "region-1-a", "", "", domain.ZoneStatusDown)
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	if z.Status != domain.ZoneStatusDown {
		t.Fatalf("Update status = %v, want DOWN", z.Status)
	}
	// Статус задан → передан в repo; regionID/name не заданы → nil (не меняются).
	if got.Status == nil || *got.Status != domain.ZoneStatusDown {
		t.Fatalf("UpdateParams.Status = %v, want &DOWN", got.Status)
	}
	if got.RegionID != nil || got.Name != nil {
		t.Fatalf("UpdateParams = %+v, want regionID/name nil", got)
	}
}

// TestUpdate_unspecifiedStatus_keepsExisting — партиал-Update без статуса
// (omitempty → ZoneStatusUnspecified=0) не должен затирать существующий статус:
// use-case передает Status=nil в repo (репо делает атомарный COALESCE-апдейт,
// не читая строку заранее). Меняется только name.
func TestUpdate_unspecifiedStatus_keepsExisting(t *testing.T) {
	var got zone.UpdateParams
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, id string, p zone.UpdateParams) (*domain.Zone, error) {
			got = p
			// Репо вернул бы строку с прежним статусом UP (status не тронут).
			return &domain.Zone{ID: id, RegionID: "region-1", Name: "new-name", Status: domain.ZoneStatusUp}, nil
		},
	}
	uc := zone.New(mock)
	z, err := uc.Update(context.Background(), "region-1-a", "", "new-name", domain.ZoneStatusUnspecified)
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	if got.Status != nil {
		t.Fatalf("UpdateParams.Status = %v, want nil (unspecified must not overwrite)", got.Status)
	}
	if got.Name == nil || *got.Name != "new-name" {
		t.Fatalf("UpdateParams.Name = %v, want &new-name", got.Name)
	}
	if z.Status != domain.ZoneStatusUp {
		t.Fatalf("Update status = %v, want UP", z.Status)
	}
}

// TestUpdate_invalidStatus_invalidArg — out-of-range статус отвергается domain
// Validate перед записью → ErrInvalidArg (репо не зовется).
func TestUpdate_invalidStatus_invalidArg(t *testing.T) {
	called := false
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, _ string, _ zone.UpdateParams) (*domain.Zone, error) {
			called = true
			return nil, nil
		},
	}
	uc := zone.New(mock)
	_, err := uc.Update(context.Background(), "region-1-a", "", "", domain.ZoneStatus(99))
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Update(status=99) err = %v, want ErrInvalidArg", err)
	}
	if called {
		t.Fatal("repo.Update must not be called on invalid status")
	}
}

// TestUpdate_notFound — repo.Update возвращает ErrNotFound (0 rows из RETURNING →
// pgx.ErrNoRows → ErrNotFound); use-case пробрасывает без подмены.
func TestUpdate_notFound(t *testing.T) {
	mock := &repomock.ZoneRepo{
		UpdateFunc: func(_ context.Context, _ string, _ zone.UpdateParams) (*domain.Zone, error) {
			return nil, geoerrors.ErrNotFound
		},
	}
	uc := zone.New(mock)
	_, err := uc.Update(context.Background(), "no-such-zone", "", "new-name", domain.ZoneStatusDown)
	if !stderrors.Is(err, geoerrors.ErrNotFound) {
		t.Fatalf("Update err = %v, want ErrNotFound", err)
	}
}

func TestDelete_emptyID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{})
	err := uc.Delete(context.Background(), "")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Delete('') err = %v, want ErrInvalidArg", err)
	}
}
