// Package handler — internal.go: admin-CRUD over the Region/Zone catalog
// (InternalRegionService / InternalZoneService). Registered ONLY on the
// cluster-internal listener (:9091), routed through the api-gateway internal mux
// onto /geo/v1/regions, /geo/v1/zones — NEVER on the external TLS endpoint
// (ban #6, security.md §Internal-vs-external).
//
// Catalog pattern (intentional, recorded scope-deviation): these admin mutations
// return the resource SYNCHRONOUSLY (not operation.Operation) — Region/Zone are
// admin-managed reference-catalog entries with admin-assigned immutable ids.
package handler

import (
	"context"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

// InternalRegionHandler implements geov1.InternalRegionServiceServer (admin CRUD).
type InternalRegionHandler struct {
	geov1.UnimplementedInternalRegionServiceServer
	uc *region.UseCase
}

// NewInternalRegionHandler constructs an InternalRegionHandler.
func NewInternalRegionHandler(uc *region.UseCase) *InternalRegionHandler {
	return &InternalRegionHandler{uc: uc}
}

// Create creates a region (returns the resource synchronously).
func (h *InternalRegionHandler) Create(ctx context.Context, req *geov1.CreateRegionRequest) (*geov1.Region, error) {
	r, err := h.uc.Create(ctx, req.GetId(), req.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoRegion(r), nil
}

// Update updates a region's name.
func (h *InternalRegionHandler) Update(ctx context.Context, req *geov1.UpdateRegionRequest) (*geov1.Region, error) {
	r, err := h.uc.Update(ctx, req.GetRegionId(), req.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoRegion(r), nil
}

// Delete deletes a region (blocked by FK RESTRICT if zones still reference it).
func (h *InternalRegionHandler) Delete(ctx context.Context, req *geov1.DeleteRegionRequest) (*geov1.DeleteRegionResponse, error) {
	if err := h.uc.Delete(ctx, req.GetRegionId()); err != nil {
		return nil, mapErr(err)
	}
	return &geov1.DeleteRegionResponse{}, nil
}

// InternalZoneHandler implements geov1.InternalZoneServiceServer (admin CRUD).
type InternalZoneHandler struct {
	geov1.UnimplementedInternalZoneServiceServer
	uc *zone.UseCase
}

// NewInternalZoneHandler constructs an InternalZoneHandler.
func NewInternalZoneHandler(uc *zone.UseCase) *InternalZoneHandler {
	return &InternalZoneHandler{uc: uc}
}

// Create creates a zone (returns the resource synchronously).
func (h *InternalZoneHandler) Create(ctx context.Context, req *geov1.CreateZoneRequest) (*geov1.Zone, error) {
	z, err := h.uc.Create(ctx, req.GetId(), req.GetRegionId(), req.GetName(), domain.ZoneStatus(req.GetStatus()))
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoZone(z), nil
}

// Update updates a zone.
func (h *InternalZoneHandler) Update(ctx context.Context, req *geov1.UpdateZoneRequest) (*geov1.Zone, error) {
	z, err := h.uc.Update(ctx, req.GetZoneId(), req.GetRegionId(), req.GetName(), domain.ZoneStatus(req.GetStatus()))
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoZone(z), nil
}

// Delete deletes a zone.
func (h *InternalZoneHandler) Delete(ctx context.Context, req *geov1.DeleteZoneRequest) (*geov1.DeleteZoneResponse, error) {
	if err := h.uc.Delete(ctx, req.GetZoneId()); err != nil {
		return nil, mapErr(err)
	}
	return &geov1.DeleteZoneResponse{}, nil
}
