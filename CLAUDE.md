# kacho-geo — CLAUDE.md

Geo-специфичный файл. Базовые правила Kachō (`.claude/rules/*`) — локальная копия,
синхронизируемая из workspace (`./sync-tooling.sh`; источник истины —
`kacho-workspace/.claude/rules/`, копию здесь не редактировать). `@import` ниже делает
репо самодостаточным и при standalone-клоне. Здесь — **только geo-специфика**.

## Базовые правила Kachō (@import — синканная копия из workspace)

@.claude/rules/00-kacho-core.md
@.claude/rules/api-conventions.md
@.claude/rules/polyrepo.md
@.claude/rules/architecture.md
@.claude/rules/data-integrity.md
@.claude/rules/security.md
@.claude/rules/git-youtrack.md
@.claude/rules/testing.md
@.claude/rules/vault.md
@.claude/rules/ai-tooling.md

## 1. Что это за сервис

gRPC control-plane **Geography** (платформенная топология): **Region** и **Zone** —
глобальные cluster-scoped read-only справочники. **kacho-geo — owner Geography**
(вынесено из kacho-compute, эпик kacho-geo). Это **leaf-сервис**: по build зависит
только от `kacho-corelib` + `kacho-proto`, ни от одного другого сервиса (как iam).
В runtime — **consumer** kacho-iam authz (ребро geo→iam `InternalIAMService.Check`).

Домен `kacho.cloud.geo.v1`, схема Postgres `kacho_geo`, env-префикс `KACHO_GEO_*`.

В скоупе:
- **Region** — Get/List (public, read-only). id admin-assigned, immutable (`ru-central1`).
- **Zone** — Get/List (public, read-only). id admin-assigned (`ru-central1-a`); `region_id`
  FK → regions(id) **ON DELETE RESTRICT**; `status` (UP/DOWN/STATUS_UNSPECIFIED).

Internal endpoints (:9091, не на external TLS — ban #6):
- **InternalRegionService** / **InternalZoneService** — admin CRUD (Create/Update/Delete).
  Catalog-паттерн: возвращают **ресурс синхронно** (НЕ Operation) — осознанная
  scope-deviation (admin-managed reference-каталог с admin-assigned immutable id).

## 2. Доменная модель и FK contract

```
Region (1) ──► (N) Zone   |   Region/Zone — read-only справочники (admin CRUD через Internal*)
zones.region_id → regions(id) ON DELETE RESTRICT (Region.Delete блокируется при наличии зон)
```

- Region/Zone НЕ привязаны к Project/Account — глобальная topology (scope `cluster`).
- Consumer-сервисы (compute/vpc/nlb) ссылаются на zone_id/region_id **по id (TEXT, без
  cross-service FK)** и валидируют через geo `ZoneService.Get`/`RegionService.Get`
  (`data-integrity.md` §cross-domain). dangling-ref переживается грациозно.
- Within-service инвариант (zones→regions) — **на DB-уровне** (FK RESTRICT, ban #10).
- seed: `ru-central1` + `ru-central1-{a,b,d}` (status UP) — в миграции `0001`, идемпотентно
  (`ON CONFLICT DO NOTHING`), чтобы S3 data-migration в те же строки не конфликтовала.

## 3. Безопасность (security.md «AuthN+AuthZ ВЕЗДЕ»)

- **Оба** листенера (public :9090 + internal :9091) — per-RPC authz Check через
  `InternalIAMService.Check` (OpenFGA/ReBAC). Internal **НЕ** освобождён (defense-in-depth).
- Internal :9091 — mTLS (`grpcsrv.TLSServer`) + cert-identity → trusted-principal
  (FD-4 anti-spoof) → authz Check. Public read → viewer-floor (`viewer` на
  `cluster:cluster_kacho_root`); admin Internal* → `system_admin`.
- production-mode (`KACHO_GEO_AUTH_MODE=production`) обязывает: оба листенера mTLS-enable
  + `KACHO_GEO_AUTHZ_IAM_GRPC_ADDR` сконфигурирован (иначе отказ старта). dev-режим
  допускает insecure + skip authz (локальные unit/integration-фикстуры).
- audit: admin Create/Update/Delete пишут строку в `geo_outbox` **атомарно** в той же
  writer-tx (ban #10, `<domain>_outbox` конвенция, parity с compute_outbox/vpc_outbox).

## 4. Чистая архитектура (`internal/`)

- `domain/` — Region/Zone entity + Validate() (чистый Go, stdlib).
- `apps/kacho/api/{region,zone}/` — use-case + port-интерфейс `Repo`.
- `repo/kacho/pg/` — pgx-adapter (handwritten SQL, SQLSTATE→sentinel в `internal/errors`,
  outbox emit в tx); `repo/kacho/repomock/` — моки портов для unit-тестов.
- `handler/` — тонкий transport (parse → use-case → format; protoconv; maperr).
- `check/` — authz interceptor factory + IAM CheckClient + PermissionMap (geo.* FQN).
- `apps/kacho/config/` — YAML/env-config (corelib `config.LoadPrefixed("KACHO_GEO")`).
- `cmd/kacho-geo/` — composition root (serve.go); `cmd/migrator/` — goose-миграции.

## 5. Migrations (`internal/migrations/*.sql`, goose, embed.FS)

Схема `kacho_geo`. `0001_initial.sql` — baseline: `regions`, `zones` (FK RESTRICT),
`geo_outbox` + LISTEN/NOTIFY trigger; seed Region/Zone. search_path задаётся в DSN
(`options=-c search_path=kacho_geo,public`). НЕ редактировать применённую миграцию.

## 6. Тесты

- unit: `internal/{domain,apps/kacho/api/*,check,handler}/*_test.go` — моки портов из
  `repo/kacho/repomock`. `make test-short`.
- integration: `internal/repo/kacho/pg/integration_test.go` — testcontainers PG16, CRUD,
  FK RESTRICT (zone→region), FK violation, seed, outbox emit, concurrent-insert race.
  Self-guard `if testing.Short()`. Локально: `export DOCKER_HOST=…colima…docker.sock;
  export TESTCONTAINERS_RYUK_DISABLED=true; go test ./internal/repo/... -p 1 -timeout 900s`.
- CI: `ci.yaml` (build+vet+test-short+golangci+govuln), `integration.yaml` (HARD gate,
  `go test ./... -race -timeout 1800s`, без continue-on-error), `docker-build.yml`,
  `security-scan.yml` (gosec HIGH=0).

## 7. Local dev

```bash
make build            # API server         |  make build-migrator
make test-short       # unit (-short)      |  make test  (unit + integration)
make migrate-up       # goose up (KACHO_GEO_DB_PASSWORD=secret)
```

## 8. Ссылки

- Acceptance: `../../docs/specs/sub-phase-6.0-kacho-geo-extraction-acceptance.md`
- Proto: `../kacho-proto/proto/kacho/cloud/geo/v1/` (домен `kacho.cloud.geo.v1`, `geov1`)
- Build-граф / runtime-edges: `@.claude/rules/polyrepo.md` (`*→geo`; geo — leaf)
- Баги/tech-debt: GitHub Issues `PRO-Robotech/kacho-geo/issues`
