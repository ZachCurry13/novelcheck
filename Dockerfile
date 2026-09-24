# syntax=docker/dockerfile:1
# ---- build ----
# Cross-compiles on the build machine's native arch (fast multi-arch builds).
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH=amd64
# Version shown in the app and used for update checks (set by CI).
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Pure-Go SQLite driver: no CGO, fully static binary.
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags="-s -w -X github.com/zachcurry13/novelcheck/internal/version.Version=${VERSION}" \
    -o /out/novelcheck ./cmd/novelcheck

# ---- runtime ----
FROM alpine:3.22
LABEL org.opencontainers.image.source="https://github.com/ZachCurry13/novelcheck" \
      org.opencontainers.image.description="NovelCheck: self-hosted e-book content checker"
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S -g 568 novelcheck && adduser -S -u 568 -G novelcheck novelcheck \
 && mkdir -p /data /calibre && chown novelcheck:novelcheck /data
COPY --from=build /out/novelcheck /usr/local/bin/novelcheck
USER 568:568
ENV NOVELCHECK_ADDR=:8080 \
    NOVELCHECK_DATA_DIR=/data \
    NOVELCHECK_CALIBRE_DIR=/calibre
VOLUME ["/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/novelcheck"]
