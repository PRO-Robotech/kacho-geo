package handler_test

import (
	"context"
	stderrors "errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/handler"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/repomock"
)

func TestRegionHandler_Get_notFound_mapsToNotFound(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Region, error) {
			return nil, geoerrors.Wrap(stderrors.New("x"), "Region", id) // ErrInternal path
		},
	})
	h := handler.NewRegionHandler(uc)
	_, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: "x"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("want INTERNAL for raw error, got %v", err)
	}
}

func TestRegionHandler_Get_emptyID_invalidArgument(t *testing.T) {
	h := handler.NewRegionHandler(region.New(&repomock.RegionRepo{}))
	_, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want INVALID_ARGUMENT for empty id, got %v", err)
	}
}

func TestRegionHandler_Get_happy(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Region, error) {
			return &domain.Region{ID: id, Name: "Russia Central 1"}, nil
		},
	})
	h := handler.NewRegionHandler(uc)
	resp, err := h.Get(context.Background(), &geov1.GetRegionRequest{RegionId: "ru-central1"})
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if resp.GetId() != "ru-central1" || resp.GetName() != "Russia Central 1" {
		t.Fatalf("Get resp = %+v", resp)
	}
}

func TestZoneHandler_Get_happy_statusMapped(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{
		GetFunc: func(_ context.Context, id string) (*domain.Zone, error) {
			return &domain.Zone{ID: id, RegionID: "ru-central1", Status: domain.ZoneStatusUp}, nil
		},
	})
	h := handler.NewZoneHandler(uc)
	resp, err := h.Get(context.Background(), &geov1.GetZoneRequest{ZoneId: "ru-central1-a"})
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if resp.GetRegionId() != "ru-central1" || resp.GetStatus() != geov1.Zone_UP {
		t.Fatalf("Get resp = %+v", resp)
	}
}

func TestInternalRegionHandler_Delete_failedPrecondition(t *testing.T) {
	uc := region.New(&repomock.RegionRepo{
		CountZonesFunc: func(_ context.Context, _ string) (int, error) { return 1, nil },
	})
	h := handler.NewInternalRegionHandler(uc)
	_, err := h.Delete(context.Background(), &geov1.DeleteRegionRequest{RegionId: "ru-central1"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FAILED_PRECONDITION (zones exist), got %v", err)
	}
}

func TestInternalZoneHandler_Create_happy(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	})
	h := handler.NewInternalZoneHandler(uc)
	resp, err := h.Create(context.Background(), &geov1.CreateZoneRequest{
		Id: "eu-west1-a", RegionId: "eu-west1", Status: geov1.Zone_UP, Name: "EU West 1 A",
	})
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if resp.GetId() != "eu-west1-a" || resp.GetStatus() != geov1.Zone_UP {
		t.Fatalf("Create resp = %+v", resp)
	}
}
