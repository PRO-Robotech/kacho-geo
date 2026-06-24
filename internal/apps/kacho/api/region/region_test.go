// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package region_test

import (
	"context"
	stderrors "errors"
	"testing"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

func TestGet_emptyID_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{})
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
	uc := region.New(mock)
	r, err := uc.Get(context.Background(), "region-1")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if r.ID != "region-1" || r.Name != "Region 1" {
		t.Fatalf("Get = %+v", r)
	}
}

func TestCreate_emptyID_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{})
	_, err := uc.Create(context.Background(), "", "x")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Create('') err = %v, want ErrInvalidArg", err)
	}
}

func TestCreate_happy(t *testing.T) {
	mock := &repomock.RegionRepo{
		InsertFunc: func(_ context.Context, r *domain.Region) (*domain.Region, error) { return r, nil },
	}
	uc := region.New(mock)
	r, err := uc.Create(context.Background(), "region-1", "Region 1")
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if r.ID != "region-1" {
		t.Fatalf("Create = %+v", r)
	}
}

// TestDelete_repoFKViolation_failedPrecondition — удаление региона с зонами
// блокируется FK RESTRICT (источник истины — DB; repo.Delete возвращает
// ErrFailedPrecondition по SQLSTATE 23503). Прямое DB-поведение покрыто
// integration-тестом TestZoneFKRestrict_DeleteRegionWithZones.
func TestDelete_repoFKViolation_failedPrecondition(t *testing.T) {
	mock := &repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { return geoerrors.ErrFailedPrecondition },
	}
	uc := region.New(mock)
	err := uc.Delete(context.Background(), "region-1")
	if !stderrors.Is(err, geoerrors.ErrFailedPrecondition) {
		t.Fatalf("Delete err = %v, want ErrFailedPrecondition", err)
	}
}

func TestDelete_noZones_ok(t *testing.T) {
	deleted := false
	mock := &repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { deleted = true; return nil },
	}
	uc := region.New(mock)
	if err := uc.Delete(context.Background(), "region-1"); err != nil {
		t.Fatalf("Delete err = %v", err)
	}
	if !deleted {
		t.Fatal("Delete repo not called")
	}
}

// TestUpdate_name_passesPointer — name задан → передается в repo указателем;
// результат — то, что вернул repo (атомарный UPDATE … RETURNING).
func TestUpdate_name_passesPointer(t *testing.T) {
	var gotName *string
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, id string, name *string) (*domain.Region, error) {
			gotName = name
			return &domain.Region{ID: id, Name: "New Name"}, nil
		},
	}
	uc := region.New(mock)
	r, err := uc.Update(context.Background(), "region-1", "New Name")
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	if gotName == nil || *gotName != "New Name" {
		t.Fatalf("repo name = %v, want &New Name", gotName)
	}
	if r.Name != "New Name" {
		t.Fatalf("Update = %+v", r)
	}
}

// TestUpdate_emptyName_noChange — name="" → use-case передает nil (поле не
// меняется), repo делает COALESCE-апдейт.
func TestUpdate_emptyName_noChange(t *testing.T) {
	var gotName *string = new(string)
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, id string, name *string) (*domain.Region, error) {
			gotName = name
			return &domain.Region{ID: id, Name: "unchanged"}, nil
		},
	}
	uc := region.New(mock)
	_, err := uc.Update(context.Background(), "region-1", "")
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	if gotName != nil {
		t.Fatalf("repo name = %v, want nil (empty name must not change)", *gotName)
	}
}

// TestUpdate_notFound — repo.Update возвращает ErrNotFound (0 rows из RETURNING →
// pgx.ErrNoRows → ErrNotFound); use-case пробрасывает без подмены.
func TestUpdate_notFound(t *testing.T) {
	mock := &repomock.RegionRepo{
		UpdateFunc: func(_ context.Context, _ string, _ *string) (*domain.Region, error) {
			return nil, geoerrors.ErrNotFound
		},
	}
	uc := region.New(mock)
	_, err := uc.Update(context.Background(), "no-such-region", "New Name")
	if !stderrors.Is(err, geoerrors.ErrNotFound) {
		t.Fatalf("Update err = %v, want ErrNotFound", err)
	}
}

func TestList_garbagePageSize_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{})
	_, _, err := uc.List(context.Background(), region.Pagination{PageSize: 1_000_000})
	if err == nil {
		t.Fatal("List(page_size too large) err = nil, want validation error")
	}
}
