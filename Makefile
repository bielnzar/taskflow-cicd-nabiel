# Makefile - TaskFlow API
BINARY   = bin/taskflow-api
IMAGE    = taskflow-api
REGISTRY ?= ghcr.io/your-username
VERSION  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
DB_URL   ?= postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable
COVERAGE_MIN ?= 75.0

.PHONY: all vet test test-race test-cover test-integration \
        build docker-build docker-push rollback \
        db-up db-down up clean help

all: vet test build

## go vet - built-in Go static analysis
vet:
	@echo "go vet ./..."
	go vet ./...

## Run unit tests (without database)
test:
	@echo "go test ./..."
	go test ./... -v -timeout 30s

## Run tests with race detector (required in CI)
test-race:
	@echo "go test -race ./..."
	go test ./... -race -timeout 30s

## Generate coverage report and enforce minimum coverage
test-cover:
	@echo "coverage report"
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out
	@COVERAGE=$$(go tool cover -func=coverage.out | awk '/total:/ {gsub("%","",$$3); print $$3}'); \
	echo "coverage total: $$COVERAGE%"; \
	awk -v coverage="$$COVERAGE" -v min="$(COVERAGE_MIN)" 'BEGIN { exit !(coverage >= min) }' || \
		(echo "Coverage $$COVERAGE% is below $(COVERAGE_MIN)%"; exit 1)

## Run integration tests (requires DATABASE_URL / active PostgreSQL)
test-integration:
	@echo "integration test (DATABASE_URL=$(DB_URL))"
	DATABASE_URL=$(DB_URL) go test -tags=integration ./... -v -race -timeout 60s

## Build Linux binary (used by Docker and CI)
build:
	@echo "go build ($(VERSION))"
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build \
		-ldflags="-w -s" \
		-o $(BINARY) ./cmd/server

## Build multi-stage Docker image
docker-build:
	@echo "docker build ($(VERSION))"
	docker build -t $(REGISTRY)/$(IMAGE):sha-$(VERSION) -t $(REGISTRY)/$(IMAGE):latest .
	@docker images $(REGISTRY)/$(IMAGE):sha-$(VERSION) --format "Size: {{.Size}}"

## Push image to registry
docker-push:
	@echo "docker push"
	docker push $(REGISTRY)/$(IMAGE):sha-$(VERSION)
	docker push $(REGISTRY)/$(IMAGE):latest

## Push stable tag (only after smoke test passes)
docker-stable:
	@echo "tag $(VERSION) as stable"
	docker tag $(REGISTRY)/$(IMAGE):sha-$(VERSION) $(REGISTRY)/$(IMAGE):stable
	docker push $(REGISTRY)/$(IMAGE):stable

## Rollback: run a previous image version
## Usage: make rollback ROLLBACK_TAG=sha-a3f2c1d
rollback:
	@test -n "$(ROLLBACK_TAG)" || (echo "Set ROLLBACK_TAG=sha-xxxxx"; exit 1)
	@echo "Rolling back to $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)"
	docker pull $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	docker stop taskflow-api 2>/dev/null || true
	docker run -d --rm \
	  --name taskflow-api \
	  -p 8080:8080 \
	  -e DATABASE_URL=$(DB_URL) \
	  $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	@echo "Waiting for server to be ready..."
	@sleep 5
	curl -sf http://localhost:8080/health || (echo "Health check failed!"; exit 1)
	@echo "Rollback completed to $(ROLLBACK_TAG)"

## Start PostgreSQL only (for development)
db-up:
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 3
	@echo "PostgreSQL is ready at localhost:5432"

## Start full stack (PostgreSQL + app)
up:
	docker compose up -d
	@echo "Stack is running. API: http://localhost:8080/health"

## Stop stack
db-down:
	docker compose down

## Clean build artifacts
clean:
	rm -rf bin/ coverage.out

## Show all targets
help:
	@grep -E '^##' Makefile | sed 's/## /  /'
