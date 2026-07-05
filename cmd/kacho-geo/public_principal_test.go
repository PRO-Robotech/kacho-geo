// Copyright (c) PRO-Robotech
// SPDX-License-Identifier: BUSL-1.1

package main

// public_principal_test.go — anti-spoof guard для ПУБЛИЧНОГО листенера (:9090),
// где живут read-only RegionService / ZoneService.Get/List.
//
// SECURITY: до фикса публичная цепочка использовала bare
// grpcsrv.UnaryPrincipalExtract(), читавшую forwarded x-kacho-principal-* metadata
// БЕЗУСЛОВНО — любой peer с валидным platform-issued client-cert'ом (mTLS —
// RequireAndVerifyClientCert) мог выставить произвольный principal-header и быть
// авторизован как этот principal (CWE-290). Internal-листенер уже trust-gated;
// публичный — нет. Fix навешивает ту же trust-aware связку
// grpcsrv.UnaryCertIdentityExtract + UnaryTrustedPrincipalExtract(
// WithTrustedForwarders(...)) и на публичную цепочку: verified-но-не-форвардер
// peer не может выдать себя за viewer-principal'а, honored только api-gateway.

import (
	"strings"
	"testing"

	"google.golang.org/grpc"

	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"
	"github.com/PRO-Robotech/kacho-corelib/operations"
)

// --- 1. source-level wiring guard ---

// TestPublicListener_TrustedPrincipalExtract_HasForwarderAllowlist — оба
// публичных набора интерсепторов (publicUnary/publicStream) обязаны навешивать
// trust-aware TrustedPrincipalExtract С allow-list форвардеров
// WithTrustedForwarders(...), а не bare UnaryPrincipalExtract (доверяющий любому
// peer'у, выставившему x-kacho-principal-*).
func TestPublicListener_TrustedPrincipalExtract_HasForwarderAllowlist(t *testing.T) {
	src := readServeSrc(t)

	for _, l := range []struct {
		name    string
		marker  string
		trusted string
	}{
		{"publicUnary", "publicUnary := []grpc.UnaryServerInterceptor{", "grpcsrv.UnaryTrustedPrincipalExtract("},
		{"publicStream", "publicStream := []grpc.StreamServerInterceptor{", "grpcsrv.StreamTrustedPrincipalExtract("},
	} {
		block := braceBlockAfter(t, src, l.marker)
		if !strings.Contains(block, l.trusted) {
			t.Fatalf("%s: missing %s — публичный principal НЕ trust-gated", l.name, l.trusted)
		}
		if !strings.Contains(block, "grpcsrv.WithTrustedForwarders(") {
			t.Errorf("%s: TrustedPrincipalExtract без WithTrustedForwarders(...) — "+
				"любой mTLS-verified peer форвардит произвольного principal'а "+
				"(principal-spoofing на публичном read-endpoint)", l.name)
		}
		if strings.Contains(block, "grpcsrv.UnaryPrincipalExtract(") ||
			strings.Contains(block, "grpcsrv.StreamPrincipalExtract(") {
			t.Errorf("%s: bare PrincipalExtract присутствует — forwarded principal доверяется безусловно", l.name)
		}
	}
}

// --- 2. behavioral guard ---

// TestPublicPrincipalChain_ForwarderAllowlist_DropsNonGateway — точная публичная
// цепочка (CertIdentityExtract → TrustedPrincipalExtract) с allow-list api-gateway
// SA: verified-но-не-форвардер peer (внутренний сервис vpc/compute со своим
// валидным cert'ом) форвардить viewer-principal НЕ может; api-gateway — может.
func TestPublicPrincipalChain_ForwarderAllowlist_DropsNonGateway(t *testing.T) {
	chain := publicPrincipalChain(gatewaySAN)

	t.Run("verified_non_gateway_forged_viewer_dropped", func(t *testing.T) {
		other := "spiffe://kacho.cloud/ns/kacho/sa/kacho-compute"
		ctx := withForgedPrincipal(verifiedPeerCtx(t, other), "usr-viewer-victim")

		carrierID, trusted := runChain(t, chain, ctx)
		if trusted {
			t.Errorf("verified-но-не-форвардер peer (%s) НЕ должен форвардить end-user principal'а", other)
		}
		if carrierID == "usr-viewer-victim" {
			t.Errorf("principal-spoofing: peer проштамповал 'usr-viewer-victim' как subject FGA Check на публичном endpoint")
		}
		if carrierID != operations.SystemPrincipal().ID {
			t.Errorf("forged principal протёк в carrier: got %q, want system fallback %q", carrierID, operations.SystemPrincipal().ID)
		}
	})

	t.Run("gateway_peer_principal_honored", func(t *testing.T) {
		ctx := withForgedPrincipal(verifiedPeerCtx(t, gatewaySAN), "usr-viewer")

		carrierID, trusted := runChain(t, chain, ctx)
		if !trusted {
			t.Errorf("principal от доверенного форвардера (api-gateway SAN) обязан быть honored")
		}
		if carrierID != "usr-viewer" {
			t.Errorf("gateway-forwarded principal не honored: got %q, want %q", carrierID, "usr-viewer")
		}
	})
}

// publicPrincipalChain собирает ту же unary-цепочку principal-extract, что
// serve.go навешивает на ПУБЛИЧНЫЙ листенер (после фикса). forwarderSANs
// пробрасываются как WithTrustedForwarders (allow-list доверенных форвардеров).
func publicPrincipalChain(forwarderSANs ...string) grpc.UnaryServerInterceptor {
	return chainUnaryServer(
		grpcsrv.UnaryCertIdentityExtract(),
		grpcsrv.UnaryTrustedPrincipalExtract(grpcsrv.WithTrustedForwarders(forwarderSANs...)),
	)
}
