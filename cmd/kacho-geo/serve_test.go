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
		{"dev + ssl disable → ok (dev unaffected)", config.Config{AuthMode: "dev", DBSSLMode: "disable"}, false},
		{"dev + ssl empty → ok (dev unaffected)", config.Config{AuthMode: "dev", DBSSLMode: ""}, false},
		{"production + ssl require → ok", config.Config{AuthMode: "production", DBSSLMode: "require"}, false},
		{"production + ssl verify-full → ok", config.Config{AuthMode: "production", DBSSLMode: "verify-full"}, false},
		{"production + ssl disable → err", config.Config{AuthMode: "production", DBSSLMode: "disable"}, true},
		{"production + ssl empty → err", config.Config{AuthMode: "production", DBSSLMode: ""}, true},
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

	// В production/production-strict пустой allow-list доверенных форвардеров —
	// критичный gap: любой mTLS-verified peer может форвардить произвольного
	// principal'а (confused-deputy до admin-CRUD). Секьюр-гейт обязан отвергать
	// старт без запиненного SAN api-gateway. В dev — back-compat, пусто допустимо.
	prodNoFwd := secure()
	prodNoFwd.AuthMode = "production"

	prodWithFwd := secure()
	prodWithFwd.AuthMode = "production"
	prodWithFwd.AuthZTrustedForwarderSANs = []string{gatewaySAN}

	prodStrictNoFwd := secure()
	prodStrictNoFwd.AuthMode = "production-strict"

	prodStrictWithFwd := secure()
	prodStrictWithFwd.AuthMode = "production-strict"
	prodStrictWithFwd.AuthZTrustedForwarderSANs = []string{gatewaySAN}

	// Пустая строка в списке — не форвардер (corelib WithTrustedForwarders
	// отбрасывает "" → пустой allow-list → trust-any). Должен отвергаться так же.
	prodEmptyStrFwd := secure()
	prodEmptyStrFwd.AuthMode = "production"
	prodEmptyStrFwd.AuthZTrustedForwarderSANs = []string{""}

	devNoFwd := secure()
	devNoFwd.AuthMode = "dev"

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
		{"production without trusted forwarders → err", prodNoFwd, true},
		{"production with trusted forwarder → ok", prodWithFwd, false},
		{"production-strict without trusted forwarders → err", prodStrictNoFwd, true},
		{"production-strict with trusted forwarder → ok", prodStrictWithFwd, false},
		{"production with empty-string forwarder (trust-any) → err", prodEmptyStrFwd, true},
		{"dev without trusted forwarders → ok (back-compat)", devNoFwd, false},
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
