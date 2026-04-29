# Makefile — TaskFlow API
BINARY   = bin/taskflow-api
IMAGE    = taskflow-api
REGISTRY ?= ghcr.io/your-username
VERSION  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DB_URL   ?= postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable

.PHONY: all vet test test-race test-cover test-integration \
        build docker-build docker-push rollback \
        db-up db-down up clean help

all: vet test build

## go vet — analisis statis bawaan Go
vet:
	@echo "→ go vet ./..."
	go vet ./...

## Jalankan unit test (tanpa database)
test:
	@echo "→ go test ./..."
	go test ./... -v -timeout 30s

## Jalankan test dengan race detector (WAJIB di CI)
test-race:
	@echo "→ go test -race ./..."
	go test ./... -race -timeout 30s

## Laporan coverage
test-cover:
	@echo "→ coverage report"
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

## Integration test (butuh DATABASE_URL / postgres aktif)
test-integration:
	@echo "→ integration test (DATABASE_URL=$(DB_URL))"
	DATABASE_URL=$(DB_URL) go test -tags=integration ./... -v -race -timeout 60s

## Build binary Linux (dipakai di Docker & CI)
build:
	@echo "→ go build ($(VERSION))"
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build \
		-ldflags="-w -s" \
		-o $(BINARY) ./cmd/server

## Build Docker image multi-stage
docker-build:
	@echo "→ docker build ($(VERSION))"
	docker build -t $(REGISTRY)/$(IMAGE):sha-$(VERSION) -t $(REGISTRY)/$(IMAGE):latest .
	@docker images $(REGISTRY)/$(IMAGE):sha-$(VERSION) --format "Size: {{.Size}}"

## Push image ke registry
docker-push:
	@echo "→ docker push"
	docker push $(REGISTRY)/$(IMAGE):sha-$(VERSION)
	docker push $(REGISTRY)/$(IMAGE):latest

## Push tag stable (hanya setelah smoke test PASS)
docker-stable:
	@echo "→ tag $(VERSION) sebagai stable"
	docker tag $(REGISTRY)/$(IMAGE):sha-$(VERSION) $(REGISTRY)/$(IMAGE):stable
	docker push $(REGISTRY)/$(IMAGE):stable

## Rollback: jalankan image versi sebelumnya
## Penggunaan: make rollback ROLLBACK_TAG=sha-a3f2c1d
rollback:
	@test -n "$(ROLLBACK_TAG)" || (echo "❌ Set ROLLBACK_TAG=sha-xxxxx"; exit 1)
	@echo "→ Rolling back ke $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)"
	docker pull $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	docker stop taskflow-api 2>/dev/null || true
	docker run -d --rm \
	  --name taskflow-api \
	  -p 8080:8080 \
	  -e DATABASE_URL=$(DB_URL) \
	  $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	@echo "⏳ Menunggu server siap..."
	@sleep 5
	curl -sf http://localhost:8080/health || (echo "❌ Health check gagal!"; exit 1)
	@echo "✅ Rollback berhasil ke $(ROLLBACK_TAG)"

## Jalankan postgres saja (untuk development)
db-up:
	docker compose up -d postgres
	@echo "⏳ Menunggu postgres siap..."
	@sleep 3
	@echo "✅ Postgres siap di localhost:5432"

## Jalankan full stack (postgres + app)
up:
	docker compose up -d
	@echo "✅ Stack berjalan. API: http://localhost:8080/health"

## Stop stack
db-down:
	docker compose down

## Bersihkan artifact
clean:
	rm -rf bin/ coverage.out

## Tampilkan semua target
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
