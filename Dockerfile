# syntax=docker/dockerfile:1

# Build the pure-Go static binary. The builder runs on the native $BUILDPLATFORM
# and cross-compiles to the requested $TARGETPLATFORM, so multi-arch builds need
# no emulation. Pin the Go toolchain to the version declared in go.mod.
FROM --platform=$BUILDPLATFORM golang:1.26.4-bookworm AS build

WORKDIR /src

# Resolve modules first so this layer is reused when only sources change. Only
# the module graph and Go packages enter the build context (see .dockerignore).
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal

# CGO_ENABLED=0 keeps the binary fully static (pure-Go modernc.org/sqlite), which
# is what allows the distroless *static* runtime below. -trimpath drops local
# paths for reproducibility; -s -w strip the symbol table and DWARF to shrink it.
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w" -o /pod-log-preserver ./cmd/pod-log-preserver

# Distroless static, running as root (uid 0): reading kubelet-owned logs under
# /var/log/pods and creating hardlinks into /var/log/pods-preserved both require
# root, so the distroless `nonroot` tag must not be used.
FROM gcr.io/distroless/static-debian12:latest

COPY --from=build /pod-log-preserver /pod-log-preserver

USER 0
ENTRYPOINT ["/pod-log-preserver"]
