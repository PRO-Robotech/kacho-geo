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

**Direct `geov1` / `protoconv` / `anypb` use in the use-case.** For the same root
cause, the use-cases also import the generated `geov1` stubs and marshal
`domain.Zone`/`domain.Region` into a proto `Any` (`geov1.{Create,Update,Delete}*Metadata`
at the mutation entrypoints; `marshalZone`/`marshalRegion` → `protoconv` + `anypb.New`
inside the `operations.Run` closure). This is **mandated by the corelib LRO
callback contract**, not a geo-local leak: `operations.NewFromContext` takes the
operation-metadata proto, and the `operations.Run(ctx, repo, id, func) (*anypb.Any, error)`
closure signature requires the terminal response to be an `Any`. Every kacho
service (`kacho-vpc`, `kacho-compute`, `kacho-nlb`) emits the LRO metadata/response
`Any` from inside the use-case identically — moving it to the handler would
diverge geo from the platform LRO pattern (godzila regime) without removing the
proto dependency, since the closure runs on the corelib worker, not the handler.
`protoconv` is the single field-mapping projection shared by handler, LRO-recovery,
and this marshal path, so there is no drift. Not a defect; not planned to change.

## 2. Black-box Newman suite not run from this repo

See `tests/newman/README.md`. The suite is tracked as a concrete open ticket
([PRO-Robotech/kacho-geo#10](https://github.com/PRO-Robotech/kacho-geo/issues/10),
`Tests-followup` per rule #12), authored against the deployed stack
(`kacho-deploy`), not `go test` in this repo. The security-critical slices are
covered at the Go layer today: the admin-verbs-not-on-public split by
`cmd/kacho-geo/serve_registration_test.go` (inspects the real
`grpc.Server.GetServiceInfo()` of both listeners), and OperationService
owner-scoping by `internal/handler/operation_owner_test.go` +
`internal/repo/kacho/pg/operation_owner_integration_test.go`.

## 3. Config via corelib `envconfig` struct-tags, not YAML/viper/koanf

**What.** `internal/apps/kacho/config` binds all settings through
`envconfig:"…"` struct-tags via `corelib config.LoadPrefixed("KACHO_GEO")`,
rather than a YAML file loaded through viper/koanf as the evgeniy regime
prescribes.

**Why it is not a geo-local defect.** This is the **platform-wide** config
mechanism: `kacho-corelib/config` exposes `LoadPrefixed`, and every kacho service
(`kacho-vpc`, `kacho-compute`, `kacho-iam`, `kacho-nlb`, …) uses it identically.
Env-only 12-factor config is a deliberate cross-service decision; per-edge TLS
blocks are expressed via env-name prefixing. Migrating to a YAML/viper loader is a
workspace-wide corelib change, not a per-service one — it would be made once in
corelib for all services or not at all. Recorded here so the regime item is not
re-flagged per service.

**Boundary.** If layered/file-based config with hot-reload is ever required
platform-wide, the change lands in `kacho-corelib/config` (keeping the per-edge
TLS structs), and every service picks it up. Not planned.

## 4. Resource-id validated by a `domain.ValidateID` function, not a newtype

**What.** `Region.ID`, `Zone.ID`, `Zone.RegionID` remain bare `string` fields.
The id-format invariant (lowercase slug `^[a-z][a-z0-9-]*$`, hyphen-separated, ≤63
chars) is enforced by `domain.ValidateID`, called from `Region.Validate` /
`Zone.Validate` on the Create path (sec-hardening-r3) — malformed ids are rejected
synchronously with `InvalidArgument` and never persisted as the canonical
cross-service reference key. This closes the substantive gap (no id-format
contract) that the audit flagged.

**Why not a self-validating newtype.** The evgeniy/godzila regime prefers domain
newtypes (`RegionID`/`ZoneID`) over bare primitives. A validating function was
chosen instead because a full newtype rollout ripples through domain structs,
repo scan targets, protoconv, and the reconciler read-ports without adding
enforcement the function does not already provide — the invariant is fully
enforced either way. The newtype refactor is a style-only follow-up (regime
alignment), not a security/consistency gap.

**Note (owner-scope, no admin bypass).** `OperationHandler.Get/Cancel` owner-scope
strictly by creator-principal with **no** cluster-admin bypass (unlike
`kacho-vpc`, which has a `tenant.Admin` cross-cut). geo has no tenant/admin ctx
concept — every mutation already requires `system_admin`, and each operation
belongs to the admin that created it — so a bypass would be dead surface. This is
intentional, not a missing feature.
