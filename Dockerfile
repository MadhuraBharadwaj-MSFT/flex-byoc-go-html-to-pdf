FROM golang:1.26-bookworm@sha256:5de7c99eb78ca5e515a4c340235d0955ec5964a5071c492c15503d7c267da473 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go test -count=1 ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -mod=readonly -trimpath -ldflags="-s -w" -o /out/app .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go test -mod=readonly -c -o /out/pdf.test .

FROM debian:bookworm-slim@sha256:7ff6ba31527b9e336162fa061746c366c4b2032f4d2cff2078bea43f78efe3f8 AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        chromium-headless-shell \
        chromium-sandbox \
        fontconfig \
        fonts-liberation \
        fonts-noto-color-emoji \
        fonts-noto-core \
    && rm -rf /var/lib/apt/lists/*

RUN groupadd --gid 10001 app \
    && useradd --uid 10001 --gid 10001 --create-home \
        --home-dir /home/app --shell /usr/sbin/nologin app \
    && mkdir -p /home/site/wwwroot /home/app/.cache /home/app/.config \
    && chown -R 10001:10001 /home/app /home/site/wwwroot

ENV FUNCTIONS_GO_DISABLE_HTTP_PROXY=1 \
    CHROMIUM_PATH=/usr/bin/chromium-headless-shell \
    HOME=/home/app \
    XDG_CACHE_HOME=/home/app/.cache \
    XDG_CONFIG_HOME=/home/app/.config

WORKDIR /home/site/wwwroot

FROM runtime AS integration

COPY --from=build --chmod=0555 /out/pdf.test /tmp/pdf.test
USER 10001:10001
ENTRYPOINT ["/tmp/pdf.test"]

FROM runtime AS final

COPY --from=build --chmod=0555 /out/app ./app
COPY host.json ./
USER 10001:10001
ENTRYPOINT ["/home/site/wwwroot/app"]
