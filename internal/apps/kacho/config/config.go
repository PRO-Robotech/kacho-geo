// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

// Package config — конфигурация kacho-geo, загружается из переменных окружения
// через corelib config.LoadPrefixed("KACHO_GEO"). Поля с абсолютным тегом
// читаются как есть; вложенные per-edge TLS-структуры (grpcclient.TLSClient /
// grpcsrv.TLSServer) получают независимые имена KACHO_GEO_<EDGE>_<NAME> (префикс
// на каждое ребро — без общего на весь процесс TLS-синглтона).
package config

import (
	"fmt"
	"os"

	"google.golang.org/grpc"

	corecfg "github.com/PRO-Robotech/kacho-corelib/config"
	"github.com/PRO-Robotech/kacho-corelib/grpcclient"
	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"
)

// envPrefix — корневой сегмент env-имен kacho-geo (KACHO_<DOMAIN>).
const envPrefix = "KACHO_GEO"

// Config — конфигурация kacho-geo.
type Config struct {
	DBHost     string `envconfig:"KACHO_GEO_DB_HOST" default:"localhost"`
	DBPort     string `envconfig:"KACHO_GEO_DB_PORT" default:"5432"`
	DBUser     string `envconfig:"KACHO_GEO_DB_USER" default:"geo"`
	DBPassword string `envconfig:"KACHO_GEO_DB_PASSWORD" required:"true"`
	DBName     string `envconfig:"KACHO_GEO_DB_NAME" default:"kacho_geo"`
	// DBSSLMode — sslmode для DSN. dev по умолчанию `disable`; в проде обязателен
	// require|verify-ca|verify-full.
	DBSSLMode string `envconfig:"KACHO_GEO_DB_SSLMODE" default:"disable"`
	// DBMaxConns — лимит pgx-пула (0 = дефолт pgx max(4, NumCPU)).
	DBMaxConns int `envconfig:"KACHO_GEO_DB_MAX_CONNS" default:"0"`

	// GrpcPort — публичный read-only листенер (RegionService/ZoneService).
	GrpcPort string `envconfig:"KACHO_GEO_GRPC_PORT" default:"9090"`
	// InternalGrpcPort — cluster-internal листенер (InternalRegion/ZoneService).
	// Не выставляется на внешнем endpoint api-gateway — только cluster-internal.
	InternalGrpcPort string `envconfig:"KACHO_GEO_INTERNAL_PORT" default:"9091"`

	// AuthMode — fail-closed режим: dev | production | production-strict.
	AuthMode string `envconfig:"KACHO_GEO_AUTH_MODE" default:"dev"`

	// AuthZIAMGRPCAddr — internal endpoint kacho-iam для per-RPC Check
	// (ребро geo→iam authz). Пусто + Breakglass=false → интерсептор НЕ
	// подключается (грациозный dev-старт без kacho-iam). Обычно iam-internal :9091.
	AuthZIAMGRPCAddr string `envconfig:"KACHO_GEO_AUTHZ_IAM_GRPC_ADDR" default:""`
	// AuthZBreakglass — аварийный режим: пропускать все RPC без Check + WARN
	// (только dev / break-glass).
	AuthZBreakglass bool `envconfig:"KACHO_GEO_AUTHZ_BREAKGLASS" default:"false"`

	// ===== per-edge mTLS =====

	// IAMAuthzMTLS — client-creds для ребра geo→iam Check (:9091).
	IAMAuthzMTLS grpcclient.TLSClient `envconfig:"IAM_AUTHZ_MTLS"`

	// PublicServerMTLS — server-creds для публичного листенера (:9090).
	PublicServerMTLS grpcsrv.TLSServer `envconfig:"PUBLIC_SERVER_MTLS"`

	// InternalServerMTLS — server-creds для cluster-internal листенера (:9091).
	InternalServerMTLS grpcsrv.TLSServer `envconfig:"INTERNAL_SERVER_MTLS"`
}

// PublicServerCreds возвращает grpc.ServerOption для публичного листенера (:9090).
func (c Config) PublicServerCreds() (grpc.ServerOption, error) {
	return grpcsrv.TLSServerCreds(c.PublicServerMTLS)
}

// InternalServerCreds возвращает grpc.ServerOption для internal-листенера (:9091).
func (c Config) InternalServerCreds() (grpc.ServerOption, error) {
	return grpcsrv.TLSServerCreds(c.InternalServerMTLS)
}

// schemaOptionsParam — URL-encoded libpq `options=-c search_path=kacho_geo,public`.
// Каждое соединение (pgxpool + goose database/sql) видит схему kacho_geo без
// отдельного SET search_path на каждый стейтмент.
const schemaOptionsParam = "options=-c%20search_path%3Dkacho_geo%2Cpublic"

// baseDSN — стандартный postgres DSN (годится и для pgxpool, и для database/sql),
// несет search_path kacho_geo через libpq options.
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

// DSN — строка подключения для pgxpool (поддерживает pool_max_conns). НЕ для
// database/sql (pool_max_conns → неизвестный PG-параметр → FATAL).
func (c Config) DSN() string {
	dsn := c.baseDSN()
	if c.DBMaxConns > 0 {
		dsn += fmt.Sprintf("&pool_max_conns=%d", c.DBMaxConns)
	}
	return dsn
}

// MigrateDSN — строка подключения для goose/database/sql (без pgxpool-параметров).
func (c Config) MigrateDSN() string {
	return c.baseDSN()
}

// Load загружает конфигурацию из переменных окружения.
func Load() (Config, error) {
	var c Config
	err := corecfg.LoadPrefixed(envPrefix, &c)
	return c, err
}

// LoadInto — test-хелпер: выставляет переданные env-переменные на время вызова
// и грузит тем же путем LoadPrefixed, что и Load (по выходу восстанавливает env).
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
