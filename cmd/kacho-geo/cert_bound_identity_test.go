// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package main

// cert_bound_identity_test.go — anti-confused-deputy guard для internal-листенера
// (:9091), где живут admin-CRUD InternalRegionService / InternalZoneService.
//
// SECURITY: principal — единственный subject per-RPC FGA Check. Trust-aware
// связка grpcsrv.UnaryCertIdentityExtract + UnaryTrustedPrincipalExtract снимает
// forwarded x-kacho-principal-* metadata на peer'е без verified mTLS-cert'а. Но
// БЕЗ allow-list форвардеров (WithTrustedForwarders) ЛЮБОЙ verified mTLS-peer
// (напр. другой внутренний сервис со своим валидным client-cert'ом) мог форвардить
// произвольного principal'а (в т.ч. администратора) → эскалация до admin-CRUD
// Region/Zone (confused-deputy). Единственный легитимный источник forwarded
// end-user principal'а на :9091 — api-gateway; consumer'ы vpc/compute/nlb ходят в
// публичный :9090 (read-only Zone/Region.Get), а не в internal. Fix пробрасывает
// allow-list SAN'ов доверенных форвардеров (WithTrustedForwarders, config-driven)
// в оба internal-интерсептора: у verified-но-не-форвардер peer'а principal
// снимается → authz видит no-principal → fail-closed deny.
//
// Два комплементарных стража:
//   1. wiring guard (source-level): internalUnary/internalStream навешивают
//      TrustedPrincipalExtract С WithTrustedForwarders(...) — не bare-вариант.
//   2. behavioral guard: точная цепочка internal-листенера с allow-list доказывает,
//      что forged principal verified-но-не-форвардер peer'а снимается (carrier —
//      SystemPrincipal, trusted=false), а api-gateway — honored.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/url"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"

	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"
	"github.com/PRO-Robotech/kacho-corelib/operations"
)

const gatewaySAN = "spiffe://kacho.cloud/ns/kacho/sa/kacho-api-gateway"

// --- 1. source-level wiring guard ---

// TestInternalListener_TrustedPrincipalExtract_HasForwarderAllowlist — оба
// internal-набора интерсепторов (internalUnary/internalStream) обязаны навешивать
// TrustedPrincipalExtract С allow-list форвардеров WithTrustedForwarders(...),
// а не bare-вариант (доверяющий любому verified peer'у).
//
// RED-демонстрация: вернуть bare grpcsrv.UnaryTrustedPrincipalExtract() без
// WithTrustedForwarders — тест падает (confused-deputy снова открыт).
func TestInternalListener_TrustedPrincipalExtract_HasForwarderAllowlist(t *testing.T) {
	src := readServeSrc(t)

	for _, l := range []struct {
		name    string
		marker  string
		trusted string
	}{
		{"internalUnary", "internalUnary := []grpc.UnaryServerInterceptor{", "grpcsrv.UnaryTrustedPrincipalExtract("},
		{"internalStream", "internalStream := []grpc.StreamServerInterceptor{", "grpcsrv.StreamTrustedPrincipalExtract("},
	} {
		block := braceBlockAfter(t, src, l.marker)
		if !strings.Contains(block, l.trusted) {
			t.Fatalf("%s: missing %s — internal principal НЕ trust-gated", l.name, l.trusted)
		}
		if !strings.Contains(block, "grpcsrv.WithTrustedForwarders(") {
			t.Errorf("%s: TrustedPrincipalExtract без WithTrustedForwarders(...) — "+
				"любой verified mTLS-peer форвардит произвольного principal'а "+
				"(confused-deputy priv-esc до admin-CRUD Region/Zone)", l.name)
		}
	}
}

// --- 2. behavioral guard ---

// TestInternalPrincipalChain_ForwarderAllowlist_DropsNonGateway — точная
// internal-цепочка (CertIdentityExtract → TrustedPrincipalExtract) с allow-list
// api-gateway SA. Verified-но-не-форвардер peer (внутренний сервис со своим
// валидным cert'ом) форвардить пользователя НЕ может; api-gateway — может.
func TestInternalPrincipalChain_ForwarderAllowlist_DropsNonGateway(t *testing.T) {
	chain := internalPrincipalChain(gatewaySAN)

	t.Run("verified_non_gateway_forged_admin_dropped", func(t *testing.T) {
		other := "spiffe://kacho.cloud/ns/kacho/sa/kacho-vpc"
		ctx := withForgedPrincipal(verifiedPeerCtx(t, other), "usr-admin-victim")

		carrierID, trusted := runChain(t, chain, ctx)
		if trusted {
			t.Errorf("verified-но-не-форвардер peer (%s) НЕ должен форвардить end-user principal'а", other)
		}
		if carrierID == "usr-admin-victim" {
			t.Errorf("confused-deputy: internal-сервис проштамповал admin-principal'а 'usr-admin-victim' как subject FGA Check")
		}
		if carrierID != operations.SystemPrincipal().ID {
			t.Errorf("forged principal протёк в carrier: got %q, want system fallback %q", carrierID, operations.SystemPrincipal().ID)
		}
	})

	t.Run("gateway_peer_principal_honored", func(t *testing.T) {
		ctx := withForgedPrincipal(verifiedPeerCtx(t, gatewaySAN), "usr-admin")

		carrierID, trusted := runChain(t, chain, ctx)
		if !trusted {
			t.Errorf("principal от доверенного форвардера (api-gateway SAN) обязан быть honored")
		}
		if carrierID != "usr-admin" {
			t.Errorf("gateway-forwarded principal не honored: got %q, want %q", carrierID, "usr-admin")
		}
	})
}

// --- helpers ---

// internalPrincipalChain собирает ту же unary-цепочку principal-extract, что
// serve.go навешивает на internal-листенер. forwarderSANs пробрасываются как
// WithTrustedForwarders (allow-list доверенных форвардеров).
func internalPrincipalChain(forwarderSANs ...string) grpc.UnaryServerInterceptor {
	return chainUnaryServer(
		grpcsrv.UnaryCertIdentityExtract(),
		grpcsrv.UnaryTrustedPrincipalExtract(grpcsrv.WithTrustedForwarders(forwarderSANs...)),
	)
}

func runChain(t *testing.T, chain grpc.UnaryServerInterceptor, ctx context.Context) (carrierID string, trusted bool) {
	t.Helper()
	final := func(c context.Context, _ any) (any, error) {
		carrierID = operations.PrincipalFromContext(c).ID
		_, trusted = grpcsrv.TrustedPrincipalFromContext(c)
		return nil, nil
	}
	if _, err := chain(ctx, nil, nil, final); err != nil {
		t.Fatalf("chain returned error: %v", err)
	}
	return carrierID, trusted
}

func withForgedPrincipal(ctx context.Context, id string) context.Context {
	return metadata.NewIncomingContext(ctx, metadata.Pairs(
		grpcsrv.MDKeyPrincipalType, "user",
		grpcsrv.MDKeyPrincipalID, id,
		grpcsrv.MDKeyPrincipalDisplay, id+"@example.com",
	))
}

// verifiedPeerCtx — mTLS-verified peer: непустая verified-chain с leaf-cert'ом,
// несущим переданный SPIFFE-SAN.
func verifiedPeerCtx(t *testing.T, san string) context.Context {
	t.Helper()
	leaf := &x509.Certificate{URIs: mustParseURIs(t, san)}
	tlsPeer := &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{leaf}},
	}}}
	return peer.NewContext(context.Background(), tlsPeer)
}

// chainUnaryServer композирует unary server-интерсепторы слева-направо вокруг
// финального handler'а (семантика grpc.ChainUnaryInterceptor).
func chainUnaryServer(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		chained := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			ic := interceptors[i]
			next := chained
			chained = func(c context.Context, r any) (any, error) { return ic(c, r, info, next) }
		}
		return chained(ctx, req)
	}
}

func readServeSrc(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("serve.go")
	if err != nil {
		t.Fatalf("read serve.go: %v", err)
	}
	return string(b)
}

// braceBlockAfter возвращает текст { ... }-блока, начинающегося с открывающей
// фигурной скобки в marker, балансируя скобки. Используется для среза
// интерсептор-слайсов internalUnary/internalStream из serve.go.
func braceBlockAfter(t *testing.T, src, marker string) string {
	t.Helper()
	i := strings.Index(src, marker)
	if i < 0 {
		t.Fatalf("serve.go: marker %q не найден", marker)
	}
	open := strings.LastIndexByte(src[:i+len(marker)], '{')
	depth := 0
	for j := open; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[open : j+1]
			}
		}
	}
	t.Fatalf("serve.go: несбалансированные скобки после marker %q", marker)
	return ""
}

func mustParseURIs(t *testing.T, raw ...string) []*url.URL {
	t.Helper()
	out := make([]*url.URL, 0, len(raw))
	for _, r := range raw {
		u, err := url.Parse(r)
		if err != nil {
			t.Fatalf("parse uri %q: %v", r, err)
		}
		out = append(out, u)
	}
	return out
}
