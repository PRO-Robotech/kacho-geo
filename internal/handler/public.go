// Package handler — thin gRPC transport for kacho-geo (parse → use-case → format,
// no business logic). Public read-only handlers (RegionService/ZoneService
// Get/List) live here; admin CRUD lives in internal.go (InternalRegionService /
// InternalZoneService, registered only on the :9091 listener — ban #6).
package handler

import (
	"context"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
)

// RegionHandler implements geov1.RegionServiceServer (read-only public).
type RegionHandler struct {
	geov1.UnimplementedRegionServiceServer
	uc *region.UseCase
}

// NewRegionHandler constructs a RegionHandler.
func NewRegionHandler(uc *region.UseCase) *RegionHandler { return &RegionHandler{uc: uc} }

// Get returns a Region by id.
func (h *RegionHandler) Get(ctx context.Context, req *geov1.GetRegionRequest) (*geov1.Region, error) {
	r, err := h.uc.Get(ctx, req.GetRegionId())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoRegion(r), nil
}

// List returns regions (cursor pagination).
func (h *RegionHandler) List(ctx context.Context, req *geov1.ListRegionsRequest) (*geov1.ListRegionsResponse, error) {
	regions, next, err := h.uc.List(ctx, region.Pagination{PageSize: req.GetPageSize(), PageToken: req.GetPageToken()})
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &geov1.ListRegionsResponse{NextPageToken: next}
	for _, r := range regions {
		resp.Regions = append(resp.Regions, toProtoRegion(r))
	}
	return resp, nil
}

// ZoneHandler implements geov1.ZoneServiceServer (read-only public).
type ZoneHandler struct {
	geov1.UnimplementedZoneServiceServer
	uc *zone.UseCase
}

// NewZoneHandler constructs a ZoneHandler.
func NewZoneHandler(uc *zone.UseCase) *ZoneHandler { return &ZoneHandler{uc: uc} }

// Get returns a Zone by id.
func (h *ZoneHandler) Get(ctx context.Context, req *geov1.GetZoneRequest) (*geov1.Zone, error) {
	z, err := h.uc.Get(ctx, req.GetZoneId())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoZone(z), nil
}

// List returns zones (cursor pagination).
func (h *ZoneHandler) List(ctx context.Context, req *geov1.ListZonesRequest) (*geov1.ListZonesResponse, error) {
	zones, next, err := h.uc.List(ctx, zone.Pagination{PageSize: req.GetPageSize(), PageToken: req.GetPageToken()})
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &geov1.ListZonesResponse{NextPageToken: next}
	for _, z := range zones {
		resp.Zones = append(resp.Zones, toProtoZone(z))
	}
	return resp, nil
}
