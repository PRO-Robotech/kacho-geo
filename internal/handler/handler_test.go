// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

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
			return nil, geoerrors.Wrap(stderrors.New("x"), "Region", id) // путь ErrInternal
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
			return &domain.Region{ID: id, Name: "Region 1"}, nil
		},
	})
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
	})
	h := handler.NewZoneHandler(uc)
	resp, err := h.Get(context.Background(), &geov1.GetZoneRequest{ZoneId: "region-1-a"})
	if err != nil {
		t.Fatalf("Get err = %v", err)
	}
	if resp.GetRegionId() != "region-1" || resp.GetStatus() != geov1.Zone_UP {
		t.Fatalf("Get resp = %+v", resp)
	}
}

func TestInternalRegionHandler_Delete_failedPrecondition(t *testing.T) {
	// repo.Delete возвращает ErrFailedPrecondition (FK RESTRICT zones→regions,
	// SQLSTATE 23503) → handler маппит в FAILED_PRECONDITION.
	uc := region.New(&repomock.RegionRepo{
		DeleteFunc: func(_ context.Context, _ string) error { return geoerrors.ErrFailedPrecondition },
	})
	h := handler.NewInternalRegionHandler(uc)
	_, err := h.Delete(context.Background(), &geov1.DeleteRegionRequest{RegionId: "region-1"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("want FAILED_PRECONDITION (зоны существуют), got %v", err)
	}
}

func TestInternalZoneHandler_Create_happy(t *testing.T) {
	uc := zone.New(&repomock.ZoneRepo{
		InsertFunc: func(_ context.Context, z *domain.Zone) (*domain.Zone, error) { return z, nil },
	})
	h := handler.NewInternalZoneHandler(uc)
	resp, err := h.Create(context.Background(), &geov1.CreateZoneRequest{
		Id: "region-1-a", RegionId: "region-1", Status: geov1.Zone_UP, Name: "Region 1 A",
	})
	if err != nil {
		t.Fatalf("Create err = %v", err)
	}
	if resp.GetId() != "region-1-a" || resp.GetStatus() != geov1.Zone_UP {
		t.Fatalf("Create resp = %+v", resp)
	}
}
