package pg_test

import (
	"context"
	"database/sql"
	stderrors "errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	coredb "github.com/PRO-Robotech/kacho-corelib/db"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/domain"
	geoerrors "github.com/PRO-Robotech/kacho-geo/internal/errors"
	"github.com/PRO-Robotech/kacho-geo/internal/migrations"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/pg"
)

// newTestPool spins up a Postgres 16 container, runs the kacho-geo migrations,
// and returns a search_path=kacho_geo pgxpool. Skipped under -short.
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test (testcontainers Postgres) — skipped with -short")
	}
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("kacho_geo"),
		tcpostgres.WithUsername("geo"),
		tcpostgres.WithPassword("secret"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	baseDSN, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Migrate via goose (database/sql).
	sqlDB, err := sql.Open("pgx", baseDSN)
	require.NoError(t, err)
	defer sqlDB.Close()
	goose.SetBaseFS(migrations.FS)
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.Up(sqlDB, "."))

	// pgxpool with search_path=kacho_geo so repo SQL resolves unqualified tables.
	// libpq runtime param form (URL query `search_path=` is not honoured by pgx).
	poolDSN := baseDSN + "&options=-c%20search_path%3Dkacho_geo%2Cpublic"
	pool, err := coredb.NewPool(ctx, poolDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestSeedRowsPresent(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)
	zr := pg.NewZoneRepo(pool)

	r, err := rr.Get(ctx, "ru-central1")
	require.NoError(t, err)
	require.Equal(t, "ru-central1", r.ID)
	require.Equal(t, "Russia Central 1", r.Name)

	for _, id := range []string{"ru-central1-a", "ru-central1-b", "ru-central1-d"} {
		z, gerr := zr.Get(ctx, id)
		require.NoError(t, gerr)
		require.Equal(t, "ru-central1", z.RegionID)
		require.Equal(t, domain.ZoneStatusUp, z.Status)
	}
}

func TestRegionGetNotFound(t *testing.T) {
	pool := newTestPool(t)
	rr := pg.NewRegionRepo(pool)
	_, err := rr.Get(context.Background(), "no-such-region")
	require.True(t, stderrors.Is(err, geoerrors.ErrNotFound), "got %v", err)
}

func TestRegionListSeed(t *testing.T) {
	pool := newTestPool(t)
	rr := pg.NewRegionRepo(pool)
	regions, _, err := rr.List(context.Background(), region.Pagination{PageSize: 50})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(regions), 1)
}

func TestZoneListByRegionSeed(t *testing.T) {
	pool := newTestPool(t)
	zr := pg.NewZoneRepo(pool)
	zones, _, err := zr.List(context.Background(), zone.Pagination{PageSize: 50})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(zones), 3)
}

func TestRegionCRUDAndOutbox(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)

	created, err := rr.Insert(ctx, &domain.Region{ID: "eu-west1", Name: "EU West 1"})
	require.NoError(t, err)
	require.Equal(t, "eu-west1", created.ID)

	// outbox CREATED row emitted atomically.
	require.Equal(t, 1, outboxCount(t, pool, "Region", "eu-west1", "CREATED"))

	updated, err := rr.Update(ctx, &domain.Region{ID: "eu-west1", Name: "EU West One"})
	require.NoError(t, err)
	require.Equal(t, "EU West One", updated.Name)
	require.Equal(t, 1, outboxCount(t, pool, "Region", "eu-west1", "UPDATED"))

	require.NoError(t, rr.Delete(ctx, "eu-west1"))
	require.Equal(t, 1, outboxCount(t, pool, "Region", "eu-west1", "DELETED"))
}

func TestRegionDeleteAlreadyExists(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)
	_, err := rr.Insert(ctx, &domain.Region{ID: "ru-central1", Name: "dup"})
	require.True(t, stderrors.Is(err, geoerrors.ErrAlreadyExists), "got %v", err)
}

func TestZoneFKRestrict_DeleteRegionWithZones(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)

	// ru-central1 has seeded zones → CountZones > 0; raw Delete must hit FK RESTRICT.
	n, err := rr.CountZones(ctx, "ru-central1")
	require.NoError(t, err)
	require.Greater(t, n, 0)

	err = rr.Delete(ctx, "ru-central1")
	require.True(t, stderrors.Is(err, geoerrors.ErrFailedPrecondition), "FK RESTRICT must surface as FailedPrecondition, got %v", err)
}

func TestZoneCreateFKViolation_NoSuchRegion(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	zr := pg.NewZoneRepo(pool)
	_, err := zr.Insert(ctx, &domain.Zone{ID: "x-a", RegionID: "no-such-region", Status: domain.ZoneStatusUp})
	require.True(t, stderrors.Is(err, geoerrors.ErrFailedPrecondition), "FK violation must surface as FailedPrecondition, got %v", err)
}

func TestRegionDeleteThenZoneDelete_FKLifecycle(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)
	zr := pg.NewZoneRepo(pool)

	_, err := rr.Insert(ctx, &domain.Region{ID: "eu-west1", Name: "EU West 1"})
	require.NoError(t, err)
	_, err = zr.Insert(ctx, &domain.Zone{ID: "eu-west1-a", RegionID: "eu-west1", Status: domain.ZoneStatusUp, Name: "EU West 1 A"})
	require.NoError(t, err)

	// Region delete blocked while the zone exists.
	require.True(t, stderrors.Is(rr.Delete(ctx, "eu-west1"), geoerrors.ErrFailedPrecondition))

	// Delete the zone, then the region succeeds.
	require.NoError(t, zr.Delete(ctx, "eu-west1-a"))
	require.NoError(t, rr.Delete(ctx, "eu-west1"))
}

// TestConcurrentRegionInsert_OneWins — UNIQUE PK under concurrency: exactly one
// INSERT wins, the rest get ErrAlreadyExists (data-integrity.md checklist #5).
func TestConcurrentRegionInsert_OneWins(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	rr := pg.NewRegionRepo(pool)

	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	var mu sync.Mutex
	ok, dup := 0, 0
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := rr.Insert(ctx, &domain.Region{ID: "race-region", Name: "Race"})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case stderrors.Is(err, geoerrors.ErrAlreadyExists):
				dup++
			default:
				t.Errorf("unexpected err: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, ok, "exactly one INSERT must win")
	require.Equal(t, n-1, dup, "the rest must get ErrAlreadyExists")
}

func outboxCount(t *testing.T, pool *pgxpool.Pool, kind, id, eventType string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM geo_outbox WHERE resource_kind=$1 AND resource_id=$2 AND event_type=$3`,
		kind, id, eventType).Scan(&n)
	require.NoError(t, err)
	return n
}
