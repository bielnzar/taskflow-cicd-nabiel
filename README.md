# TaskFlow API — Source Code PBL CI/CD

Proyek Go ini adalah **source code untuk Problem-Based Learning** mata kuliah
Operasional Pengembang (DevOps), Pertemuan 9: CI/CD Pipeline.

## Quick Start

```bash
# Cara 1: Full stack dengan Docker Compose (tidak perlu Go terinstall)
docker compose up -d
curl http://localhost:8080/health

# Cara 2: Development lokal (butuh Go 1.22+)
cp .env.example .env          # edit DATABASE_URL jika perlu
make db-up                    # start postgres
make test                     # unit test (tanpa DB)
make test-integration         # integration test (butuh DB aktif)
make build                    # compile binary
./bin/taskflow-api
```

## Makefile Targets

| Target | Keterangan |
|--------|------------|
| `make vet` | Analisis statis `go vet` |
| `make test` | Unit test (tanpa database) |
| `make test-race` | Unit test + race detector |
| `make test-cover` | Coverage report |
| `make test-integration` | Integration test (butuh `DATABASE_URL`) |
| `make build` | Compile binary ke `bin/taskflow-api` |
| `make docker-build` | Multi-stage Docker image |
| `make docker-push` | Push image ke registry |
| `make docker-stable` | Tag image sebagai `:stable` |
| `make rollback ROLLBACK_TAG=sha-xxxxx` | Rollback ke versi tertentu |
| `make db-up` | Start postgres via Docker Compose |
| `make up` | Start full stack (postgres + app) |

## API Endpoints

| Method | Path | Keterangan |
|--------|------|------------|
| GET | `/health` | Health check |
| GET | `/api/v1/tasks` | List task (`?status=todo\|in_progress\|done`) |
| POST | `/api/v1/tasks` | Buat task baru |
| GET | `/api/v1/tasks/{id}` | Ambil task |
| PUT | `/api/v1/tasks/{id}` | Update task |
| DELETE | `/api/v1/tasks/{id}` | Hapus task |
| GET | `/api/v1/stats` | Statistik |

## Environment Variables

| Variable | Default | Keterangan |
|----------|---------|------------|
| `DATABASE_URL` | *(kosong)* | Jika tidak di-set → pakai MemoryRepository |
| `PORT` | `8080` | Port server |

## Arsitektur

```
cmd/server/main.go          ← Entry point
internal/
  handler/handler.go        ← HTTP layer (Go 1.22 routing)
  service/service.go        ← Business logic
  repository/
    repository.go           ← Interface
    memory.go               ← In-memory (unit test)
    postgres.go             ← PostgreSQL via pgx/v5 (production)
  model/task.go             ← Struct & types
  validator/validator.go    ← Input validation
migrations/001_create_tasks.sql  ← Skema database
```

> **Catatan untuk mahasiswa**: Kode ini mengandung **3 bug yang disengaja**.
> Lihat `pbl-cicd-problem.md` untuk detail skenario dan instruksi lengkap.
