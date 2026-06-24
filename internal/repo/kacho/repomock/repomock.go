// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package repomock — in-memory моки портов region/zone Repo для unit-тестов
// use-case (без Postgres; иначе adapter протек бы в use-case).
package repomock

import (
	"context"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

// RegionRepo — мок region.Repo на функциях-полях.
type RegionRepo struct {
	GetFunc    func(ctx context.Context, id string) (*domain.Region, error)
	ListFunc   func(ctx context.Context, p region.Pagination) ([]*domain.Region, string, error)
	InsertFunc func(ctx context.Context, r *domain.Region) (*domain.Region, error)
	UpdateFunc func(ctx context.Context, id string, name *string) (*domain.Region, error)
	DeleteFunc func(ctx context.Context, id string) error
}

// Get реализует region.Repo.
func (m *RegionRepo) Get(ctx context.Context, id string) (*domain.Region, error) {
	return m.GetFunc(ctx, id)
}

// List реализует region.Repo.
func (m *RegionRepo) List(ctx context.Context, p region.Pagination) ([]*domain.Region, string, error) {
	return m.ListFunc(ctx, p)
}

// Insert реализует region.Repo.
func (m *RegionRepo) Insert(ctx context.Context, r *domain.Region) (*domain.Region, error) {
	return m.InsertFunc(ctx, r)
}

// Update реализует region.Repo.
func (m *RegionRepo) Update(ctx context.Context, id string, name *string) (*domain.Region, error) {
	return m.UpdateFunc(ctx, id, name)
}

// Delete реализует region.Repo.
func (m *RegionRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFunc(ctx, id)
}

var _ region.Repo = (*RegionRepo)(nil)

// ZoneRepo — мок zone.Repo на функциях-полях.
type ZoneRepo struct {
	GetFunc    func(ctx context.Context, id string) (*domain.Zone, error)
	ListFunc   func(ctx context.Context, p zone.Pagination) ([]*domain.Zone, string, error)
	InsertFunc func(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	UpdateFunc func(ctx context.Context, id string, p zone.UpdateParams) (*domain.Zone, error)
	DeleteFunc func(ctx context.Context, id string) error
}

// Get реализует zone.Repo.
func (m *ZoneRepo) Get(ctx context.Context, id string) (*domain.Zone, error) {
	return m.GetFunc(ctx, id)
}

// List реализует zone.Repo.
func (m *ZoneRepo) List(ctx context.Context, p zone.Pagination) ([]*domain.Zone, string, error) {
	return m.ListFunc(ctx, p)
}

// Insert реализует zone.Repo.
func (m *ZoneRepo) Insert(ctx context.Context, z *domain.Zone) (*domain.Zone, error) {
	return m.InsertFunc(ctx, z)
}

// Update реализует zone.Repo.
func (m *ZoneRepo) Update(ctx context.Context, id string, p zone.UpdateParams) (*domain.Zone, error) {
	return m.UpdateFunc(ctx, id, p)
}

// Delete реализует zone.Repo.
func (m *ZoneRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFunc(ctx, id)
}

var _ zone.Repo = (*ZoneRepo)(nil)
