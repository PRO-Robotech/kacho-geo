package check

import (
	"errors"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	"github.com/PRO-Robotech/kacho-corelib/authz"
)

// Options — parameters for NewInterceptor.
type Options struct {
	ServiceName string
	IAMConn     grpc.ClientConnInterface
	Breakglass  bool
	Logger      *slog.Logger

	CheckTimeout         time.Duration
	DenyRateLimitPerSec  float64
	CacheTTL             time.Duration
	AllowSystemPrincipal bool
}

// ErrIAMConnNotConfigured — IAM conn = nil AND Breakglass=false.
var ErrIAMConnNotConfigured = errors.New("check: IAM connection not configured and Breakglass=false")

// NewInterceptor builds the geo authz interceptor. Returns:
//   - (*authz.Interceptor, nil) — success; caller chains Unary()/Stream().
//   - (nil, ErrIAMConnNotConfigured) — IAM not configured AND Breakglass=false.
//     Caller decides: production → fatal; dev → skip interceptor.
func NewInterceptor(opts Options) (*authz.Interceptor, error) {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.Breakglass {
		return authz.NewInterceptor(authz.InterceptorOptions{
			ServiceName:          opts.ServiceName,
			Map:                  PermissionMap(),
			Client:               nil,
			Cache:                authz.NewCache(opts.CacheTTL),
			Logger:               opts.Logger,
			Breakglass:           true,
			DenyRateLimitPerSec:  opts.DenyRateLimitPerSec,
			CheckTimeout:         opts.CheckTimeout,
			AllowSystemPrincipal: opts.AllowSystemPrincipal,
		}), nil
	}
	if opts.IAMConn == nil {
		return nil, ErrIAMConnNotConfigured
	}
	client := NewIAMCheckClient(opts.IAMConn)
	return authz.NewInterceptor(authz.InterceptorOptions{
		ServiceName:          opts.ServiceName,
		Map:                  PermissionMap(),
		Client:               client,
		Cache:                authz.NewCache(opts.CacheTTL),
		Logger:               opts.Logger,
		Breakglass:           false,
		DenyRateLimitPerSec:  opts.DenyRateLimitPerSec,
		CheckTimeout:         opts.CheckTimeout,
		AllowSystemPrincipal: opts.AllowSystemPrincipal,
	}), nil
}
