package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRO-Robotech/kacho-corelib/outbox"
	"github.com/PRO-Robotech/kacho-corelib/validate"

	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
)

// ZoneRepo is the pgx-backed implementation of zone.Repo.
type ZoneRepo struct {
	pool *pgxpool.Pool
}

// NewZoneRepo constructs a ZoneRepo over a pgxpool.
func NewZoneRepo(pool *pgxpool.Pool) *ZoneRepo { return &ZoneRepo{pool: pool} }

// Get returns a zone by id.
func (r *ZoneRepo) Get(ctx context.Context, id string) (*domain.Zone, error) {
	var z domain.Zone
	var statusName string
	err := r.pool.QueryRow(ctx, `SELECT id, region_id, name, status, created_at FROM zones WHERE id = $1`, id).
		Scan(&z.ID, &z.RegionID, &z.Name, &statusName, &z.CreatedAt)
	if err != nil {
		return nil, geoerrors.Wrap(err, "Zone", id)
	}
	z.Status = zoneStatusFromName(statusName)
	return &z, nil
}

// List returns zones with cursor pagination by id.
func (r *ZoneRepo) List(ctx context.Context, p zone.Pagination) ([]*domain.Zone, string, error) {
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
	q := fmt.Sprintf(`SELECT id, region_id, name, status, created_at FROM zones %s ORDER BY id ASC LIMIT $%d`, where, len(args)+1)
	args = append(args, pageSize+1)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, "", geoerrors.Wrap(err, "Zone", "")
	}
	defer rows.Close()
	var out []*domain.Zone
	for rows.Next() {
		var z domain.Zone
		var statusName string
		if err := rows.Scan(&z.ID, &z.RegionID, &z.Name, &statusName, &z.CreatedAt); err != nil {
			return nil, "", geoerrors.Wrap(err, "Zone", "")
		}
		z.Status = zoneStatusFromName(statusName)
		out = append(out, &z)
	}
	if err := rows.Err(); err != nil {
		return nil, "", geoerrors.Wrap(err, "Zone", "")
	}
	var next string
	if int64(len(out)) > pageSize {
		last := out[pageSize-1]
		next = encodePageToken(last.CreatedAt, last.ID)
		out = out[:pageSize]
	}
	return out, next, nil
}

// Insert creates a zone (admin-only) + emits geo_outbox CREATED in the same tx.
// A non-existent region_id surfaces as FK violation (23503) → FailedPrecondition.
func (r *ZoneRepo) Insert(ctx context.Context, z *domain.Zone) (*domain.Zone, error) {
	var created domain.Zone
	var statusName string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		serr := tx.QueryRow(ctx,
			`INSERT INTO zones (id, region_id, name, status, created_at) VALUES ($1,$2,$3,$4,$5) RETURNING id, region_id, name, status, created_at`,
			z.ID, z.RegionID, z.Name, zoneStatusName(z.Status), time.Now().UTC()).
			Scan(&created.ID, &created.RegionID, &created.Name, &statusName, &created.CreatedAt)
		if serr != nil {
			return geoerrors.Wrap(serr, "Zone", z.ID)
		}
		created.Status = zoneStatusFromName(statusName)
		return outbox.Emit(ctx, tx, outboxTable, "Zone", created.ID, "CREATED", map[string]any{
			"id":        created.ID,
			"region_id": created.RegionID,
			"name":      created.Name,
			"status":    statusName,
		})
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// Update mutates a zone (admin-only) + emits geo_outbox UPDATED.
func (r *ZoneRepo) Update(ctx context.Context, z *domain.Zone) (*domain.Zone, error) {
	var updated domain.Zone
	var statusName string
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		serr := tx.QueryRow(ctx,
			`UPDATE zones SET region_id=$2, name=$3, status=$4 WHERE id=$1 RETURNING id, region_id, name, status, created_at`,
			z.ID, z.RegionID, z.Name, zoneStatusName(z.Status)).
			Scan(&updated.ID, &updated.RegionID, &updated.Name, &statusName, &updated.CreatedAt)
		if serr != nil {
			return geoerrors.Wrap(serr, "Zone", z.ID)
		}
		updated.Status = zoneStatusFromName(statusName)
		return outbox.Emit(ctx, tx, outboxTable, "Zone", updated.ID, "UPDATED", map[string]any{
			"id":        updated.ID,
			"region_id": updated.RegionID,
			"name":      updated.Name,
			"status":    statusName,
		})
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

// Delete removes a zone (admin-only) + emits geo_outbox DELETED.
func (r *ZoneRepo) Delete(ctx context.Context, id string) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM zones WHERE id = $1`, id)
		if err != nil {
			return geoerrors.Wrap(err, "Zone", id)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("%w: Zone %s not found", geoerrors.ErrNotFound, id)
		}
		return outbox.Emit(ctx, tx, outboxTable, "Zone", id, "DELETED", map[string]any{"id": id})
	})
}

func zoneStatusName(s domain.ZoneStatus) string {
	switch s {
	case domain.ZoneStatusUp:
		return "UP"
	case domain.ZoneStatusDown:
		return "DOWN"
	default:
		return "STATUS_UNSPECIFIED"
	}
}

func zoneStatusFromName(s string) domain.ZoneStatus {
	switch s {
	case "UP":
		return domain.ZoneStatusUp
	case "DOWN":
		return domain.ZoneStatusDown
	default:
		return domain.ZoneStatusUnspecified
	}
}

var _ zone.Repo = (*ZoneRepo)(nil)
