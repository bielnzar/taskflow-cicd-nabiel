# Laporan CI/CD TaskFlow API

## 1. Identitas dan Scope

### Anggota Kelompok

| Nama | NRP |
| --- | --- |
| Nathan Kho Pancras | 5027231002 |
| Daffa Rajendra Priyatama | 5027231009 |
| Maulana Ahmad Zahiri | 5027231010 |
| Kevin Anugerah Faza | 5027231027 |
| RM. Novian Malcolm Bayuputra | 5027231035 |
| Azza Farichi Tjahjono | 5027231071 |
| Nabiel Nizar Anwari | 5027231087 |

### Scope Pengerjaan

Project ini mengimplementasikan pipeline CI/CD untuk aplikasi TaskFlow API. Tool utama yang digunakan adalah GitHub Actions dengan fokus khusus pada matrix testing Go 1.21, Go 1.22, dan Go 1.23.

Laporan ini menjelaskan isi repository secara menyeluruh, mulai dari arsitektur aplikasi, database, testing, pipeline CI/CD, Docker image, smoke test, rollback, sampai security scanning.

## 2. Ringkasan Project

TaskFlow API adalah backend sederhana untuk manajemen task. Aplikasi ditulis dengan bahasa Go dan menggunakan `net/http` dari standard library sebagai HTTP server. Dependency eksternal utama adalah `github.com/jackc/pgx/v5` untuk koneksi PostgreSQL.

Endpoint utama aplikasi:

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/health` | Mengecek status aplikasi |
| GET | `/api/v1/tasks` | Mengambil semua task, bisa difilter berdasarkan status |
| POST | `/api/v1/tasks` | Membuat task baru |
| GET | `/api/v1/tasks/{id}` | Mengambil task berdasarkan ID |
| PUT | `/api/v1/tasks/{id}` | Mengubah task |
| DELETE | `/api/v1/tasks/{id}` | Menghapus task |
| GET | `/api/v1/stats` | Mengambil statistik task |

Aplikasi memiliki dua mode storage:

| Mode | Kondisi | Implementasi |
| --- | --- | --- |
| In-memory | `DATABASE_URL` kosong | `MemoryRepository` |
| PostgreSQL | `DATABASE_URL` terisi | `PostgresRepository` |

Mode in-memory berguna untuk development cepat dan unit test. Mode PostgreSQL digunakan untuk integrasi, staging, dan production-like environment.

## 3. Arsitektur Aplikasi

TaskFlow API memakai layered architecture. Setiap layer punya tanggung jawab yang jelas agar kode mudah dites dan mudah dikembangkan.

```text
HTTP Request
    |
    v
Handler
    |
    v
Service
    |
    v
Repository Interface
    |
    +--> MemoryRepository
    |
    +--> PostgresRepository
             |
             v
         PostgreSQL
```

### Struktur Folder Penting

| Path | Fungsi |
| --- | --- |
| `cmd/server/main.go` | Entry point aplikasi, membaca environment, memilih repository, dan menjalankan server |
| `internal/handler/handler.go` | HTTP layer, routing, parsing request, dan response JSON |
| `internal/service/service.go` | Business logic seperti create, update, delete, stats, dan completion rate |
| `internal/repository/repository.go` | Interface storage agar service tidak bergantung langsung ke database tertentu |
| `internal/repository/memory.go` | Implementasi repository berbasis memory dan thread-safe dengan mutex |
| `internal/repository/postgres.go` | Implementasi repository PostgreSQL memakai `pgx/v5` |
| `internal/model/task.go` | Definisi struct, enum status, enum priority, dan response shape |
| `internal/validator/validator.go` | Validasi input seperti status, priority, empty string, dan panjang title |
| `migrations/001_create_tasks.sql` | Skema database task |

### Alur Create Task

```text
Client
  |
  | POST /api/v1/tasks
  v
Handler
  |
  | decode JSON -> CreateTaskRequest
  v
Service
  |
  | validate title + priority
  | set ID, status, priority, timestamp
  v
Repository
  |
  +--> MemoryRepository
  |
  +--> PostgresRepository
  |
  v
Response 201 Created
```

1. Client mengirim `POST /api/v1/tasks`.
2. Handler membaca JSON request ke `CreateTaskRequest`.
3. Service memvalidasi `title` dan `priority`.
4. Service membuat ID, status default `todo`, priority default `medium`, dan timestamp.
5. Repository menyimpan task ke memory atau PostgreSQL.
6. Handler mengembalikan response `201 Created`.

### Alur Update Task

```text
Client
  |
  | PUT /api/v1/tasks/{id}
  v
Handler
  |
  | extract id + decode JSON
  v
Service
  |
  | find existing task
  | validate provided fields
  | set completed_at if status == done
  v
Repository
  |
  | save updated task
  v
Handler
  |
  v
Response 200 OK
```

1. Client mengirim `PUT /api/v1/tasks/{id}`.
2. Handler mengambil ID dari URL dan membaca JSON body.
3. Service mencari task lama.
4. Service memvalidasi field yang dikirim.
5. Jika status berubah menjadi `done`, field `completed_at` diisi.
6. Repository menyimpan hasil update.
7. Handler mengembalikan task terbaru.

## 4. Database dan Migrasi

Database yang digunakan adalah PostgreSQL. Skema utama berada di `migrations/001_create_tasks.sql`.

Tabel `tasks` memiliki kolom utama:

| Kolom | Tipe | Fungsi |
| --- | --- | --- |
| `id` | `VARCHAR(64)` | Primary key task |
| `title` | `VARCHAR(200)` | Judul task, wajib diisi |
| `description` | `TEXT` | Deskripsi task |
| `priority` | `VARCHAR(20)` | Prioritas task |
| `status` | `VARCHAR(20)` | Status task |
| `created_at` | `TIMESTAMPTZ` | Waktu task dibuat |
| `updated_at` | `TIMESTAMPTZ` | Waktu task terakhir diubah |
| `completed_at` | `TIMESTAMPTZ` | Waktu task selesai |

Constraint penting:

```sql
CHECK (priority IN ('low', 'medium', 'high'))
CHECK (status IN ('todo', 'in_progress', 'done'))
```

Index yang dibuat:

```sql
CREATE INDEX IF NOT EXISTS idx_tasks_status   ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
CREATE INDEX IF NOT EXISTS idx_tasks_created  ON tasks(created_at DESC);
```

Index `status` membantu query filter task berdasarkan status. Index `priority` membantu query berdasarkan prioritas jika nanti diperlukan. Index `created_at DESC` mendukung pengurutan task terbaru lebih efisien.

### Setup Database Lokal

```text
PostgreSQL Local
  |
  | create user taskflow
  | create database taskflow
  v
Database taskflow
  |
  | run migrations/001_create_tasks.sql
  v
Table tasks
  |
  | verify schema
  v
Ready for integration test
```

Jika PostgreSQL lokal sudah berjalan di `localhost:5432`, buat user dan database:

```sql
CREATE USER taskflow WITH PASSWORD 'taskflow_secret';
CREATE DATABASE taskflow OWNER taskflow;
GRANT ALL PRIVILEGES ON DATABASE taskflow TO taskflow;
```

Jika user sudah ada tetapi password belum sesuai:

```sql
ALTER USER taskflow WITH PASSWORD 'taskflow_secret';
```

Jalankan migrasi:

```powershell
$env:PGPASSWORD="taskflow_secret"
psql -h localhost -U taskflow -d taskflow -f .\migrations\001_create_tasks.sql
psql -h localhost -U taskflow -d taskflow -c "\dt"
psql -h localhost -U taskflow -d taskflow -c "\d tasks"
```

Environment aplikasi:

```env
DATABASE_URL=postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable
PORT=8080
```

File `.env` tidak boleh dicommit karena berisi konfigurasi lokal.

## 5. Bug Fix dan Test

Repository ini berisi skenario pembelajaran CI/CD dengan bug yang harus terdeteksi oleh test. Saat diverifikasi, implementasi bug utama sudah dalam kondisi benar.

| Bug | Lokasi | Masalah | Dampak | Test yang Mendeteksi | Status |
| --- | --- | --- | --- | --- | --- |
| Bug 1 | `internal/service/service.go` | Perhitungan completion rate rawan salah jika memakai integer division | Persentase task selesai bisa menjadi `0` walaupun ada task selesai | `TestCalculateCompletionRate`, `TestGetStats_CompletionRate` | Sudah benar memakai `float64` |
| Bug 2 | `internal/repository/memory.go`, `internal/repository/postgres.go` | Filter status bisa terbalik jika memakai `!=` | Filter `done` dapat mengembalikan task yang bukan `done` | `TestFindByStatus_HanyaTodo`, `TestGetAll_WithStatusFilter`, integration test Postgres | Sudah benar memakai equality filter |
| Bug 3 | `internal/validator/validator.go` | Priority `urgent` tidak boleh valid | Data tidak sesuai constraint database | `TestIsValidPriority` | Sudah benar, hanya `low`, `medium`, `high` yang valid |

### Test Tambahan

Beberapa test tambahan yang memperkuat coverage dan validasi perilaku:

| Test | Tujuan |
| --- | --- |
| `TestGetStats_CompletionRate` | Memastikan statistik completion rate konsisten |
| `TestGetAll_WithStatusFilter` | Memastikan filter status mengembalikan data yang tepat |
| `TestCreate_WithUnicodeTitle` | Memastikan title unicode dapat diproses |
| `TestDelete_AndVerifyStats` | Memastikan delete mempengaruhi statistik |
| `TestStats_ConsistencyWithTaskList` | Memastikan total stats sama dengan total list |
| `TestCreateMultipleTasks_UniqueIDs` | Memastikan ID task unik |
| `TestPostgresRepository_WithFakePool` | Menguji repository Postgres tanpa database nyata |

### Hasil Verifikasi Lokal

Verifikasi lokal dijalankan dengan command berikut:

```powershell
go vet ./...
go test ./...
go test -race ./...
$env:DATABASE_URL="postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable"
go test -tags=integration -race ./... -timeout 60s
```

Command `go vet`, unit test, race test, dan integration test berhasil dijalankan. Integration test memakai PostgreSQL lokal melalui `DATABASE_URL`.

Coverage dijalankan dengan command:

```powershell
$cover = "$env:TEMP\taskflow-coverage.out"
go test ./... -coverprofile="$cover" -covermode=atomic
go tool cover -func "$cover"
```

Hasil coverage total adalah 78.9 persen. Nilai ini memenuhi target minimum 75 persen.

## 6. Workflow CI GitHub Actions

Workflow utama berada di `.github/workflows/ci-cd.yml`.

Trigger pipeline:

```yaml
on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]
```

Artinya pipeline otomatis berjalan setiap ada push ke `main` atau `develop`, serta setiap pull request ke branch tersebut.

```text
Developer Push / Pull Request
  |
  v
GitHub Actions Trigger
  |
  v
CI Matrix
  |
  +--> Go 1.21
  +--> Go 1.22
  +--> Go 1.23
  |
  v
Security Scan
  |
  v
CD on main push
  |
  v
Smoke Test
  |
  v
Stable Tag
```

### Matrix Testing

Job CI menggunakan matrix:

```yaml
matrix:
  go-version: ["1.21", "1.22", "1.23"]
```

Tujuannya adalah memastikan kode tetap bisa diuji di beberapa versi Go. Ini sesuai dengan bagian penugasan kelompok, yaitu GitHub Actions dengan matrix testing Go 1.21, 1.22, dan 1.23.

### PostgreSQL Service Container

CI menyediakan PostgreSQL service container:

```yaml
services:
  postgres:
    image: postgres:16-alpine
    env:
      POSTGRES_USER: taskflow
      POSTGRES_PASSWORD: taskflow_secret
      POSTGRES_DB: taskflow
```

Service ini dipakai untuk integration test:

```yaml
DATABASE_URL: postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable
```

### Tahapan CI

```text
Checkout
  |
  v
Setup Go
  |
  v
go vet
  |
  v
go test -race
  |
  v
Build Binary
  |
  v
Integration Test + PostgreSQL
  |
  v
Coverage Gate >= 75%
  |
  v
Upload Artifacts
```

| Tahap | Command | Tujuan |
| --- | --- | --- |
| Setup Go | `actions/setup-go@v5` | Menyiapkan Go sesuai versi matrix |
| Vet | `go vet ./...` | Analisis statis bawaan Go |
| Unit test race | `go test -race ./... -timeout 30s` | Menjalankan test dan mendeteksi race condition |
| Build check | `go build ./cmd/server` | Memastikan aplikasi bisa dikompilasi |
| Integration test | `go test -tags=integration -race ./...` | Menguji integrasi dengan PostgreSQL |
| Coverage check | `go test ./... -coverprofile=coverage.out` | Mengukur coverage |
| Coverage gate | `coverage >= 75` | Memblokir pipeline jika coverage kurang |
| Artifact upload | `actions/upload-artifact@v4` | Menyimpan coverage report dan binary |

Pipeline akan gagal jika vet gagal, test gagal, race detector menemukan masalah, integration test gagal, build gagal, atau coverage berada di bawah 75 persen.

## 7. Security Scan Pipeline

Security scan ditambahkan sebagai job terpisah bernama `Security Scan`. Job ini berjalan setelah CI selesai:

```text
CI Success
  |
  v
Security Scan
  |
  +--> govulncheck
  |       |
  |       v
  |   govulncheck-report.json
  |
  +--> gosec
          |
          v
      gosec-report.json
  |
  v
Upload Security Artifacts
  |
  v
Fail pipeline if blocking findings exist
```

```yaml
security:
  name: Security Scan
  runs-on: ubuntu-latest
  needs: ci
```

CD dibuat bergantung pada CI dan security:

```yaml
needs: [ci, security]
```

Dengan konfigurasi ini, deployment tidak berjalan jika security job gagal.

### SCA dengan govulncheck

SCA atau Software Composition Analysis digunakan untuk memeriksa vulnerability pada dependency Go.

Tool yang digunakan:

```bash
govulncheck
```

Command di pipeline:

```yaml
govulncheck -json ./... > govulncheck-report.json
```

Alasan memilih `govulncheck`:

- Tool resmi dari ekosistem Go.
- Menganalisis dependency berdasarkan kode Go.
- Menghasilkan output JSON yang bisa disimpan sebagai artifact.

Artifact:

```text
govulncheck-report
```

### Analisis Hasil govulncheck

Scan lokal `govulncheck` menghasilkan report JSON dan menemukan beberapa vulnerability pada dependency serta toolchain yang digunakan saat scan. 

Berikut adalah beberapa istilah yang perlu untuk diketahui dalam pembacaan hasil govulncheck :

| Istilah | Arti |
| --- | --- |
| Reachable finding | Vulnerability memiliki trace dari dependency/runtime sampai ke fungsi aplikasi |
| Dependency-level finding | Vulnerability ada pada dependency graph, tetapi trace langsung ke business logic aplikasi tidak terlihat pada potongan report yang diperiksa |
| No finding | Tool tidak menemukan masalah pada area tersebut |

Berikut adalah temuan govulncheck :

| Area | Contoh ID | Analisis | Rekomendasi |
| --- | --- | --- | --- |
| `github.com/jackc/pgx/v5 v5.6.0` | `GO-2026-4771`, `GO-2026-4772` | Reachable dependency finding karena trace melewati fungsi aplikasi seperti `NewPostgresRepository` dan `FindByStatus` | Update `pgx/v5` minimal ke versi fixed `v5.9.0` setelah diuji |
| Go standard library lokal `v1.26.2` | `GO-2026-4971`, `GO-2026-4918`, `GO-2026-4981` | Reachable runtime/toolchain finding karena beberapa trace melewati `net/http`, koneksi database, dan `main` | Gunakan Go patch version yang sudah fixed, misalnya `v1.26.3` atau versi stabil sesuai CI |
| `golang.org/x/crypto v0.17.0` | `GO-2024-3321`, `GO-2025-3487`, `GO-2025-4116` | Dependency-level finding karena module ada di dependency graph, tetapi tidak terlihat sebagai bug langsung di business logic aplikasi | Update dependency transitif melalui `go get -u` secara terkontrol |

Kesimpulan `govulncheck`: temuan yang paling perlu diprioritaskan adalah reachable finding pada `pgx/v5` dan toolchain Go. Temuan `x/crypto` tetap dicatat sebagai risiko dependency-level, tetapi tidak diklaim sebagai bug langsung pada kode aplikasi. Tidak ada indikasi SQL injection atau kebocoran secret dari hasil `govulncheck`.

**Rekomendasi**: membuat branch khusus dependency update, memperbarui `pgx/v5` dan dependency transitif, lalu menjalankan ulang `go test`, integration test, `govulncheck`, dan `gosec`.

### SAST dengan gosec

SAST atau Static Application Security Testing digunakan untuk mencari pola kode yang berisiko secara keamanan.

Tool yang digunakan:

```bash
gosec
```

Command di pipeline:

```yaml
gosec -severity high -confidence medium -fmt json -out gosec-report.json ./...
```

Rule yang relevan untuk aplikasi Go dan PostgreSQL:

| Rule | Risiko |
| --- | --- |
| G101 | Hardcoded credential |
| G201/G202 | SQL injection |
| G304 | File path traversal |
| G501 | Weak crypto |

Aplikasi ini memakai query parameter seperti `$1`, bukan string concatenation, sehingga risiko SQL injection lebih rendah.

Artifact:

```text
gosec-report
```

### Analisis Hasil gosec

Hasil `gosec-report.json`:

```json
{
  "Issues": [],
  "Stats": {
    "files": 8,
    "lines": 837,
    "nosec": 0,
    "found": 0
  }
}
```

Analisis:

| Aspek | Hasil |
| --- | --- |
| Jumlah file yang discan | 8 file Go |
| Jumlah baris yang discan | 837 baris |
| Jumlah issue | 0 |
| Penggunaan `nosec` | 0 |
| Reachable finding | Tidak ada |
| Dependency-level finding | Tidak ada |

Kesimpulan SAST: tidak ada temuan keamanan dari `gosec` pada kode Go saat scan dijalankan. Query database juga memakai parameter SQL seperti `$1`, sehingga pola SQL injection berbasis string concatenation tidak terdeteksi.

### Fail Gate Security

Step `govulncheck` dan `gosec` memakai `continue-on-error: true` agar artifact tetap terupload walaupun ada temuan. Setelah upload artifact, ada step khusus yang menggagalkan job jika salah satu scan gagal:

```yaml
if: steps.govulncheck.outcome == 'failure' || steps.gosec.outcome == 'failure'
```

Dengan pola ini, pipeline tetap menyimpan report untuk audit, tetapi deployment tetap diblokir jika ada temuan yang memblokir.

### Secret Scanning Lokal

Secret scanning lokal dilakukan dengan `gitleaks`.

```text
Developer stages files
  |
  v
gitleaks protect --staged --redact
  |
  +--> leak found
  |       |
  |       v
  |   commit blocked
  |
  +--> no leaks found
          |
          v
      commit allowed
```

Command:

```bash
gitleaks protect --staged --redact
```

Hasil verifikasi lokal menunjukkan tidak ada secret yang ditemukan pada staged files.

Pre-commit hook lokal juga disiapkan di `.git/hooks/pre-commit`. Hook ini tidak ikut Git karena folder `.git` bukan bagian dari repository. Tujuannya adalah mencegah file yang mengandung secret terlanjur dicommit dari mesin developer.

Catatan penting:

- `.env` tidak dicommit.
- `govulncheck-report.json` dan `gosec-report.json` lokal di-ignore.
- Artifact resmi tetap berasal dari GitHub Actions.

## 8. CD, Docker, dan Registry

CD berjalan hanya pada push ke `main`:

```text
CI Success
  |
  v
Security Success
  |
  v
Docker Buildx
  |
  v
Build Image from Dockerfile
  |
  v
Tag image as sha-<commit>
  |
  v
Push image to GHCR
```

```yaml
if: github.ref == 'refs/heads/main' && github.event_name == 'push'
```

Job CD membutuhkan CI dan security:

```yaml
needs: [ci, security]
```

### Dockerfile Multi-stage

Dockerfile memakai dua stage:

```text
Source Code
  |
  v
Builder Stage: golang:1.22-alpine
  |
  | go mod download
  | go vet ./...
  | go build static binary
  v
Runtime Stage: scratch
  |
  | copy binary + CA certificates
  v
Small runtime image
```

| Stage | Base Image | Fungsi |
| --- | --- | --- |
| Builder | `golang:1.22-alpine` | Download dependency, vet, dan build binary |
| Runtime | `scratch` | Menjalankan binary final |

Keuntungan multi-stage build:

- Runtime image lebih kecil.
- Tidak membawa compiler Go ke production image.
- Surface area vulnerability lebih kecil karena runtime memakai `scratch`.
- Cocok untuk aplikasi Go yang bisa dikompilasi menjadi static binary.

Build command di Dockerfile:

```dockerfile
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o taskflow-api ./cmd/server
```

### Image Tagging

CD membuat image dengan tag commit SHA:

```text
ghcr.io/<owner>/<repo>:sha-<short-sha>
```

Tag berbasis SHA membuat image dapat dilacak ke commit tertentu. Ini lebih aman daripada hanya mengandalkan `latest`, karena `latest` tidak menjelaskan versi kode yang sebenarnya berjalan.

### Push ke Registry

Workflow login ke GHCR memakai `GITHUB_TOKEN`:

```yaml
uses: docker/login-action@v3
```

Image dibuild dan dipush dengan:

```yaml
uses: docker/build-push-action@v5
```

## 9. Smoke Test Deployment

Setelah image dipush, pipeline menjalankan container untuk smoke test. Tujuannya adalah memastikan image yang baru dibuat benar-benar bisa hidup dan endpoint utama tetap merespons.

```text
Image pushed to GHCR
  |
  v
Pull sha-<commit> image
  |
  v
Run taskflow-api container
  |
  v
Wait until /health is ready
  |
  +--> timeout / container crash
  |       |
  |       v
  |   pipeline failed
  |
  +--> ready
          |
          v
      curl /health
          |
          v
      curl /api/v1/stats
          |
          v
      cleanup container
```

Langkah smoke test:

1. Pull image dengan tag SHA.
2. Jalankan container `taskflow-api`.
3. Tunggu server siap maksimal 30 detik.
4. Cek `/health`.
5. Cek `/api/v1/stats`.
6. Cleanup container.

Endpoint yang diuji:

```bash
curl -sf http://localhost:8080/health
curl -sf http://localhost:8080/api/v1/stats
```

Jika salah satu endpoint gagal, pipeline akan gagal. Ini penting karena build sukses belum tentu aplikasi bisa berjalan dengan benar.

## 10. Stable Tag dan Rollback

Setelah CD sukses, job `tag-stable` berjalan untuk memberi tag `stable` pada image yang sudah lolos pipeline.

Kondisi job:

```yaml
if: github.ref == 'refs/heads/main' && github.event_name == 'push' && needs.cd.result == 'success'
```

Artinya tag `stable` hanya diperbarui jika:

- Push terjadi ke `main`.
- CD selesai dengan sukses.
- Smoke test tidak gagal.

### Strategi Rollback

Rollback disediakan lewat target Makefile:

```text
Incident detected
  |
  v
Choose rollback tag
  |
  v
make rollback ROLLBACK_TAG=sha-xxxxx
  |
  v
docker pull selected image
  |
  v
stop old taskflow-api container
  |
  v
run replacement container
  |
  v
curl /health
  |
  +--> health failed -> rollback failed
  |
  +--> health ok -> rollback complete
```

```bash
make rollback ROLLBACK_TAG=sha-a3f2c1d
```

Alur rollback:

1. Tentukan tag image yang aman, misalnya `sha-xxxxx` atau `stable`.
2. Pull image dari registry.
3. Stop container lama.
4. Jalankan container baru dari tag rollback.
5. Verifikasi `/health`.

Potongan penting Makefile:

```makefile
rollback:
	@test -n "$(ROLLBACK_TAG)" || (echo "Set ROLLBACK_TAG=sha-xxxxx"; exit 1)
	docker pull $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	docker stop taskflow-api 2>/dev/null || true
	docker run -d --rm \
	  --name taskflow-api \
	  -p 8080:8080 \
	  -e DATABASE_URL=$(DB_URL) \
	  $(REGISTRY)/$(IMAGE):$(ROLLBACK_TAG)
	curl -sf http://localhost:8080/health || (echo "Health check failed!"; exit 1)
```

Prosedur lengkap rollback ditulis di `ROLLBACK_PROCEDURE.md`.

## 11. Bukti Verifikasi

Bukti verifikasi yang sudah tersedia di repository saat laporan ini dibuat adalah hasil coverage lokal, secret scanning lokal, dan GitHub Actions green.

### GitHub Actions Green

Gambar berikut menunjukkan pipeline GitHub Actions berhasil. Job matrix Go 1.21, Go 1.22, Go 1.23, `Security Scan`, `CD - Docker Build & Push`, dan `Tag Stable` berada dalam status sukses.

![GitHub Actions green](img/actions-green.png)

### Coverage Lokal

Gambar berikut menunjukkan output coverage lokal. Total coverage adalah 78.9 persen, sehingga melewati batas minimum 75 persen.

![Hasil coverage lokal](img/local-coverage.png)

### Secret Scanning Lokal

Gambar berikut menunjukkan hasil `gitleaks protect --staged --redact`. Hasilnya `no leaks found`, sehingga staged changes tidak mengandung secret yang terdeteksi.

![Hasil gitleaks lokal](img/local-gitleaks.png)

## 12. Kesimpulan

Pipeline CI/CD TaskFlow API sudah mencakup kebutuhan utama penugasan:

- CI otomatis berjalan pada push dan pull request.
- Matrix testing berjalan untuk Go 1.21, 1.22, dan 1.23.
- Unit test, race test, integration test, build check, dan coverage gate dijalankan otomatis.
- Coverage total lokal mencapai 78.9 persen dan memenuhi target minimal 75 persen.
- Security scan ditambahkan melalui `govulncheck` untuk SCA dan `gosec` untuk SAST.
- Artifact coverage dan security report disimpan oleh GitHub Actions.
- Docker image dibuild dengan multi-stage build dan runtime `scratch`.
- Image diberi tag berbasis commit SHA agar mudah dilacak.
- Smoke test memastikan image yang dideploy tetap bisa melayani endpoint utama.
- Tag `stable` hanya diperbarui setelah pipeline dan smoke test sukses.
- Rollback tersedia melalui Makefile dan didokumentasikan.

Dengan rancangan ini, risiko utama seperti bug tidak terdeteksi, binary tidak terlacak, deployment gagal tanpa diketahui, dan kurangnya audit keamanan dapat dikurangi secara sistematis.