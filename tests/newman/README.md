# kacho-geo — Newman black-box coverage (planned)

Status: **coverage gap tracked here** — this repo has strong Go unit +
integration (testcontainers) coverage, but no black-box Newman suite that
traverses the api-gateway REST mux yet. Project rule #12 mandates, for every new
RPC, at least one happy-path + one negative Newman case through the gateway.

This note is the landing spot for that suite (mirroring the declarative
`cases/*.py` → `gen.py` layout used by `kacho-vpc/tests/newman`). It is
intentionally additive test tooling and requires a deployed stack
(api-gateway + kacho-geo + Postgres), so it is authored against `kacho-deploy`
rather than run from this repo's `go test`.

## RPCs to cover (10)

Public read-only (`:9090` REST via gateway):

- `RegionService.Get`  — `GET /geo/v1/regions/{regionId}`
- `RegionService.List` — `GET /geo/v1/regions`
- `ZoneService.Get`    — `GET /geo/v1/zones/{zoneId}`
- `ZoneService.List`   — `GET /geo/v1/zones`

Internal admin CRUD (`:9091`, internal mux only — MUST NOT be reachable on the
public endpoint):

- `InternalRegionService.Create/Update/Delete` — `/geo/v1/regions[:verb]`
- `InternalZoneService.Create/Update/Delete`   — `/geo/v1/zones[:verb]`

## Required cases

| Case | Kind | Expectation |
|---|---|---|
| `REGION-GET-HAPPY` | happy | seeded region → 200 + body |
| `REGION-GET-NEG-NOTFOUND` | negative | absent id → `NOT_FOUND` |
| `REGION-LIST-HAPPY` | happy | pagination page_size + next_page_token |
| `REGION-LIST-NEG-BADTOKEN` | negative | garbage `page_token` → `INVALID_ARGUMENT` |
| `ZONE-GET-HAPPY` / `ZONE-GET-NEG-NOTFOUND` | happy/neg | mirror region |
| `ZONE-LIST-HAPPY` / `ZONE-LIST-NEG-BADTOKEN` | happy/neg | mirror region |
| `REGION-CREATE-HAPPY` | happy | Operation → poll `OperationService.Get` to `done=true`, response=Region |
| `REGION-DELETE-NEG-HASZONES` | negative | delete region with zones → Operation.error `FAILED_PRECONDITION` |
| `ZONE-CREATE-NEG-GHOSTREGION` | negative | create zone with absent region_id → Operation.error `FAILED_PRECONDITION` |
| `ZONE-UPDATE-NEG-GHOSTREGION` | negative | re-point region_id to absent region → Operation.error `FAILED_PRECONDITION` |
| `ADMIN-NOT-ON-PUBLIC` | security | `InternalRegionService`/`InternalZoneService` verbs unreachable on the public `:9090` REST mux |

The `ADMIN-NOT-ON-PUBLIC` case is the black-box guard for the Internal-vs-external
split (CLAUDE.md §Запреты #6): an api-gateway restmux misregistration that exposed
an admin verb on the public endpoint would ship green today, because every
existing test calls the Go handlers/use-cases directly and never crosses the
gateway.

## Why not blocking this branch

Authoring + running the suite needs the deployed stack; it cannot be verified by
`go test` in this repo. The security-critical slice of this gap
(`ADMIN-NOT-ON-PUBLIC`) is separately guaranteed at wiring level: admin CRUD is
registered only on the internal `:9091` server in `cmd/kacho-geo/serve.go`, and
that wiring is covered by `cert_bound_identity_test.go` /
`public_principal_test.go`.
