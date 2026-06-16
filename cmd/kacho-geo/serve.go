package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	coredb "github.com/PRO-Robotech/kacho-corelib/db"
	"github.com/PRO-Robotech/kacho-corelib/grpcclient"
	"github.com/PRO-Robotech/kacho-corelib/grpcsrv"
	"github.com/PRO-Robotech/kacho-corelib/observability"

	geov1 "github.com/PRO-Robotech/kacho-proto/gen/go/kacho/cloud/geo/v1"

	region "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/region"
	zone "github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/api/zone"
	"github.com/PRO-Robotech/kacho-geo/internal/apps/kacho/config"
	"github.com/PRO-Robotech/kacho-geo/internal/check"
	"github.com/PRO-Robotech/kacho-geo/internal/handler"
	"github.com/PRO-Robotech/kacho-geo/internal/repo/kacho/pg"
)

// runServe is the composition root (the single wiring point — no globals).
func runServe(cfg config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	logger := observability.NewSlogger(os.Stdout)
	slog.SetDefault(logger)

	productionMode, err := validateAuthMode(cfg, logger)
	if err != nil {
		return err
	}

	pool, err := coredb.NewPool(ctx, cfg.DSN())
	if err != nil {
		return err
	}
	defer pool.Close()

	// ── use-cases (repo → use-case → handler) ──────────────────────────────
	regionUC := region.New(pg.NewRegionRepo(pool))
	zoneUC := zone.New(pg.NewZoneRepo(pool))

	// ── authz: per-RPC OpenFGA Check on BOTH listeners (security.md invariant:
	// AuthN+AuthZ everywhere — internal :9091 is NOT exempt). The geo→iam Check
	// edge dials kacho-iam internal (:9091) and may present a client-cert (mTLS).
	// In dev (no iam addr, no breakglass) the interceptor is skipped — production
	// mode forbids that below.
	var authzConn *grpc.ClientConn
	if cfg.AuthZIAMGRPCAddr != "" {
		authzCreds, cerr := grpcclient.TLSClientTransportCreds(cfg.IAMAuthzMTLS)
		if cerr != nil {
			return fmt.Errorf("geo→iam Check mTLS creds: %w", cerr)
		}
		authzConn, err = grpc.NewClient(cfg.AuthZIAMGRPCAddr,
			grpc.WithTransportCredentials(authzCreds),
			grpcclient.KeepaliveDialOption(true))
		if err != nil {
			return fmt.Errorf("dial kacho-iam (authz): %w", err)
		}
		defer authzConn.Close()
	}

	authzIntr, aerr := check.NewInterceptor(check.Options{
		ServiceName: "kacho-geo",
		IAMConn:     authzConn,
		Breakglass:  cfg.AuthZBreakglass,
		Logger:      logger,
	})

	// ── interceptor chains ─────────────────────────────────────────────────
	// Public (:9090): principal-extract → authz Check.
	publicUnary := []grpc.UnaryServerInterceptor{grpcsrv.UnaryPrincipalExtract()}
	publicStream := []grpc.StreamServerInterceptor{grpcsrv.StreamPrincipalExtract()}
	// Internal (:9091): cert-identity → trusted-principal (FD-4 anti-spoof) →
	// authz Check. SAME per-RPC authz as public — internal is NOT trusted
	// (defense-in-depth against lateral movement, security.md).
	internalUnary := []grpc.UnaryServerInterceptor{
		grpcsrv.UnaryCertIdentityExtract(),
		grpcsrv.UnaryTrustedPrincipalExtract(),
	}
	internalStream := []grpc.StreamServerInterceptor{
		grpcsrv.StreamCertIdentityExtract(),
		grpcsrv.StreamTrustedPrincipalExtract(),
	}

	switch {
	case aerr == nil && authzIntr != nil:
		publicUnary = append(publicUnary, authzIntr.Unary())
		publicStream = append(publicStream, authzIntr.Stream())
		internalUnary = append(internalUnary, authzIntr.Unary())
		internalStream = append(internalStream, authzIntr.Stream())
		logger.Info("authz interceptor enabled",
			"iam_endpoint", cfg.AuthZIAMGRPCAddr,
			"breakglass", cfg.AuthZBreakglass,
			"listeners", "public+internal")
	case errors.Is(aerr, check.ErrIAMConnNotConfigured):
		if productionMode {
			return errors.New("production mode requires KACHO_GEO_AUTHZ_IAM_GRPC_ADDR (per-RPC authz Check); refusing to start without authz")
		}
		logger.Warn("authz interceptor NOT enabled — KACHO_GEO_AUTHZ_IAM_GRPC_ADDR not configured (dev mode)")
	case aerr != nil:
		return fmt.Errorf("build authz interceptor: %w", aerr)
	}

	// ── server-creds (opt-in mTLS per listener; production requires enable) ──
	if productionMode {
		if !cfg.PublicServerMTLS.Enable {
			return errors.New("production mode requires public listener mTLS (KACHO_GEO_PUBLIC_SERVER_MTLS_ENABLE=true)")
		}
		if !cfg.InternalServerMTLS.Enable {
			return errors.New("production mode requires internal listener mTLS (KACHO_GEO_INTERNAL_SERVER_MTLS_ENABLE=true)")
		}
	}
	publicCreds, err := cfg.PublicServerCreds()
	if err != nil {
		return fmt.Errorf("public listener tls creds: %w", err)
	}
	internalCreds, err := cfg.InternalServerCreds()
	if err != nil {
		return fmt.Errorf("internal listener tls creds: %w", err)
	}

	grpcSrv := grpcsrv.NewServer(
		publicCreds,
		grpc.ChainUnaryInterceptor(publicUnary...),
		grpc.ChainStreamInterceptor(publicStream...),
	)
	internalSrv := grpcsrv.NewServer(
		internalCreds,
		grpc.ChainUnaryInterceptor(internalUnary...),
		grpc.ChainStreamInterceptor(internalStream...),
	)

	// Public read-only services on :9090.
	geov1.RegisterRegionServiceServer(grpcSrv, handler.NewRegionHandler(regionUC))
	geov1.RegisterZoneServiceServer(grpcSrv, handler.NewZoneHandler(zoneUC))
	// Admin CRUD services ONLY on the cluster-internal :9091 (ban #6).
	geov1.RegisterInternalRegionServiceServer(internalSrv, handler.NewInternalRegionHandler(regionUC))
	geov1.RegisterInternalZoneServiceServer(internalSrv, handler.NewInternalZoneHandler(zoneUC))

	listener, err := net.Listen("tcp", ":"+cfg.GrpcPort)
	if err != nil {
		return err
	}
	internalListener, err := net.Listen("tcp", ":"+cfg.InternalGrpcPort)
	if err != nil {
		_ = listener.Close()
		return err
	}
	logger.Info("kacho-geo listening",
		"public_mtls", cfg.PublicServerMTLS.Enable,
		"internal_mtls", cfg.InternalServerMTLS.Enable,
		"public_port", cfg.GrpcPort,
		"internal_port", cfg.InternalGrpcPort)

	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		internalSrv.GracefulStop()
		grpcSrv.GracefulStop()
	}()

	go func() {
		if serr := internalSrv.Serve(internalListener); serr != nil && !errors.Is(serr, grpc.ErrServerStopped) {
			logger.Error("internal grpc server stopped", "err", serr)
		}
	}()

	serveErr := grpcSrv.Serve(listener)
	cancel()
	<-shutdownDone
	return serveErr
}

// validateAuthMode parses KACHO_GEO_AUTH_MODE (whitelist), logging insecure
// dev-defaults. production / production-strict → productionMode=true.
func validateAuthMode(cfg config.Config, logger *slog.Logger) (bool, error) {
	switch cfg.AuthMode {
	case "dev":
		if cfg.DBSSLMode == "" || cfg.DBSSLMode == "disable" {
			logger.Warn("KACHO_GEO_DB_SSLMODE=disable — DB plaintext (dev only)")
		}
		return false, nil
	case "production":
		logger.Warn("AuthMode=production: anonymous callers will be rejected")
		return true, nil
	case "production-strict":
		switch cfg.DBSSLMode {
		case "require", "verify-ca", "verify-full":
		default:
			return false, fmt.Errorf("production-strict mode: KACHO_GEO_DB_SSLMODE must be one of require|verify-ca|verify-full (got %q)", cfg.DBSSLMode)
		}
		logger.Warn("AuthMode=production-strict: anonymous rejected + DB SSL strictly validated")
		return true, nil
	default:
		return false, fmt.Errorf("unknown KACHO_GEO_AUTH_MODE=%q (allowed: dev, production, production-strict)", cfg.AuthMode)
	}
}
