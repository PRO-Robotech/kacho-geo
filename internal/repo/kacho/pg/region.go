// Package pg — Postgres adapter (handwritten pgx) for the kacho-geo regions /
// zones catalog. Implements the region.Repo / zone.Repo ports. Admin mutations
// (Insert/Update/Delete) emit a geo_outbox audit row atomically in the same
// writer-tx (ban #10 — no software check-then-act, audit cannot be lost).
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRO-Robotech/kacho-corelib/outbox"
	"github.com/PRO-Robotech/kacho-corelib/validate"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// outboxTable is the audit-outbox table for kacho-geo admin mutations
// (<domain>_outbox convention, parity with compute_outbox / vpc_outbox).
const outboxTable = "geo_outbox"

// RegionRepo is the pgx-backed implementation of region.Repo.
type RegionRepo struct {
	pool *pgxpool.Pool
}

// NewRegionRepo constructs a RegionRepo over a pgxpool.
func NewRegionRepo(pool *pgxpool.Pool) *RegionRepo { return &RegionRepo{pool: pool} }

// Get returns a region by id.
func (r *RegionRepo) Get(ctx context.Context, id string) (*domain.Region, error) {
	var rg domain.Region
	err := r.pool.QueryRow(ctx, `SELECT id, name, created_at FROM regions WHERE id = $1`, id).
		Scan(&rg.ID, &rg.Name, &rg.CreatedAt)
	if err != nil {
		return nil, geoerrors.Wrap(err, "Region", id)
	}
	return &rg, nil
}

// List returns regions with cursor pagination by id.
func (r *RegionRepo) List(ctx context.Context, p region.Pagination) ([]*domain.Region, string, error) {
	pageSize, err := validate.PageSize("page_size", p.PageSize)
	if err != nil {
		return nil, "", err
	}
	args := []any{}
	where := ""
	if p.PageToken != "" {
		_, cursorID, derr := decodePageToken(p.PageToken)
		if derr != nil {
			return nil, "", invalidPageTokenErr(derr)
		}
		where = "WHERE id > $1"
		args = append(args, cursorID)
	}
	q := fmt.Sprintf(`SELECT id, name, created_at FROM regions %s ORDER BY id ASC LIMIT $%d`, where, len(args)+1)
	args = append(args, pageSize+1)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", geoerrors.Wrap(err, "Region", "")
	}
	defer rows.Close()
	var out []*domain.Region
	for rows.Next() {
		var rg domain.Region
		if err := rows.Scan(&rg.ID, &rg.Name, &rg.CreatedAt); err != nil {
			return nil, "", geoerrors.Wrap(err, "Region", "")
		}
		out = append(out, &rg)
	}
	if err := rows.Err(); err != nil {
		return nil, "", geoerrors.Wrap(err, "Region", "")
	}
	var next string
	if int64(len(out)) > pageSize {
		last := out[pageSize-1]
		next = encodePageToken(last.CreatedAt, last.ID)
		out = out[:pageSize]
	}
	return out, next, nil
}

// Insert creates a region (admin-only) and emits a geo_outbox CREATED row in the
// same tx (atomic; ban #10).
func (r *RegionRepo) Insert(ctx context.Context, rg *domain.Region) (*domain.Region, error) {
	var created domain.Region
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		serr := tx.QueryRow(ctx,
			`INSERT INTO regions (id, name, created_at) VALUES ($1,$2,$3) RETURNING id, name, created_at`,
			rg.ID, rg.Name, time.Now().UTC()).
			Scan(&created.ID, &created.Name, &created.CreatedAt)
		if serr != nil {
			return geoerrors.Wrap(serr, "Region", rg.ID)
		}
		return outbox.Emit(ctx, tx, outboxTable, "Region", created.ID, "CREATED", map[string]any{
			"id":   created.ID,
			"name": created.Name,
		})
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// Update mutates a region's name (admin-only) + emits geo_outbox UPDATED.
func (r *RegionRepo) Update(ctx context.Context, rg *domain.Region) (*domain.Region, error) {
	var updated domain.Region
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		serr := tx.QueryRow(ctx,
			`UPDATE regions SET name=$2 WHERE id=$1 RETURNING id, name, created_at`,
			rg.ID, rg.Name).
			Scan(&updated.ID, &updated.Name, &updated.CreatedAt)
		if serr != nil {
			return geoerrors.Wrap(serr, "Region", rg.ID)
		}
		return outbox.Emit(ctx, tx, outboxTable, "Region", updated.ID, "UPDATED", map[string]any{
			"id":   updated.ID,
			"name": updated.Name,
		})
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete removes a region (admin-only) + emits geo_outbox DELETED. FK RESTRICT
// (zones→regions) surfaces as SQLSTATE 23503 → ErrFailedPrecondition.
func (r *RegionRepo) Delete(ctx context.Context, id string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM regions WHERE id = $1`, id)
		if err != nil {
			return geoerrors.Wrap(err, "Region", id)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: Region %s not found", geoerrors.ErrNotFound, id)
		}
		return outbox.Emit(ctx, tx, outboxTable, "Region", id, "DELETED", map[string]any{"id": id})
	})
}

// CountZones returns the number of zones referencing this region.
func (r *RegionRepo) CountZones(ctx context.Context, regionID string) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM zones WHERE region_id = $1`, regionID).Scan(&n); err != nil {
		return 0, geoerrors.Wrap(err, "Region", regionID)
	}
	return n, nil
}

var _ region.Repo = (*RegionRepo)(nil)
