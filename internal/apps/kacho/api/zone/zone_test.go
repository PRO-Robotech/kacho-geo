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
			return &domain.Zone{ID: id, RegionID: "ru-central1", Status: domain.ZoneStatusUp}, nil
		},
	}
	uc := zone.New(mock)
	z, err := uc.Get(context.Background(), "ru-central1-a")
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if z.ID != "ru-central1-a" || z.RegionID != "ru-central1" || z.Status != domain.ZoneStatusUp {
		t.Fatalf("Get = %+v", z)
	}
}

func TestCreate_emptyRegionID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{})
	_, err := uc.Create(context.Background(), "ru-central1-a", "", "x", domain.ZoneStatusUp)
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Create(region_id='') err = %v, want ErrInvalidArg", err)
	}
}

func TestCreate_happy(t *testing.T) {
	mock := &repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	}
	uc := zone.New(mock)
	z, err := uc.Create(context.Background(), "eu-west1-a", "eu-west1", "EU West 1 A", domain.ZoneStatusUp)
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if z.ID != "eu-west1-a" || z.RegionID != "eu-west1" {
		t.Fatalf("Create = %+v", z)
	}
}

func TestUpdate_status(t *testing.T) {
	mock := &repomock.ZoneRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Zone, error) {
			return &domain.Zone{ID: id, RegionID: "ru-central1", Status: domain.ZoneStatusUp}, nil
		},
		UpdateFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	}
	uc := zone.New(mock)
	z, err := uc.Update(context.Background(), "ru-central1-a", "", "", domain.ZoneStatusDown)
	if err != nil {
		t.Fatalf("Update err = %v", err)
	}
	if z.Status != domain.ZoneStatusDown {
		t.Fatalf("Update status = %v, want DOWN", z.Status)
	}
}

func TestDelete_emptyID_invalidArg(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{})
	err := uc.Delete(context.Background(), "")
	if !stderrors.Is(err, geoerrors.ErrInvalidArg) {
		t.Fatalf("Delete('') err = %v, want ErrInvalidArg", err)
	}
}
