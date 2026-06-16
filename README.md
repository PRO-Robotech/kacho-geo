# kacho-geo

Kachō **Geography** control-plane — the leaf platform-topology service owning
**Region** and **Zone** (global, cluster-scoped read-only catalogs).

Extracted from `kacho-compute` (epic kacho-geo). `kacho-geo` is a **leaf**: it
depends only on `kacho-corelib` + `kacho-proto` by build, and on nothing else in
Kachō. At runtime it consumes `kacho-iam` authz (`geo → iam` Check edge). All
other domains validate their `zone_id` / `region_id` against `kacho-geo` by id.

## Surface

| Service | Methods | Listener |
|---|---|---|
| `RegionService` | `Get`, `List` (sync, read-only) | public `:9090` |
| `ZoneService` | `Get`, `List` (sync, read-only) | public `:9090` |
| `InternalRegionService` | `Create`, `Update`, `Delete` (sync — catalog pattern) | cluster-internal `:9091` |
| `InternalZoneService` | `Create`, `Update`, `Delete` (sync — catalog pattern) | cluster-internal `:9091` |

`Internal*` services are NEVER exposed on the external TLS endpoint (ban #6).
Admin mutations return the resource synchronously (Region/Zone are admin-managed
reference catalogs — recorded scope-deviation from "mutations → Operation").

## Domain model

- `Region{ id, name, created_at }` — admin-assigned immutable id (`ru-central1`).
- `Zone{ id, region_id, status (UP/DOWN/UNSPECIFIED), name, created_at }` —
  `region_id` FK → `regions(id)` **ON DELETE RESTRICT** (a region with zones can
  not be deleted; within-service invariant on the DB level).

Schema `kacho_geo`; seed `ru-central1` + `ru-central1-{a,b,d}` (status UP).

## Security

Per-RPC authz (`InternalIAMService.Check` → OpenFGA) on **both** listeners —
internal `:9091` is not exempt (defense-in-depth). Internal `:9091` runs mTLS +
cert-identity / trusted-principal extraction. Public read → `viewer` floor on
`cluster:cluster_kacho_root`; admin `Internal*` → `system_admin`. Admin mutations
emit a `geo_outbox` audit row atomically in the writer-tx.

## Build & test

```bash
make build           # gRPC API server (cmd/kacho-geo)
make build-migrator  # migrations CLI (cmd/migrator)
make test-short      # unit tests (-short)
make test            # unit + integration (testcontainers Postgres)
```

Integration tests use testcontainers Postgres 16. Locally with colima:

```bash
export DOCKER_HOST=unix:///$HOME/.colima/default/docker.sock
export TESTCONTAINERS_RYUK_DISABLED=true
go test ./internal/repo/... -count=1 -p 1 -timeout 900s
```

## Config (env, prefix `KACHO_GEO_`)

| Var | Default | Meaning |
|---|---|---|
| `KACHO_GEO_DB_{HOST,PORT,USER,PASSWORD,NAME}` | `localhost`/`5432`/`geo`/—/`kacho_geo` | Postgres |
| `KACHO_GEO_GRPC_PORT` / `KACHO_GEO_INTERNAL_PORT` | `9090` / `9091` | listeners |
| `KACHO_GEO_AUTH_MODE` | `dev` | `dev` \| `production` \| `production-strict` |
| `KACHO_GEO_AUTHZ_IAM_GRPC_ADDR` | "" | kacho-iam internal endpoint for per-RPC Check |
| `KACHO_GEO_{PUBLIC,INTERNAL}_SERVER_MTLS_*` | insecure | per-listener mTLS (SEC-B) |
| `KACHO_GEO_IAM_AUTHZ_MTLS_*` | insecure | geo→iam Check client mTLS |

Migrations: `kacho-migrator up|down|status` (DSN via `--dsn`, `KACHO_MIGRATOR_DSN`,
or the `KACHO_GEO_*` config).
