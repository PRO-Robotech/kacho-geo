package config_test

import (
	"strings"
	"testing"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/config"
)

func TestLoad_defaults_and_dsn(t *testing.T) {
	var c config.Config
	if err := config.LoadInto(&c, map[string]string{
		"KACHO_GEO_DB_PASSWORD": "secret",
	}); err != nil {
		t.Fatalf("LoadInto err = %v", err)
	}
	if c.GrpcPort != "9090" || c.InternalGrpcPort != "9091" {
		t.Fatalf("ports = %s/%s, want 9090/9091", c.GrpcPort, c.InternalGrpcPort)
	}
	if c.DBName != "kacho_geo" {
		t.Fatalf("db name = %q, want kacho_geo", c.DBName)
	}
	if c.AuthMode != "dev" {
		t.Fatalf("auth mode = %q, want dev", c.AuthMode)
	}
	dsn := c.DSN()
	if !strings.Contains(dsn, "search_path%3Dkacho_geo") {
		t.Fatalf("DSN missing kacho_geo search_path: %s", dsn)
	}
	if !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("DSN missing sslmode: %s", dsn)
	}
	if mdsn := c.MigrateDSN(); strings.Contains(mdsn, "pool_max_conns") {
		t.Fatalf("MigrateDSN must not carry pool params: %s", mdsn)
	}
}

func TestServerCreds_insecureByDefault(t *testing.T) {
	var c config.Config
	if err := config.LoadInto(&c, map[string]string{"KACHO_GEO_DB_PASSWORD": "x"}); err != nil {
		t.Fatalf("LoadInto err = %v", err)
	}
	if c.PublicServerMTLS.Enable || c.InternalServerMTLS.Enable {
		t.Fatal("mTLS must default to disabled (dev backward-compat)")
	}
	if _, err := c.PublicServerCreds(); err != nil {
		t.Fatalf("PublicServerCreds (insecure) err = %v", err)
	}
	if _, err := c.InternalServerCreds(); err != nil {
		t.Fatalf("InternalServerCreds (insecure) err = %v", err)
	}
}
