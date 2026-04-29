# Problem Based Learning — CI/CD Pipeline Challenge

# TaskFlow Go API

**Mata Kuliah**: Operasional Pengembang (DevOps)
**Pertemuan**: 9 | **Sifat**: Kelompok (8 kelompok, 4–5 orang)
**Waktu**: Sesi kelas + 1 minggu | **Presentasi**: Pertemuan 11

---

## 1. Latar Belakang Masalah

**TaskFlow Inc.** adalah startup yang membangun aplikasi manajemen proyek berbasis web. Backend mereka ditulis dalam Go dan sudah berjalan di server, namun cara kerja tim engineering mereka jauh dari kata modern:

Setiap kali ada perubahan kode, developer harus mengirim pesan di grup WhatsApp: *"Hei guys, jangan push dulu ya, aku mau deploy."* Lalu ia menjalankan `go build` di laptopnya sendiri, meng-copy binary ke server via SCP, SSH ke server, mematikan proses lama, dan menjalankan yang baru — semua dilakukan malam hari saat traffic rendah.

Hasilnya? Dalam dua bulan terakhir tercatat tiga insiden serius:

- **Insiden #1 (3 minggu lalu)**: Developer A push kode pukul 11 malam. Tidak ada yang tahu. Developer B deploy versinya sendiri pukul 2 dini hari. Dua versi berbeda berjalan bergantian. Data klien kacau.
- **Insiden #2 (2 minggu lalu)**: Setelah deploy, filter "tampilkan task selesai" di dashboard klien justru menampilkan task yang *belum* selesai. Butuh 4 jam untuk debugging karena tidak ada test otomatis. Tim kehilangan satu klien besar.
- **Insiden #3 (minggu lalu)**: Developer lupa menjalankan `go build` setelah edit kode. Binary lama yang ter-deploy. Fitur baru yang sudah dijanjikan ke klien tidak muncul.

CTO TaskFlow Inc. baru saja mendapat tekanan dari investor: *"Kalau kalian tidak bisa deploy dengan andal, kami tidak akan melanjutkan pendanaan."*

**Tugas Anda** sebagai tim DevOps konsultan yang baru dikontrak: rancang dan implementasikan sistem CI/CD otomatis untuk repository TaskFlow agar masalah di atas tidak pernah terulang.

---

## 2. Source Code

### Informasi Umum

| Atribut              | Detail                                            |
| -------------------- | ------------------------------------------------- |
| **Bahasa**     | Go 1.22 (standard library + pgx/v5)               |
| **Framework**  | `net/http` standard library (Go 1.22 routing)   |
| **Arsitektur** | Layered: Handler → Service → Repository         |
| **Pattern**    | Repository Pattern (interface + dua implementasi) |
| **Port**       | 8080                                              |

Repository diberikan di direktori `pbl-taskflow-go/`. Struktur lengkapnya:

```
pbl-taskflow-go/
├── cmd/server/main.go              ← Entry point; auto-pilih DB atau Memory
├── internal/
│   ├── model/task.go               ← Struct Task, Request, Response
│   ├── repository/
│   │   ├── repository.go           ← Interface TaskRepository
│   │   ├── memory.go               ← Implementasi in-memory [ada Bug #2]
│   │   ├── memory_test.go          ← Unit test repository
│   │   ├── postgres.go             ← Implementasi PostgreSQL [ada Bug #2]
│   │   └── postgres_test.go        ← Integration test (//go:build integration)
│   ├── service/
│   │   ├── service.go              ← Business logic [ada Bug #1]
│   │   └── service_test.go         ← Unit test service
│   └── validator/
│       ├── validator.go            ← Validasi input [ada Bug #3]
│       └── validator_test.go       ← Unit test validator
├── migrations/001_create_tasks.sql ← Skema tabel (auto-run saat startup)
├── docker-compose.yml              ← Stack lokal: postgres + app
├── .env.example                    ← Template environment variables
├── Dockerfile                      ← Multi-stage build (builder → scratch)
├── Makefile                        ← Target: vet, test, build, docker
└── go.mod                          ← Go 1.22, dependensi: pgx/v5
```

> **Peringatan**: Kode ini sengaja memiliki **3 bug tersembunyi**. Bug ini hanya akan terdeteksi jika pipeline CI berjalan dengan benar. Menemukan dan memperbaikinya adalah bagian dari tugas.

### Database

Aplikasi menggunakan **PostgreSQL 16** sebagai penyimpanan utama.

```sql
-- migrations/001_create_tasks.sql
CREATE TABLE tasks (
    id           VARCHAR(64)  PRIMARY KEY,
    title        VARCHAR(200) NOT NULL,
    description  TEXT         DEFAULT '',
    priority     VARCHAR(20)  CHECK (priority IN ('low','medium','high')),
    status       VARCHAR(20)  CHECK (status IN ('todo','in_progress','done')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ  DEFAULT NULL
);
```

Aplikasi menggunakan **Repository Pattern** dengan dua implementasi yang dapat dipertukarkan:

| Implementasi           | Digunakan Saat                | Keterangan                  |
| ---------------------- | ----------------------------- | --------------------------- |
| `MemoryRepository`   | `DATABASE_URL` tidak di-set | Untuk unit test & dev cepat |
| `PostgresRepository` | `DATABASE_URL` di-set       | Untuk staging & production  |

**Menjalankan lokal:**

```bash
# Cara 1: Stack lengkap (direkomendasikan)
docker compose up -d
curl http://localhost:8080/health

# Cara 2: Binary + postgres terpisah
make db-up
export DATABASE_URL=postgres://taskflow:taskflow_secret@localhost:5432/taskflow?sslmode=disable
make build && ./bin/taskflow-api

# Cara 3: Tanpa DB (data hilang saat restart)
make build && ./bin/taskflow-api
```

---

## 3. API Endpoints

| Method     | Path                   | Deskripsi                                           | Kode Sukses |
| ---------- | ---------------------- | --------------------------------------------------- | ----------- |
| `GET`    | `/health`            | Health check — wajib untuk smoke test              | `200`     |
| `GET`    | `/api/v1/tasks`      | List semua task (`?status=todo\|in_progress\|done`) | `200`     |
| `POST`   | `/api/v1/tasks`      | Buat task baru                                      | `201`     |
| `GET`    | `/api/v1/tasks/{id}` | Ambil task berdasarkan ID                           | `200`     |
| `PUT`    | `/api/v1/tasks/{id}` | Update task (status/title/description)              | `200`     |
| `DELETE` | `/api/v1/tasks/{id}` | Hapus task                                          | `200`     |
| `GET`    | `/api/v1/stats`      | Statistik task (total, by_status, completion_rate)  | `200`     |

**Contoh request:**

```bash
# Buat task
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{"title":"Deploy pipeline CI/CD","priority":"high"}'

# Filter task yang sudah selesai
curl "http://localhost:8080/api/v1/tasks?status=done"

# Lihat statistik
curl http://localhost:8080/api/v1/stats
```

---

## 4. Skenario yang Harus Diatasi

Setiap kelompok mendapat tool CI/CD yang berbeda namun menghadapi **skenario yang sama**. Baca setiap skenario dengan seksama — skenario ini menggambarkan masalah nyata yang harus Anda selesaikan.

### Pembagian Kelompok & Tools

| Kel.        | Tool CI/CD          | Platform      | Fokus Khusus                                 |
| ----------- | ------------------- | ------------- | -------------------------------------------- |
| **1** | GitHub Actions      | github.com    | Multi-job dengan dependency graph            |
| **2** | GitHub Actions      | github.com    | Matrix testing (Go 1.21, 1.22, 1.23)         |
| **3** | GitLab CI/CD        | gitlab.com    | Stages + GitLab Container Registry           |
| **4** | GitLab CI/CD        | gitlab.com    | Environments: staging (auto) + prod (manual) |
| **5** | Jenkins             | Docker lokal  | Declarative Jenkinsfile                      |
| **6** | CircleCI            | circleci.com  | Go orb + workspace                           |
| **7** | Drone CI            | Gitea + Drone | `.drone.yml` + Docker plugin               |
| **8** | Bitbucket Pipelines | bitbucket.org | Parallel steps                               |

---

### Skenario 1 — "Kode Rusak yang Tidak Ada yang Tahu"

**Situasi:**
Senin pagi, Anda tiba di kantor TaskFlow dan mendapati email dari klien: *"Fitur filter task di aplikasi kalian terbalik! Saya klik 'Tampilkan yang Selesai', yang muncul justru yang belum selesai."*

Tim debugging selama 3 jam. Ternyata ada bug di fungsi filter repository — kondisinya menggunakan `!=` padahal seharusnya `==`. Bug ini sudah ada sejak dua sprint lalu namun tidak ada yang menyadarinya karena tidak ada pengujian otomatis.

**Yang harus Anda lakukan:**

Jalankan test yang sudah disediakan di repository:

```bash
go test ./... -v
```

Amati output-nya. Beberapa test akan **gagal** — itulah cara pipeline CI seharusnya mendeteksi bug sebelum kode masuk ke branch utama.

Tugas Anda:

1. Identifikasi **ketiga bug** dalam kode (petunjuk: lihat komentar `// BUG` di source code)
2. Perbaiki ketiga bug tersebut
3. Pastikan **semua test** lulus setelah perbaikan
4. Pastikan **race condition test** juga lulus: `go test -race ./...`
5. Tambahkan **minimal 2 test case baru** yang belum ada untuk memperkuat coverage
6. Pastikan coverage keseluruhan minimal **75%**:
   ```bash
   go test ./... -coverprofile=cov.out && go tool cover -func=cov.out
   ```

**Output yang diharapkan:** Semua test PASS, coverage ≥ 75%, tidak ada race condition.

---

### Skenario 2 — "Pipeline yang Selalu Hijau"

**Situasi:**
Setelah Skenario 1 diselesaikan, CTO TaskFlow meminta jaminan: *"Saya tidak mau lagi ada bug yang lolos ke production. Apapun yang di-push ke branch `main` atau `develop`, harus otomatis diuji."*

Ia juga menambahkan: *"Dan saya mau tahu kalau ada developer yang push kode tanpa menjalankan vet. Saya sudah dapat laporan bahwa ada beberapa fungsi yang tidak digunakan."*

**Yang harus Anda lakukan:**

Buat file konfigurasi CI pipeline sesuai tool kelompok Anda (`.github/workflows/`, `.gitlab-ci.yml`, `Jenkinsfile`, dll). Pipeline harus:

1. **Trigger otomatis** setiap ada `push` ke branch `main`/`develop` dan setiap `pull request`
2. **Jalankan `go vet`** — pipeline harus **gagal** jika ditemukan error analisis statis
3. **Jalankan unit test** dengan race detector: `go test -race ./...`
4. **Jalankan integration test** (membutuhkan PostgreSQL sebagai service container):
   ```bash
   go test -tags=integration -race ./...
   ```
5. **Cek coverage** — pipeline harus **gagal otomatis** jika coverage < 75%
6. **Kompilasi binary** — pipeline harus **gagal** jika `go build` tidak berhasil
7. Simpan laporan coverage sebagai **artifact** pipeline

Pipeline harus dapat membuktikan: jika Anda sengaja mengembalikan salah satu bug yang sudah diperbaiki di Skenario 1 lalu push, pipeline akan **merah**.

**Output yang diharapkan:** Screenshot pipeline hijau; screenshot pipeline merah saat bug dimasukkan kembali.

---

### Skenario 3 — "Binary yang Tidak Bisa Dilacak"

**Situasi:**
Tim TaskFlow sedang panik. Mereka menemukan bahwa Docker image yang berjalan di server production diberi tag `:latest` — tapi `:latest` yang mana? Apakah yang dari 3 hari lalu atau yang dari tadi malam?

*"Kalau terjadi masalah, kita tidak tahu harus rollback ke image yang mana,"* kata DevOps engineer senior.

Ia juga menunjukkan Dockerfile lama yang digunakan sebelumnya — sebuah file dengan `FROM golang:1.22` yang menghasilkan image sebesar **800 MB**. Padahal aplikasinya hanya binary Go sederhana.

**Yang harus Anda lakukan:**

Pelajari `Dockerfile` yang sudah disediakan di repository (multi-stage build dengan `scratch` base image). Tambahkan stage CD ke pipeline Anda:

1. **Build Docker image** menggunakan multi-stage `Dockerfile` yang tersedia
2. **Tag image** dengan commit SHA (bukan hanya `:latest`):
   - Format: `<registry>/<nama-image>:sha-<7-karakter-SHA>`
   - Contoh: `ghcr.io/user/taskflow-api:sha-a3f2c1d`
3. **Push image** ke container registry yang sesuai dengan tool Anda:
   - GitHub Actions → GitHub Container Registry (GHCR)
   - GitLab → GitLab Container Registry
   - Jenkins/CircleCI/Drone/Bitbucket → Docker Hub
4. CD hanya boleh berjalan **setelah CI sukses** (tidak boleh jalan paralel)
5. Dokumentasikan **perbandingan ukuran image**: multi-stage vs `FROM golang:1.22` langsung

**Output yang diharapkan:** Image ter-push ke registry dengan tag SHA; laporan perbandingan ukuran image.

---

### Skenario 4 — "Malam Sebelum Demo Investor"

**Situasi:**
Besok pagi jam 9, TaskFlow Inc. demo ke investor. CTO mengirim pesan pukul 10 malam:

*"Tim, pastikan versi yang di-staging adalah yang STABIL. Jangan ada yang deploy sembarangan malam ini. Dan kalau ada yang mau deploy, tolong pastikan server tidak mati dulu."*

Anda menyadari bahwa tidak ada cara otomatis untuk memastikan server masih hidup setelah deployment. Kalau terjadi crash malam ini, tidak ada yang akan tahu sampai investor datang besok.

**Yang harus Anda lakukan:**

Tambahkan **smoke test otomatis** yang berjalan setelah setiap deployment:

1. Setelah Docker container berhasil dijalankan, pipeline secara otomatis menjalankan:
   ```bash
   # Tunggu server siap
   sleep 5
   # Health check — gagal jika tidak merespons
   curl -f http://localhost:8080/health || exit 1
   # Pastikan endpoint utama berjalan
   curl -f http://localhost:8080/api/v1/stats || exit 1
   echo "✅ Smoke test berhasil"
   ```
2. Jika smoke test **gagal** → pipeline harus **mark as failure** (bukan sukses)
3. Tambahkan **notifikasi** (pilih salah satu): Slack webhook / Telegram bot / email
   - Notifikasi berbeda untuk pipeline **sukses** (✅) dan **gagal** (❌)
   - Sertakan: nama branch, commit SHA, waktu, link ke pipeline run

**Output yang diharapkan:** Demo live smoke test gagal lalu berhasil; screenshot notifikasi sukses dan gagal.

---

### Skenario 5 — "Bencana Deployment di Hari Jumat"

**Situasi:**
Jumat sore pukul 16.30, salah satu developer TaskFlow baru saja push dan merge sebuah fitur ke branch `main`. Pipeline CI berjalan hijau, CD pipeline pun berjalan — image baru berhasil di-push ke registry dan otomatis di-deploy ke staging.

Namun 10 menit kemudian, klien mulai mengirim laporan: *"Endpoint `/api/v1/stats` mengembalikan `completion_rate_percent: 0` padahal sudah ada 5 task yang saya selesaikan."*

Tim memeriksa kode dan menemukan: si developer secara tidak sengaja menghapus konversi `float64` saat refactoring — bug integer division kembali muncul. Kode yang salah sudah ter-deploy.

*"Kita perlu rollback sekarang! Tapi bagaimana caranya? Kita tidak tahu tag image versi sebelumnya!"* kata DevOps engineer panik. *"Dan kita tidak mau deploy ulang dari laptop — harus ada prosedur yang terdokumentasi."*

**Yang harus Anda lakukan:**

Rancang dan implementasikan **strategi rollback** dalam pipeline Anda:

1. **Pastikan setiap image punya tag yang dapat dilacak.**
   Selain tag `sha-<commit>`, tambahkan tag `stable` yang hanya diperbarui saat pipeline berjalan penuh tanpa error:

   ```
   taskflow-api:sha-a3f2c1d   ← setiap commit
   taskflow-api:stable         ← hanya saat semua test & smoke test PASS
   ```
2. **Buat Makefile target `rollback`** yang dapat dijalankan oleh siapapun di tim:

   ```makefile
   ## Rollback: jalankan image versi sebelumnya
   rollback:
   	@echo "→ Rolling back ke image: $(ROLLBACK_TAG)"
   	@test -n "$(ROLLBACK_TAG)" || (echo "ERROR: Set ROLLBACK_TAG=sha-xxxxx"; exit 1)
   	docker pull $(REGISTRY)/taskflow-api:$(ROLLBACK_TAG)
   	docker stop taskflow-api || true
   	docker run -d --rm \
   	  --name taskflow-api \
   	  -p 8080:8080 \
   	  -e DATABASE_URL=$(DATABASE_URL) \
   	  $(REGISTRY)/taskflow-api:$(ROLLBACK_TAG)
   	sleep 5
   	curl -f http://localhost:8080/health || exit 1
   	@echo "✅ Rollback berhasil ke $(ROLLBACK_TAG)"
   ```
3. **Simulasikan skenario rollback** saat presentasi:

   - Commit kode yang mengandung bug integer division
   - Push → pipeline hijau (karena bug ada di logika bisnis, bukan compile error — *inilah kenapa test penting!*)
   - Tunjukkan bahwa `/api/v1/stats` mengembalikan `completion_rate_percent: 0`
   - Jalankan rollback ke tag sebelumnya: `make rollback ROLLBACK_TAG=sha-<commit-lama>`
   - Verifikasi bahwa `/api/v1/stats` kembali benar
4. **Dokumentasikan prosedur rollback** dalam satu halaman: langkah-langkah yang harus dilakukan tim saat production bermasalah, dari deteksi hingga verifikasi selesai.

> **Catatan**: Skenario ini membuktikan dua hal sekaligus — (1) pentingnya tagging image yang konsisten agar rollback bisa dilakukan kapanpun, dan (2) pentingnya test yang mendeteksi bug logika (bukan hanya compile error).

**Output yang diharapkan:** Demo live rollback; tag `stable` di registry; dokumen prosedur rollback 1 halaman.

---

### Skenario 6 — "Audit Keamanan Pipeline"

**Situasi:**
TaskFlow Inc. sedang dalam proses sertifikasi ISO 27001 dan mendapat kunjungan dari tim auditor keamanan. Setelah memeriksa pipeline CI/CD, auditor mengeluarkan empat temuan kritis:

> **Temuan #1 — SCA (Software Composition Analysis)**
> *"Tidak ada pemeriksaan CVE pada dependency pihak ketiga. Jika library `pgx` memiliki vulnerability yang diketahui, tidak ada mekanisme yang mencegahnya masuk ke production."*
>
> **Temuan #2 — SAST (Static Application Security Testing)**
> *"Tidak ada analisis keamanan statis pada kode sumber. Pola kode berbahaya seperti SQL injection, hardcoded credential, atau penggunaan fungsi kriptografi yang lemah tidak terdeteksi secara otomatis."*
>
> **Temuan #3 — Secret Scanning**
> *"Kami menemukan bahwa tidak ada mekanisme yang mencegah developer meng-commit credential, API key, atau token ke repository. Satu kejadian seperti ini dapat membocorkan akses ke seluruh infrastruktur."*
>
> **Temuan #4 — Container Image Scanning**
> *"Docker image yang di-deploy ke production tidak pernah dipindai. Image base yang sudah lama dapat mengandung vulnerability OS-level yang serius."*

**Yang harus Anda lakukan:**

Pilih **minimal 2 dari 4 kategori** berikut dan integrasikan ke pipeline CI Anda. Setiap kategori yang diimplementasikan bernilai sama. Semua scan harus menghasilkan **laporan artifact** dan **memblokir pipeline** jika ditemukan temuan kritis.

---

**Kategori A — SCA: Dependency Vulnerability Check**

Periksa apakah library Go yang digunakan memiliki CVE yang diketahui:

```bash
# Opsi 1: govulncheck (resmi dari Google/Go team)
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck -json ./... > vuln-report.json

# Opsi 2: nancy (dari Sonatype, format OSS Index)
go list -json -deps ./... | nancy sleuth

# Opsi 3: trivy (juga bisa scan dependency Go)
trivy fs --scanners vuln --exit-code 1 --severity HIGH,CRITICAL .
```

Pipeline harus **gagal** jika ditemukan severity HIGH atau CRITICAL.

---

**Kategori B — SAST: Analisis Keamanan Kode Sumber**

Analisis pola kode berbahaya di source code Go:

```bash
# Opsi 1: gosec — SAST tool khusus Go
go install github.com/securego/gosec/v2/cmd/gosec@latest
gosec -fmt json -out gosec-report.json ./...
# Contoh rule yang dicek: G101 (hardcoded credential), G201 (SQL injection),
# G304 (file path injection), G501 (weak crypto)

# Opsi 2: staticcheck — analisis statis komprehensif
go install honnef.co/go/tools/cmd/staticcheck@latest
staticcheck ./...

# Opsi 3: semgrep (multi-bahasa, banyak rule komunitas)
semgrep scan --config "p/golang" --json > semgrep-report.json .
```

Dokumentasikan: rule mana yang relevan untuk aplikasi Go + PostgreSQL? Adakah temuan di kode TaskFlow?

---

**Kategori C — Secret Scanning: Cegah Credential Bocor**

Pastikan tidak ada secret yang ter-commit ke repository:

```bash
# Opsi 1: gitleaks — scan git history untuk secrets
docker run --rm -v $(pwd):/path zricethezav/gitleaks:latest \
  detect --source /path --report-format json --report-path /path/gitleaks-report.json

# Opsi 2: trufflehog — scan mendalam dengan entropy analysis
trufflehog git file://. --json > trufflehog-report.json

# Opsi 3: detect-secrets (dari Yelp)
detect-secrets scan > .secrets.baseline
detect-secrets audit .secrets.baseline
```

Tambahkan juga sebagai **pre-commit hook** agar scan berjalan sebelum developer bisa commit:

```bash
# .git/hooks/pre-commit
gitleaks protect --staged --redact || exit 1
```

---

**Kategori D — Container Image Scanning**

Pindai Docker image yang sudah di-build untuk menemukan vulnerability di layer OS dan library:

```bash
# Opsi 1: trivy (paling populer, gratis, dari Aqua Security)
trivy image \
  --exit-code 1 \
  --severity HIGH,CRITICAL \
  --format json \
  --output trivy-report.json \
  taskflow-api:latest

# Opsi 2: grype (dari Anchore)
grype taskflow-api:latest -o json > grype-report.json

# Opsi 3: docker scout (built-in Docker Desktop)
docker scout cves taskflow-api:latest --format json
```

> **Catatan menarik**: Image `scratch` yang digunakan TaskFlow hampir tidak punya vulnerability OS karena tidak ada OS di dalamnya! Dokumentasikan mengapa ini berbeda dengan image berbasis `ubuntu` atau `debian`.

---

**Output yang Diharapkan:**

1. Minimal 2 kategori scan berjalan di pipeline
2. Setiap scan menghasilkan laporan **artifact** (JSON/HTML)
3. Pipeline **gagal otomatis** jika ditemukan temuan dengan severity HIGH/CRITICAL
4. Laporan tertulis (1 halaman per kategori):
   - Tool yang dipilih dan alasannya
   - Temuan yang ditemukan di kode/image TaskFlow (jika ada)
   - Perbedaan false positive vs true positive
   - Rekomendasi perbaikan

---

## 5. Rubrik Penilaian

### Komponen Nilai

| #            | Komponen                        | Bobot          | Kriteria Utama                                                              |
| ------------ | ------------------------------- | -------------- | --------------------------------------------------------------------------- |
| **S1** | Temukan & perbaiki 3 bug + test | **15%**  | Semua bug benar, semua test PASS, 2 test baru, coverage ≥ 75%              |
| **S2** | CI Pipeline otomatis            | **15%**  | Trigger PR+push, vet, test-race, integration test + postgres, coverage gate |
| **S3** | CD: Docker image + registry     | **15%**  | Multi-stage build, tag SHA + stable, push ke registry, depends on CI        |
| **S4** | Smoke test + notifikasi         | **10%**  | Health check post-deploy, notifikasi sukses/gagal otomatis                  |
| **S5** | Rollback strategy               | **10%**  | Tag stable kondisional,`make rollback`, demo live, prosedur tertulis      |
| **S6** | Audit Keamanan Pipeline         | **15%**  | Min 2 kategori scan (SCA/SAST/Secret/Container), artifact, pipeline blokir  |
|              | **Laporan**               | **10%**  | Diagram pipeline, diff 3 bug, perbandingan image size, prosedur rollback    |
|              | **Presentasi & Demo**     | **10%**  | Live demo push → pipeline + rollback, tanya jawab                          |
|              | **Total**                 | **100%** |                                                                             |

---

### Detail Rubrik S1 — Bug Fix & Test (15%)

| Kriteria                    | Penuh                              | Sebagian           | Minimal                       | Tidak Ada |
| --------------------------- | ---------------------------------- | ------------------ | ----------------------------- | --------- |
| 3 bug diperbaiki            | Ketiganya benar & dapat dijelaskan | 2 dari 3           | 1 dari 3                      | 0         |
| Semua test PASS             | Ya, termasuk `-race`             | Unit test saja     | Ada yang masih gagal          | Tidak     |
| Integration test (postgres) | Berjalan dengan DB nyata & PASS    | Berjalan, ada fail | Di-skip / tidak dikonfigurasi | Tidak ada |
| 2 test case baru            | ≥ 2, relevan, PASS                | 1 test baru        | Ada tapi tidak lulus          | Tidak ada |
| Coverage ≥ 75%             | ≥ 75%                             | 65–74%            | 50–64%                       | < 50%     |

---

### Detail Rubrik S2 — CI Pipeline Otomatis (15%)

| Kriteria                            | Penuh                                               | Sebagian                   | Minimal              | Tidak Ada |
| ----------------------------------- | --------------------------------------------------- | -------------------------- | -------------------- | --------- |
| Trigger push + PR                   | Keduanya aktif                                      | Push saja                  | Ada trigger          | Tidak ada |
| `go vet` memblokir                | Pipeline merah jika `vet` error                   | Berjalan, tidak memblokir  | Ada, hasil diabaikan | Tidak ada |
| Unit test + race detector           | PASS, blokir jika ada gagal                         | PASS tanpa `-race`       | Ada, kadang gagal    | Tidak ada |
| Integration test + postgres service | Service container postgres +`DATABASE_URL` di-set | Test jalan tapi DB di-skip | Tidak dikonfigurasi  | Tidak ada |
| Coverage gate                       | Pipeline gagal otomatis jika < 75%                  | Laporan coverage saja      | Tidak ada gate       | Tidak ada |
| Coverage artifact                   | Upload ke pipeline (bisa diunduh)                   | Tersimpan lokal saja       | Tidak ada            | Tidak ada |

---

### Detail Rubrik S3 — CD: Docker Image + Registry (15%)

| Kriteria                            | Penuh                                                      | Sebagian                         | Minimal                      | Tidak Ada     |
| ----------------------------------- | ---------------------------------------------------------- | -------------------------------- | ---------------------------- | ------------- |
| Depends on CI (tidak jalan paralel) | Eksplisit, CD batal jika CI gagal                          | Implisit / manual                | Tidak ada dependency         | —            |
| Multi-stage Docker build            | Builder + scratch, image ≤ 15 MB                          | Builder + alpine                 | Single stage `FROM golang` | Gagal build   |
| Tag SHA per commit                  | Tag `sha-<7-char>` di setiap push                        | Tag SHA tanpa format konsisten   | Hanya `:latest`            | Tidak ada tag |
| Tag `stable` kondisional          | Hanya diperbarui saat semua stage + smoke test PASS        | Ada, tidak kondisional           | Tidak ada                    | —            |
| Push ke registry & dapat di-pull    | Bisa di-pull dari registry, SHA terlacak                   | Push berhasil, belum dicoba pull | Partial / tidak konsisten    | Gagal         |
| Perbandingan ukuran image           | Multi-stage vs single stage didokumentasikan dalam laporan | Disebutkan di presentasi         | Tidak ada                    | —            |

---

### Detail Rubrik S4 — Smoke Test + Notifikasi (10%)

| Kriteria                                      | Penuh                                                            | Sebagian                    | Minimal                          | Tidak Ada |
| --------------------------------------------- | ---------------------------------------------------------------- | --------------------------- | -------------------------------- | --------- |
| Smoke test `/health` otomatis post-deploy   | Berjalan di pipeline, gagal → pipeline merah                    | Berjalan, tidak blokir      | Script ada, tidak di pipeline    | Tidak ada |
| Smoke test endpoint utama (`/api/v1/stats`) | Dicek, gagal → pipeline merah                                   | Dicek, tidak blokir         | Tidak ada                        | Tidak ada |
| Notifikasi saat pipeline**sukses**      | Dikirim ke Slack/Telegram/email, isi lengkap (branch, SHA, link) | Dikirim, isi minimal        | Ada konfigurasi, tidak berfungsi | Tidak ada |
| Notifikasi saat pipeline**gagal**       | Dikirim dengan isi berbeda (tanda ❌, pesan error)               | Dikirim, sama dengan sukses | Tidak ada                        | Tidak ada |

---

### Detail Rubrik S5 — Rollback Strategy (10%)

| Kriteria                                    | Penuh                                                         | Sebagian                   | Minimal                 | Tidak Ada |
| ------------------------------------------- | ------------------------------------------------------------- | -------------------------- | ----------------------- | --------- |
| Tag `stable` hanya update saat PASS       | Kondisional: smoke test PASS → update stable                 | Ada, tidak kondisional     | Hanya tag SHA           | Tidak ada |
| `make rollback ROLLBACK_TAG=…` berfungsi | Pull image, stop lama, jalankan baru, verifikasi health       | Berjalan, tanpa verifikasi | Ada tapi tidak berjalan | Tidak ada |
| Demo rollback live saat presentasi          | Rollback berhasil,`/api/v1/stats` kembali benar             | Dicoba tapi gagal          | Tidak demo              | Tidak ada |
| Prosedur rollback terdokumentasi            | Lengkap: deteksi masalah → rollback → verifikasi → selesai | Sebagian langkah           | 1–2 baris saja         | Tidak ada |

---

### Detail Rubrik S6 — Audit Keamanan Pipeline (15%)

Mahasiswa memilih **minimal 2 dari 4 kategori** dan mengintegrasikannya ke pipeline CI.

| Kategori                                   | Tool Contoh                          | Penuh (7–8 poin)                                                                   | Sebagian (3–4 poin)              | Tidak Ada (0) |
| ------------------------------------------ | ------------------------------------ | ----------------------------------------------------------------------------------- | --------------------------------- | ------------- |
| **A — SCA** (dependency CVE)        | govulncheck, trivy fs, nancy         | Berjalan, blokir HIGH/CRITICAL, artifact JSON dihasilkan                            | Berjalan, tidak memblokir         | Tidak ada     |
| **B — SAST** (analisis kode sumber) | gosec, staticcheck, semgrep          | Berjalan, rule relevan dipilih, temuan dianalisis di laporan                        | Berjalan, temuan tidak dianalisis | Tidak ada     |
| **C — Secret Scanning**             | gitleaks, trufflehog, detect-secrets | Pipeline + pre-commit hook, artifact laporan                                        | Pipeline saja                     | Tidak ada     |
| **D — Container Image Scan**        | trivy image, grype, docker scout     | Image dipindai, severity gate, perbandingan scratch vs non-scratch didokumentasikan | Dipindai, tidak ada gate          | Tidak ada     |

> **Catatan**: 2 kategori penuh = 15 poin. 1 kategori penuh + 1 sebagian = ~11 poin. 1 kategori penuh saja = ~8 poin. Laporan wajib menyertakan analisis *false positive* vs *true positive* untuk setiap kategori yang dikerjakan.

---

### Struktur Laporan 

```
1.  Identitas kelompok & tool CI/CD yang digunakan
2.  Diagram alur pipeline lengkap (push → vet → test → build → docker → deploy)
3.  Tabel 3 bug: file, baris, kode salah, kode benar, nama test yang mendeteksi
4.  Screenshot pipeline MERAH (bug ada) dan HIJAU (bug diperbaiki)
5.  Perbandingan ukuran Docker image: multi-stage vs FROM golang:1.22 langsung
6.  Bukti image di registry: URL + contoh tag sha-xxxxx dan tag stable
7.  Screenshot smoke test berjalan + notifikasi sukses & gagal (S4)
8.  Prosedur rollback + screenshot demo rollback live (S5)
9.  [Jika S6] Laporan per kategori: tool, temuan, analisis false positive, rekomendasi
10. Refleksi: keunggulan & keterbatasan tool vs tool kelompok lain
```

---

### Presentasi & Demo Live (15 menit)

| Menit  | Aktivitas                                                                                                       |
| ------ | --------------------------------------------------------------------------------------------------------------- |
| 0–2   | Penjelasan singkat: tool yang digunakan + arsitektur pipeline                                                   |
| 2–7   | **Demo live S1 & S2**: push kode dengan bug → pipeline merah → fix → push → pipeline hijau            |
| 7–10  | **Demo live S3 & S4**: tunjukkan image di registry (tag SHA + stable), smoke test berjalan                |
| 10–13 | **Demo live S5 & S6**: commit bug baru → rollback ke versi sebelumnya → verifikasi, hasil Security Scan |
| 13–15 | Q&A dari dosen & kelompok lain                                                                                  |
