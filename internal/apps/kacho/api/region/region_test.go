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
			return &domain.Region{ID: id, Name: "Russia Central 1"}, nil
		},
	}
	uc := region.New(mock)
	r, err := uc.Get(context.Background(), "ru-central1")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if r.ID != "ru-central1" || r.Name != "Russia Central 1" {
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
	r, err := uc.Create(context.Background(), "eu-west1", "EU West 1")
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if r.ID != "eu-west1" {
		t.Fatalf("Create = %+v", r)
	}
}

func TestDelete_withZones_failedPrecondition(t *testing.T) {
	mock := &repomock.RegionRepo{
		CountZonesFunc: func(_ context.Context, _ string) (int, error) { return 2, nil },
		DeleteFunc: func(_ context.Context, _ string) error {
			t.Fatal("Delete must not be called when zones exist")
			return nil
		},
	}
	uc := region.New(mock)
	err := uc.Delete(context.Background(), "ru-central1")
	if !stderrors.Is(err, geoerrors.ErrFailedPrecondition) {
		t.Fatalf("Delete err = %v, want ErrFailedPrecondition", err)
	}
}

func TestDelete_noZones_ok(t *testing.T) {
	deleted := false
	mock := &repomock.RegionRepo{
		CountZonesFunc: func(_ context.Context, _ string) (int, error) { return 0, nil },
		DeleteFunc:     func(_ context.Context, _ string) error { deleted = true; return nil },
	}
	uc := region.New(mock)
	if err := uc.Delete(context.Background(), "eu-west1"); err != nil {
		t.Fatalf("Delete err = %v", err)
	}
	if !deleted {
		t.Fatal("Delete repo not called")
	}
}

func TestList_garbagePageSize_invalidArg(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{})
	_, _, err := uc.List(context.Background(), region.Pagination{PageSize: 1_000_000})
	if err == nil {
		t.Fatal("List(page_size too large) err = nil, want validation error")
	}
}
