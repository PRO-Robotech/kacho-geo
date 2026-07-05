# kacho-geo — known, by-design divergences

Deliberate deviations that are **not** defects. Each is bounded and documented
here so a future reviewer does not re-open them as findings.

## 1. `google.golang.org/grpc` remains a transitive dep of the use-case layer

**What.** After the CQRS + error-mapper refactor (`internal/apps/kacho/api/{region,zone}`),
the use-case packages no longer import `serviceerr`, `grpc/codes`, or
`grpc/status` directly — transport-code selection is injected as an
`ErrToStatus func(error) error` from the composition root (handler-owned
`serviceerr.ToStatus`). However `go list -deps ./internal/apps/kacho/api/zone`
still lists `google.golang.org/grpc` (and `jackc/pgx`).

**Why it is not a defect.** Both come from `github.com/PRO-Robotech/kacho-corelib/operations`,
which every kacho service's use-case imports for the async LRO envelope
(`operations.Repo`, `operations.Run`, `operations.NewFromContext`). The corelib
`operations` package is a horizontal cross-cutting concern (its `Repo` is a
pgx-backed table and its worker persists a `google.rpc.Status`); it is shared
identically by `kacho-vpc`, `kacho-compute`, `kacho-nlb`. The Clean-Architecture
rule this repo enforces is "the **service's own** adapter concerns
(pgx SQLSTATE translation, gRPC-code **selection**) must not live in the
use-case" — that is satisfied: SQLSTATE→sentinel lives in
`internal/repo/kacho/dberr`, and code selection is injected. The residual
transitive import via a corelib horizontal is out of scope and would only be
removable by a workspace-wide corelib redesign.

**Boundary.** If the LRO worker contract were ever changed so the closure could
return a plain domain sentinel (corelib-side mapper), this residual would
disappear. Not planned.

## 2. Black-box Newman suite not run from this repo

See `tests/newman/README.md`. The suite is planned/tracked, authored against the
deployed stack (`kacho-deploy`), not `go test` in this repo. The
security-critical slice (admin verbs not on the public endpoint) is covered at
wiring level by `cmd/kacho-geo/cert_bound_identity_test.go` and
`public_principal_test.go`.
