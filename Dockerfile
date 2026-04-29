# ── Stage 1: Builder ──────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Cache layer: download dependencies sebelum copy source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Vet sebelum build
RUN go vet ./...

# Compile — CGO_ENABLED=0 untuk static binary, bisa jalan di scratch container
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s" \
    -o taskflow-api \
    ./cmd/server

# ── Stage 2: Runtime (scratch — image ~8MB) ───────────────────────────────────
FROM scratch

# CA certificates untuk koneksi TLS (ke PostgreSQL/external API)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Binary saja — tidak ada OS, shell, atau library lain
COPY --from=builder /build/taskflow-api /taskflow-api

EXPOSE 8080

ENTRYPOINT ["/taskflow-api"]
