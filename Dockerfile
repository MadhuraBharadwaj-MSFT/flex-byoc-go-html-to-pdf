# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e

ARG GO_IMAGE=golang:1.26-bookworm@sha256:5de7c99eb78ca5e515a4c340235d0955ec5964a5071c492c15503d7c267da473
ARG RUNTIME_IMAGE=debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go test ./...
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/app .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/rendercheck ./cmd/rendercheck

FROM ${RUNTIME_IMAGE} AS runtime-base

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium-headless-shell \
        chromium-sandbox \
        fonts-dejavu-core \
        fonts-liberation \
        fonts-noto-color-emoji \
        fonts-noto-core \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 app \
    && useradd --uid 10001 --gid 10001 --create-home --home-dir /home/site --shell /usr/sbin/nologin app \
    && mkdir -p /home/site/wwwroot /home/site/.cache \
    && chown -R 10001:10001 /home/site

COPY --chmod=0555 docker/entrypoint.sh /usr/local/bin/byoc-entrypoint.sh

ENV AzureWebJobsScriptRoot=/home/site/wwwroot \
    FUNCTIONS_WORKER_RUNTIME=native \
    FUNCTIONS_WORKER_RUNTIME_VERSION=1.0 \
    FUNCTIONS_GO_DISABLE_HTTP_PROXY=1 \
    CHROMIUM_PATH=/usr/bin/chromium-headless-shell \
    HOME=/home/site \
    XDG_CACHE_HOME=/home/site/.cache

WORKDIR /home/site/wwwroot
USER 10001:10001

FROM runtime-base AS validation

COPY --from=build --chmod=0555 /out/rendercheck /home/site/wwwroot/rendercheck
ENTRYPOINT ["/home/site/wwwroot/rendercheck"]

FROM runtime-base AS final

COPY --from=build --chmod=0555 /out/app /home/site/wwwroot/app
ENTRYPOINT ["/usr/local/bin/byoc-entrypoint.sh"]
