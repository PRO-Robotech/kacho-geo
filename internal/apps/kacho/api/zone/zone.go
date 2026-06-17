// Package zone — use-case (business logic) for the Zone catalog.
//
// Clean-arch use-case layer: imports domain + the ZoneRepo port; never imports
// pgx/grpc-stubs/transport. Public ZoneService.Get/List are read-only (sync);
// admin CRUD is driven by InternalZoneService on :9091 and returns the resource
// synchronously (catalog pattern — see acceptance S2).
package zone

import (
	"context"
	"fmt"

	"github.com/PRO-Robotech/kacho-corelib/validate"

	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// Pagination — cursor-based List input (page_size + opaque page_token).
type Pagination struct {
	PageSize  int64
	PageToken string
}

// Repo — port-interface for the zones table (read + admin CRUD). Implemented by
// internal/repo/kacho/pg.ZoneRepo; mocked by repomock for unit tests.
type Repo interface {
	Get(ctx context.Context, id string) (*domain.Zone, error)
	List(ctx context.Context, p Pagination) ([]*domain.Zone, string, error)
	Insert(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	Update(ctx context.Context, z *domain.Zone) (*domain.Zone, error)
	Delete(ctx context.Context, id string) error
}

// UseCase — Zone business logic over a Repo port.
type UseCase struct {
	repo Repo
}

// New constructs a Zone UseCase.
func New(repo Repo) *UseCase { return &UseCase{repo: repo} }

// Get returns a Zone by id.
func (u *UseCase) Get(ctx context.Context, id string) (*domain.Zone, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	z, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return z, nil
}

// List returns zones (cursor pagination by id).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Zone, string, error) {
	if _, err := validate.PageSize("page_size", p.PageSize); err != nil {
		return nil, "", err
	}
	return u.repo.List(ctx, p)
}

// Create inserts a Zone (admin-only). id/region_id are admin-assigned; region_id
// must reference an existing Region (DB FK enforces this → FailedPrecondition).
func (u *UseCase) Create(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*domain.Zone, error) {
	z := domain.Zone{ID: id, RegionID: regionID, Name: name, Status: st}
	if err := z.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	created, err := u.repo.Insert(ctx, &z)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update mutates a Zone (admin-only). id is immutable.
func (u *UseCase) Update(ctx context.Context, id, regionID, name string, st domain.ZoneStatus) (*domain.Zone, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	z, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if regionID != "" {
		z.RegionID = regionID
	}
	if name != "" {
		z.Name = name
	}
	z.Status = st
	updated, err := u.repo.Update(ctx, z)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a Zone (admin-only).
func (u *UseCase) Delete(ctx context.Context, id string) error {
	if id == "" {
		return geoerrors.ErrInvalidArg
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}
