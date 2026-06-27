# Copyright (c) PRO-Robotech
# SPDX-License-Identifier: BUSL-1.1

BINARY         := kacho-geo
CMD            := ./cmd/kacho-geo
MIGRATOR_BIN   := kacho-migrator
MIGRATOR_CMD   := ./cmd/migrator
IMAGE          := kacho-geo:dev

.PHONY: build build-migrator test test-short vet lint docker proto-install-plugins proto-lint proto-gen

build:
	CGO_ENABLED=0 go build -o bin/$(BINARY) $(CMD)

build-migrator:
	CGO_ENABLED=0 go build -o bin/$(MIGRATOR_BIN) $(MIGRATOR_CMD)

# test — unit + integration (testcontainers Postgres). При contention гнать с
# -p 1 (локально DOCKER_HOST=colima).
test:
	go test ./... -race -cover -timeout 900s

test-short:
	go test ./... -race -cover -short -timeout 120s

vet:
	go vet ./...

lint:
	golangci-lint run ./...

docker:
	docker build -f Dockerfile -t $(IMAGE) .

# proto-install-plugins — ставит protoc-плагины в $GOBIN (lookup через $PATH для buf).
# Доменный proto geo генерируется этими тремя плагинами; permission-catalog для geo —
# hand-written (internal/check/permission_map.go), buf-catalog-плагин не нужен.
proto-install-plugins:
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway

proto-lint:
	cd proto && buf lint

# proto-gen — регенерация Go-stubs доменного proto geo (kacho/cloud/geo/v1) из proto/.
# Универсальная инфра (operation/validation/authz_options/cloud-api/google) вендорится
# в proto/ только для buf-резолва импортов и НЕ генерируется (Go-stubs живут в
# kacho-corelib / canonical genproto) — см. proto/buf.gen.yaml inputs.paths.
proto-gen:
	cd proto && buf generate

.PHONY: migrate-up migrate-down migrate-status
migrate-up: build-migrator
	KACHO_GEO_DB_PASSWORD=secret bin/$(MIGRATOR_BIN) up

migrate-down: build-migrator
	KACHO_GEO_DB_PASSWORD=secret bin/$(MIGRATOR_BIN) down

migrate-status: build-migrator
	KACHO_GEO_DB_PASSWORD=secret bin/$(MIGRATOR_BIN) status
