// Package region — use-case (business logic) for the Region catalog.
//
// Clean-arch use-case layer: imports domain + the RegionRepo port; never imports
// pgx/grpc-stubs/transport. Public RegionService.Get/List are read-only (sync);
// admin CRUD (Create/Update/Delete) is driven by InternalRegionService on :9091
// and returns the resource synchronously (catalog pattern — see acceptance S2).
package region

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

// Repo — port-interface for the regions table (read + admin CRUD). Implemented
// by internal/repo/kacho/pg.RegionRepo; mocked by repomock for unit tests.
type Repo interface {
	Get(ctx context.Context, id string) (*domain.Region, error)
	List(ctx context.Context, p Pagination) ([]*domain.Region, string, error)
	Insert(ctx context.Context, r *domain.Region) (*domain.Region, error)
	Update(ctx context.Context, r *domain.Region) (*domain.Region, error)
	Delete(ctx context.Context, id string) error
	// CountZones — number of zones referencing this region (delete-RESTRICT precheck).
	CountZones(ctx context.Context, regionID string) (int, error)
}

// UseCase — Region business logic over a Repo port.
type UseCase struct {
	repo Repo
}

// New constructs a Region UseCase.
func New(repo Repo) *UseCase { return &UseCase{repo: repo} }

// Get returns a Region by id.
func (u *UseCase) Get(ctx context.Context, id string) (*domain.Region, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	r, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// List returns regions (cursor pagination by id).
func (u *UseCase) List(ctx context.Context, p Pagination) ([]*domain.Region, string, error) {
	if _, err := validate.PageSize("page_size", p.PageSize); err != nil {
		return nil, "", err
	}
	return u.repo.List(ctx, p)
}

// Create inserts a Region (admin-only). id is admin-assigned and immutable.
func (u *UseCase) Create(ctx context.Context, id, name string) (*domain.Region, error) {
	r := domain.Region{ID: id, Name: name}
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", geoerrors.ErrInvalidArg, err.Error())
	}
	created, err := u.repo.Insert(ctx, &r)
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update mutates a Region's name (admin-only). id is immutable.
func (u *UseCase) Update(ctx context.Context, id, name string) (*domain.Region, error) {
	if id == "" {
		return nil, geoerrors.ErrInvalidArg
	}
	r, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != "" {
		r.Name = name
	}
	updated, err := u.repo.Update(ctx, r)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Delete removes a Region (admin-only). Blocked while zones reference it
// (FailedPrecondition; DB-level FK RESTRICT zones→regions is the source of truth).
func (u *UseCase) Delete(ctx context.Context, id string) error {
	if id == "" {
		return geoerrors.ErrInvalidArg
	}
	if n, err := u.repo.CountZones(ctx, id); err == nil && n > 0 {
		return fmt.Errorf("%w: region %s has %d zone(s); delete the zones first", geoerrors.ErrFailedPrecondition, id, n)
	}
	if err := u.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}
