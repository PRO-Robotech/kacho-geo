FROM --platform=$BUILDPLATFORM mirror.gcr.io/library/golang:1.25-alpine AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src

# Polyrepo build-context = parent dir (kacho-workspace/project); siblings are
# COPY'd next to kacho-geo so the `replace ../kacho-{corelib,proto}` resolve.
COPY kacho-corelib /src/kacho-corelib
COPY kacho-proto /src/kacho-proto
COPY kacho-geo /src/kacho-geo

WORKDIR /src/kacho-geo
RUN go mod download
# Two independent binaries in one image (skill evgeniy §9 K.1 / AP-9):
#   kacho-geo      — gRPC API server (`serve`).
#   kacho-migrator — migrations CLI (up|down|status); used by the deploy init-container.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /kacho-geo ./cmd/kacho-geo \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /kacho-migrator ./cmd/migrator

FROM mirror.gcr.io/library/alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /kacho-geo /usr/local/bin/kacho-geo
COPY --from=builder /kacho-migrator /usr/local/bin/kacho-migrator
USER 65532
ENTRYPOINT ["/usr/local/bin/kacho-geo"]
CMD ["serve"]
