// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package handler — internal.go: admin-CRUD над каталогом Region/Zone
// (InternalRegionService / InternalZoneService). Регистрируется ТОЛЬКО на
// cluster-internal листенере (:9091), проброшен через internal mux api-gateway
// на /geo/v1/regions, /geo/v1/zones — НИКОГДА не на внешнем TLS endpoint
// (только cluster-internal).
//
// Catalog-паттерн (осознанное отклонение): эти admin-мутации возвращают ресурс
// СИНХРОННО (не operation.Operation) — Region/Zone это admin-managed записи
// reference-каталога с admin-assigned immutable id.
package handler

import (
	"context"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

// InternalRegionHandler реализует geov1.InternalRegionServiceServer (admin CRUD).
type InternalRegionHandler struct {
	geov1.UnimplementedInternalRegionServiceServer
	uc *region.UseCase
}

// NewInternalRegionHandler конструирует InternalRegionHandler.
func NewInternalRegionHandler(uc *region.UseCase) *InternalRegionHandler {
	return &InternalRegionHandler{uc: uc}
}

// Create создает region (возвращает ресурс синхронно).
func (h *InternalRegionHandler) Create(ctx context.Context, req *geov1.CreateRegionRequest) (*geov1.Region, error) {
	r, err := h.uc.Create(ctx, req.GetId(), req.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoRegion(r), nil
}

// Update обновляет имя region.
func (h *InternalRegionHandler) Update(ctx context.Context, req *geov1.UpdateRegionRequest) (*geov1.Region, error) {
	r, err := h.uc.Update(ctx, req.GetRegionId(), req.GetName())
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoRegion(r), nil
}

// Delete удаляет region (блокируется FK RESTRICT, если на него еще ссылаются зоны).
func (h *InternalRegionHandler) Delete(ctx context.Context, req *geov1.DeleteRegionRequest) (*geov1.DeleteRegionResponse, error) {
	if err := h.uc.Delete(ctx, req.GetRegionId()); err != nil {
		return nil, mapErr(err)
	}
	return &geov1.DeleteRegionResponse{}, nil
}

// InternalZoneHandler реализует geov1.InternalZoneServiceServer (admin CRUD).
type InternalZoneHandler struct {
	geov1.UnimplementedInternalZoneServiceServer
	uc *zone.UseCase
}

// NewInternalZoneHandler конструирует InternalZoneHandler.
func NewInternalZoneHandler(uc *zone.UseCase) *InternalZoneHandler {
	return &InternalZoneHandler{uc: uc}
}

// Create создает zone (возвращает ресурс синхронно).
func (h *InternalZoneHandler) Create(ctx context.Context, req *geov1.CreateZoneRequest) (*geov1.Zone, error) {
	z, err := h.uc.Create(ctx, req.GetId(), req.GetRegionId(), req.GetName(), domain.ZoneStatus(req.GetStatus()))
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoZone(z), nil
}

// Update обновляет zone.
func (h *InternalZoneHandler) Update(ctx context.Context, req *geov1.UpdateZoneRequest) (*geov1.Zone, error) {
	z, err := h.uc.Update(ctx, req.GetZoneId(), req.GetRegionId(), req.GetName(), domain.ZoneStatus(req.GetStatus()))
	if err != nil {
		return nil, mapErr(err)
	}
	return toProtoZone(z), nil
}

// Delete удаляет zone.
func (h *InternalZoneHandler) Delete(ctx context.Context, req *geov1.DeleteZoneRequest) (*geov1.DeleteZoneResponse, error) {
	if err := h.uc.Delete(ctx, req.GetZoneId()); err != nil {
		return nil, mapErr(err)
	}
	return &geov1.DeleteZoneResponse{}, nil
}
