// Package repomock — in-memory mocks of the region/zone Repo ports for use-case
// unit tests (no Postgres; adapter leakage into the use-case would require it).
package repomock

import (
	"context"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
)

// RegionRepo is a function-backed mock of region.Repo.
type RegionRepo struct {
	GetFunc        func(ctx context.Context, id string) (*domain.Region, error)
	ListFunc       func(ctx context.Context, p region.Pagination) ([]*domain.Region, string, error)
	InsertFunc     func(ctx context.Context, r *domain.Region) (*domain.Region, error)
	UpdateFunc     func(ctx context.Context, r *domain.Region) (*domain.Region, error)
	DeleteFunc     func(ctx context.Context, id string) error
	CountZonesFunc func(ctx context.Context, regionID string) (int, error)
}

// Get implements region.Repo.
func (m *RegionRepo) Get(ctx context.Context, id string) (*domain.Region, error) {
	return m.GetFunc(ctx, id)
}

// List implements region.Repo.
func (m *RegionRepo) List(ctx context.Context, p region.Pagination) ([]*domain.Region, string, error) {
	return m.ListFunc(ctx, p)
}

// Insert implements region.Repo.
func (m *RegionRepo) Insert(ctx context.Context, r *domain.Region) (*domain.Region, error) {
	return m.InsertFunc(ctx, r)
}

// Update implements region.Repo.
func (m *RegionRepo) Update(ctx context.Context, r *domain.Region) (*domain.Region, error) {
	return m.UpdateFunc(ctx, r)
}

// Delete implements region.Repo.
func (m *RegionRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFunc(ctx, id)
}

// CountZones implements region.Repo.
func (m *RegionRepo) CountZones(ctx context.Context, regionID string) (int, error) {
	return m.CountZonesFunc(ctx, regionID)
}

var _ region.Repo = (*RegionRepo)(nil)

// ZoneRepo is a function-backed mock of zone.Repo.
type ZoneRepo struct {
	GetFunc    func(ctx context.Context, id string) (*domain.Zone, error)
	ListFunc   func(ctx context.Context, p zone.Pagination) ([]*domain.Zone, string, error)
	InsertFunc func(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	UpdateFunc func(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	DeleteFunc func(ctx context.Context, id string) error
}

// Get implements zone.Repo.
func (m *ZoneRepo) Get(ctx context.Context, id string) (*domain.Zone, error) {
	return m.GetFunc(ctx, id)
}

// List implements zone.Repo.
func (m *ZoneRepo) List(ctx context.Context, p zone.Pagination) ([]*domain.Zone, string, error) {
	return m.ListFunc(ctx, p)
}

// Insert implements zone.Repo.
func (m *ZoneRepo) Insert(ctx context.Context, z *domain.Zone) (*domain.Zone, error) {
	return m.InsertFunc(ctx, z)
}

// Update implements zone.Repo.
func (m *ZoneRepo) Update(ctx context.Context, z *domain.Zone) (*domain.Zone, error) {
	return m.UpdateFunc(ctx, z)
}

// Delete implements zone.Repo.
func (m *ZoneRepo) Delete(ctx context.Context, id string) error {
	return m.DeleteFunc(ctx, id)
}

var _ zone.Repo = (*ZoneRepo)(nil)
