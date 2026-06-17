// Package config — kacho-geo configuration, loaded from environment variables
// via corelib config.LoadPrefixed("KACHO_GEO"). Absolute-tagged fields resolve
// verbatim; nested per-edge TLS value-structs (grpcclient.TLSClient /
// grpcsrv.TLSServer) get independent KACHO_GEO_<EDGE>_<NAME> names (per-edge
// prefixing — no process-wide TLS singleton).
package config

import (
	"fmt"
	"os"

	"google.golang.org/grpc"

	corecfg "github.com/PRO-Robotech/kacho-corelib/config"
	"github.com/PRO-Robotech/kacho-corelib/grpcclient"
	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"
)

// envPrefix — root segment for kacho-geo env names (KACHO_<DOMAIN>).
const envPrefix = "KACHO_GEO"

// Config — kacho-geo configuration.
type Config struct {
	DBHost     string `envconfig:"KACHO_GEO_DB_HOST" default:"localhost"`
	DBPort     string `envconfig:"KACHO_GEO_DB_PORT" default:"5432"`
	DBUser     string `envconfig:"KACHO_GEO_DB_USER" default:"geo"`
	DBPassword string `envconfig:"KACHO_GEO_DB_PASSWORD" required:"true"`
	DBName     string `envconfig:"KACHO_GEO_DB_NAME" default:"kacho_geo"`
	// DBSSLMode — sslmode for the DSN. dev default `disable`; production must set
	// require|verify-ca|verify-full.
	DBSSLMode string `envconfig:"KACHO_GEO_DB_SSLMODE" default:"disable"`
	// DBMaxConns — pgx pool limit (0 = pgx default max(4, NumCPU)).
	DBMaxConns int `envconfig:"KACHO_GEO_DB_MAX_CONNS" default:"0"`

	// GrpcPort — public read-only listener (RegionService/ZoneService).
	GrpcPort string `envconfig:"KACHO_GEO_GRPC_PORT" default:"9090"`
	// InternalGrpcPort — cluster-internal listener (InternalRegion/ZoneService).
	// NOT exposed on the api-gateway external endpoint (ban #6).
	InternalGrpcPort string `envconfig:"KACHO_GEO_INTERNAL_PORT" default:"9091"`

	// AuthMode — fail-closed gate: dev | production | production-strict.
	AuthMode string `envconfig:"KACHO_GEO_AUTH_MODE" default:"dev"`

	// AuthZIAMGRPCAddr — kacho-iam internal endpoint for per-RPC Check
	// (geo→iam authz edge). Empty + Breakglass=false → interceptor NOT attached
	// (graceful dev start without kacho-iam). Typically the iam-internal :9091.
	AuthZIAMGRPCAddr string `envconfig:"KACHO_GEO_AUTHZ_IAM_GRPC_ADDR" default:""`
	// AuthZBreakglass — emergency mode: pass all RPCs without Check + WARN
	// (dev / break-glass only).
	AuthZBreakglass bool `envconfig:"KACHO_GEO_AUTHZ_BREAKGLASS" default:"false"`

	// ===== per-edge mTLS (corelib SEC-B) =====

	// IAMAuthzMTLS — client-creds for the geo→iam Check edge (:9091).
	IAMAuthzMTLS grpcclient.TLSClient `envconfig:"IAM_AUTHZ_MTLS"`

	// PublicServerMTLS — server-creds for the public listener (:9090).
	PublicServerMTLS grpcsrv.TLSServer `envconfig:"PUBLIC_SERVER_MTLS"`

	// InternalServerMTLS — server-creds for the cluster-internal listener (:9091).
	InternalServerMTLS grpcsrv.TLSServer `envconfig:"INTERNAL_SERVER_MTLS"`
}

// IAMAuthzClientCreds returns the grpc.DialOption for the geo→iam Check edge.
// Enable=false → insecure (dev); Enable=true without a valid cert-trio → error
// (fail-closed, no silent insecure fallback).
func (c Config) IAMAuthzClientCreds() (grpc.DialOption, error) {
	return grpcclient.TLSClientCreds(c.IAMAuthzMTLS)
}

// PublicServerCreds returns the grpc.ServerOption for the public listener (:9090).
func (c Config) PublicServerCreds() (grpc.ServerOption, error) {
	return grpcsrv.TLSServerCreds(c.PublicServerMTLS)
}

// InternalServerCreds returns the grpc.ServerOption for the internal listener (:9091).
func (c Config) InternalServerCreds() (grpc.ServerOption, error) {
	return grpcsrv.TLSServerCreds(c.InternalServerMTLS)
}

// schemaOptionsParam — URL-encoded libpq `options=-c search_path=kacho_geo,public`.
// Every connection (pgxpool + goose database/sql) resolves the kacho_geo schema
// without per-statement SET search_path.
const schemaOptionsParam = "options=-c%20search_path%3Dkacho_geo%2Cpublic"

// baseDSN — standard postgres DSN (works for both pgxpool and database/sql),
// carrying the kacho_geo search_path via libpq options.
func (c Config) baseDSN() string {
	mode := c.DBSSLMode
	if mode == "" {
		mode = "disable"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, mode, schemaOptionsParam,
	)
}

// DSN — pgxpool connection string (supports pool_max_conns). NOT for
// database/sql (pool_max_conns → unknown PG param → FATAL).
func (c Config) DSN() string {
	dsn := c.baseDSN()
	if c.DBMaxConns > 0 {
		dsn += fmt.Sprintf("&pool_max_conns=%d", c.DBMaxConns)
	}
	return dsn
}

// MigrateDSN — connection string for goose/database/sql (no pgxpool params).
func (c Config) MigrateDSN() string {
	return c.baseDSN()
}

// Load loads the configuration from environment variables.
func Load() (Config, error) {
	var c Config
	err := corecfg.LoadPrefixed(envPrefix, &c)
	return c, err
}

// LoadInto — test helper: sets the given env vars for the duration of the call
// and loads via the same LoadPrefixed path as Load (restores prior env after).
func LoadInto(c *Config, env map[string]string) error {
	saved := make(map[string]*string, len(env))
	for k, v := range env {
		if prev, ok := os.LookupEnv(k); ok {
			saved[k] = &prev
		} else {
			saved[k] = nil
		}
		_ = os.Setenv(k, v)
	}
	defer func() {
		for k, prev := range saved {
			if prev == nil {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, *prev)
			}
		}
	}()
	return corecfg.LoadPrefixed(envPrefix, c)
}
