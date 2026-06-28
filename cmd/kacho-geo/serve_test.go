// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"

	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/config"
)

// quietLogger — slog в /dev/null, чтобы тест не шумел.
func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// secure — конфиг с заданными authz и mTLS на обоих листенерах (без breakglass).
func secure() config.Config {
	return config.Config{
		AuthMode:           "dev",
		AuthZIAMGRPCAddr:   "kacho-iam:9091",
		PublicServerMTLS:   grpcsrv.TLSServer{Enable: true},
		InternalServerMTLS: grpcsrv.TLSServer{Enable: true},
	}
}

// ── validateAuthMode: режим + строгость DB-SSL (authz/mTLS — не здесь) ──

func TestValidateAuthMode(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{"dev", config.Config{AuthMode: "dev"}, false},
		{"production", config.Config{AuthMode: "production"}, false},
		{"production-strict + ssl require", config.Config{AuthMode: "production-strict", DBSSLMode: "require"}, false},
		{"production-strict + ssl disable → err", config.Config{AuthMode: "production-strict", DBSSLMode: "disable"}, true},
		{"unknown mode → err", config.Config{AuthMode: "wat"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAuthMode(tc.cfg, quietLogger())
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
		})
	}
}

// ── validateSecurityConfig: secure-by-default; breakglass — единственный обход ──

func TestValidateSecurityConfig(t *testing.T) {
	noMTLS := secure()
	noMTLS.PublicServerMTLS.Enable = false

	noInternalMTLS := secure()
	noInternalMTLS.InternalServerMTLS.Enable = false

	noAuthz := secure()
	noAuthz.AuthZIAMGRPCAddr = ""

	cases := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{"secure (authz + both mTLS) → ok", secure(), false},
		{"no authz addr, no breakglass → err", noAuthz, true},
		{"public mTLS off, no breakglass → err", noMTLS, true},
		{"internal mTLS off, no breakglass → err", noInternalMTLS, true},
		{"breakglass bypasses all requirements → ok", config.Config{AuthZBreakglass: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSecurityConfig(tc.cfg)
			if tc.wantErr != (err != nil) {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
		})
	}
}
