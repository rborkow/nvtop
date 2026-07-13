# syntax=docker/dockerfile:1
# Build context is the REPO ROOT:  docker build -f web/Dockerfile .

# --- Stage 1: build nvtop from this repo's source ---
FROM debian:bookworm-slim AS nvtop-build
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential cmake pkg-config libncurses-dev libdrm-dev libsystemd-dev \
 && rm -rf /var/lib/apt/lists/*
COPY . /src
RUN cmake -S /src -B /build -DCMAKE_BUILD_TYPE=Release \
 && cmake --build /build -j"$(nproc)"

# --- Stage 2: build the Go sidecar (compiles natively on the build host) ---
FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS go-build
ARG TARGETOS TARGETARCH
WORKDIR /w
COPY web/ .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /nvtop-web ./cmd/nvtop-web

# --- Stage 3: runtime ---
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      libncursesw6 libtinfo6 libdrm2 libdrm-amdgpu1 libsystemd0 \
 && rm -rf /var/lib/apt/lists/*
COPY --from=nvtop-build /build/src/nvtop /usr/local/bin/nvtop
COPY --from=go-build /nvtop-web /usr/local/bin/nvtop-web
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s \
  CMD ["/usr/local/bin/nvtop-web", "--healthcheck"]
ENTRYPOINT ["/usr/local/bin/nvtop-web"]
