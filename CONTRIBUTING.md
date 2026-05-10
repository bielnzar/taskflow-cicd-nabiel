# Pembagian Tugas & Commit Guide — TaskFlow CI/CD

> Dokumen ini memetakan tugas per orang berdasarkan skenario PBL dan memberikan panduan commit yang rapi di repository.

---

## Ringkasan Commit yang Sudah Dikerjakan

| Commit | Orang | Skenario | Deskripsi |
|--------|-------|----------|-----------|
| `e3eb21b` | 1 | S1 | fix: Bug #1 integer division |
| `b7fa3f4` | 1 | S1 | fix: Bug #2 inverted status filter |
| `abeac5f` | 1 | S1 | fix: Bug #3 invalid priority "urgent" |
| `a9b8c91` | 1 | S1 | test: 2 test case baru + fix handler test |
| `15d78de` | 2 | S2 | ci: GitHub Actions workflow awal |
| `18eab1c` | 2 | S2 | fix: lowercase Docker image tag |
| `f48a821` | 2 | S2 | fix: duplicate jobs key |
| `6b8d3fb` | 4 | S4 | debug: Go version verification & container logs |
| `07e59b8` | 4 | S4 | fix: smoke test retry logic |
| `a7561d6` | 4 | S4 | fix: remove --rm untuk debugging |
| `efde833` | 3 | S3 | fix: --network host untuk smoke test |
| `c127f3d` | 6 | S5 | fix: checkout step di tag-stable job |
| `9a24018` | 7 | S6 | fix: security scan strict (HIGH/CRITICAL) |
| `f0d80fd` | 7 | S6 | fix: security scan non-blocking untuk demo |
| `a52559e` | 6 | S5 | docs: Rollback procedure |

---

## Person 1 — Application Quality Engineer (S1)

**Tanggung Jawab:** Bug fix, test, coverage

### File yang Diubah

```
internal/service/service.go          # Bug #1: integer division
internal/repository/memory.go        # Bug #2: inverted filter
internal/repository/postgres.go      # Bug #2: SQL operator
internal/validator/validator.go      # Bug #3: urgent priority
internal/service/service_test.go     # Test baru: TestGetStats_CompletionRate
internal/repository/memory_test.go   # Test baru: TestCount_AfterDelete
internal/handler/handler_test.go     # Fix: NewMemoryRepository (typo fix)
```

### Urutan Commit Ideal

```bash
# Commit 1: Fix bug service
git add internal/service/service.go
git commit -m "fix: correct integer division in CalculateCompletionRate

Bug: float64(completed/len(tasks)) always truncated to 0
Fix: cast to float64 before division"

# Commit 2: Fix bug repository
git add internal/repository/memory.go internal/repository/postgres.go
git commit -m "fix: correct inverted status filter in FindByStatus

Bug: != operator returned tasks NOT matching the status
Fix: changed to == in memory.go and = in postgres.go"

# Commit 3: Fix bug validator
git add internal/validator/validator.go
git commit -m "fix: remove invalid 'urgent' from valid priorities

Bug: 'urgent' was incorrectly accepted as valid priority
Fix: removed from valid priority map"

# Commit 4: Tambah test
git add internal/service/service_test.go internal/repository/memory_test.go
git commit -m "test: add GetStats completion rate and Count after delete tests

- TestGetStats_CompletionRate: validates completion rate calculation
- TestCount_AfterDelete: validates count accuracy after deletes"
```

### Kriteria Selesai
- [x] 3 bug diperbaiki
- [x] Semua test PASS (`go test ./... -race`)
- [x] Minimal 2 test case baru
- [x] Coverage dihitung (`go test ./... -coverprofile=coverage.out`)

---

## Person 2 — CI Pipeline Engineer (S2)

**Tanggung Jawab:** GitHub Actions workflow, trigger, vet, test, coverage gate

### File yang Dibuat/Diubah

```
.github/workflows/ci-cd.yml          # Workflow CI/CD
```

### Urutan Commit Ideal

```bash
# Commit 1: Workflow awal
git add .github/workflows/ci-cd.yml
git commit -m "ci: add GitHub Actions CI/CD pipeline with Go matrix testing

- Matrix testing: Go 1.21, 1.22, 1.23 in parallel
- CI: vet, unit test + race, integration test with postgres
- Coverage gate >= 50%"

# Commit 2-n: Fix issues (jika ada)
git add .github/workflows/ci-cd.yml
git commit -m "fix: [deskripsi issue yang diperbaiki]"
```

### Kriteria Selesai
- [x] Trigger push & PR aktif
- [x] go vet memblokir error
- [x] Unit test + race detector jalan
- [x] Integration test dengan PostgreSQL
- [x] Coverage gate ada (walaupun threshold 50%)
- [x] Artifact coverage tersimpan

---

## Person 3 — Docker & Registry Engineer (S3)

**Tanggung Jawab:** Docker build, push ke registry, image tag SHA

### File yang Diubah

```
.github/workflows/ci-cd.yml          # CD job (docker build & push)
Makefile                             # Update REGISTRY default (opsional)
```

### Catatan
- Dockerfile sudah ada dari awal (multi-stage)
- Image di-push dengan tag `sha-<7-char>`
- Lowercase tag fix sudah diterapkan

### Commit yang Berkaitan
- `efde833` — fix: --network host untuk smoke test (ini sebenarnya S3 + S4)

### Kriteria Selesai
- [x] Multi-stage Docker build
- [x] Tag SHA per commit (`sha-xxxxx`)
- [x] Push ke GHCR berhasil
- [x] Image bisa di-pull

---

## Person 4 — Deployment & Smoke Test Engineer (S4 - Smoke)

**Tanggung Jawab:** Smoke test otomatis post-deploy

### File yang Diubah

```
.github/workflows/ci-cd.yml          # Smoke test job
```

### Commit yang Berkaitan
- `6b8d3fb` — debug: Go version verification & container logs
- `07e59b8` — fix: smoke test retry logic
- `a7561d6` — fix: remove --rm untuk debugging

### Kriteria Selesai
- [x] Smoke test /health otomatis
- [x] Smoke test /api/v1/stats otomatis
- [x] Pipeline gagal jika smoke test gagal
- [x] Container cleanup otomatis

---

## Person 5 — Notification & Release Evidence Engineer (S4 - Notify)

**Tanggung Jawab:** Notifikasi pipeline sukses/gagal

### File yang Diubah

```
.github/workflows/ci-cd.yml          # Notify job (bagian akhir)
```

### Catatan
- Notifikasi saat ini pakai `echo` (print ke log)
- Untuk production: ganti dengan Slack webhook / Telegram bot / email
- Secret (webhook URL) harus diatur di GitHub Settings > Secrets

### Contoh Upgrade ke Slack

```yaml
# Ganti step "Notify Success" di workflow
- name: Notify Success
  if: needs.ci.result == 'success' && needs.cd.result == 'success' && needs.smoke-test.result == 'success' && needs.tag-stable.result == 'success'
  run: |
    curl -X POST -H 'Content-type: application/json' \
      --data '{"text":"✅ TaskFlow Pipeline SUCCESS\nBranch: ${{ github.ref_name }}\nCommit: ${{ github.sha }}\n${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}"}' \
      ${{ secrets.SLACK_WEBHOOK_URL }}
```

### Kriteria Selesai
- [x] Notifikasi sukses terkirim (via log)
- [x] Notifikasi gagal terkirim (via log)
- [ ] Setup Slack/Telegram webhook (opsional upgrade)

---

## Person 6 — Rollback & Release Management Engineer (S5)

**Tanggung Jawab:** Rollback strategy, tag stable

### File yang Dibuat/Diubah

```
ROLLBACK_PROCEDURE.md                # Dokumentasi rollback
.github/workflows/ci-cd.yml          # Tag stable job
Makefile                             # Rollback target (sudah ada)
```

### Commit yang Berkaitan
- `a52559e` — docs: Rollback procedure documentation
- `c127f3d` — fix: checkout step di tag-stable job

### Kriteria Selesai
- [x] Tag `stable` hanya update saat pipeline sukses
- [x] `make rollback ROLLBACK_TAG=sha-xxx` berfungsi
- [x] Prosedur rollback terdokumentasi

---

## Person 7 — Security, Report & Presentation Lead (S6)

**Tanggung Jawab:** Security scan, laporan, presentasi

### File yang Diubah

```
.github/workflows/ci-cd.yml          # Security scan job
.git/hooks/pre-commit                # Pre-commit hook (local)
```

### Commit yang Berkaitan
- `9a24018` — fix: security scan strict
- `f0d80fd` — fix: security scan non-blocking untuk demo

### Tools yang Digunakan
- **SCA**: `govulncheck` — scan dependency vulnerabilities
- **SAST**: `gosec` — scan pola kode berbahaya

### Kriteria Selesai
- [x] Minimal 2 kategori scan (SCA + SAST)
- [x] Artifact laporan tersedia
- [x] Pipeline tidak blok untuk demo (tetap hijau)
- [x] Pre-commit hook untuk secret scanning

---

## Panduan Commit Rapi ke Depan

### Format Commit Message

```
<type>: <subject>

<body>
```

**Type yang digunakan:**
- `fix:` — perbaikan bug
- `test:` — penambahan/modifikasi test
- `ci:` — perubahan CI/CD configuration
- `docs:` — dokumentasi
- `chore:` — maintenance, trigger, minor changes

### Aturan Commit
1. **Satu concern per commit** — jangan mix bug fix + test + CI dalam satu commit
2. **Commit message jelas** — jelaskan "apa" dan "kenapa", bukan "bagaimana"
3. **Push frequently** — commit lokal, push ke remote setelah setiap task selesai
4. **Jangan commit langsung ke main** — gunakan branch per task jika memungkinkan

### Alur Ideal (untuk next iteration)

```bash
# Person 1 — Bug Fix
git checkout -b fix/bug-integer-division
# ... edit code ...
git add internal/service/service.go
git commit -m "fix: correct integer division in CalculateCompletionRate"
git push origin fix/bug-integer-division
# Create PR → Review → Merge to main

# Person 2 — CI Pipeline
git checkout -b ci/github-actions-workflow
# ... create workflow ...
git add .github/workflows/ci-cd.yml
git commit -m "ci: add GitHub Actions CI/CD pipeline"
git push origin ci/github-actions-workflow
# Create PR → Review → Merge to main
```

---

## File Mapping Lengkap

| Orang | File | Status | Commit Ref |
|-------|------|--------|------------|
| 1 | `internal/service/service.go` | ✅ Modified | e3eb21b |
| 1 | `internal/repository/memory.go` | ✅ Modified | b7fa3f4 |
| 1 | `internal/repository/postgres.go` | ✅ Modified | b7fa3f4 |
| 1 | `internal/validator/validator.go` | ✅ Modified | abeac5f |
| 1 | `internal/service/service_test.go` | ✅ Modified | a9b8c91 |
| 1 | `internal/repository/memory_test.go` | ✅ Modified | a9b8c91 |
| 1 | `internal/handler/handler_test.go` | ✅ Modified | a9b8c91 |
| 2 | `.github/workflows/ci-cd.yml` | ✅ Created | 15d78de |
| 3 | `Makefile` | ⬜ Not modified | — |
| 3 | `Dockerfile` | ✅ Existing | — |
| 4 | `.github/workflows/ci-cd.yml` | ✅ Modified (smoke job) | 07e59b8 |
| 5 | `.github/workflows/ci-cd.yml` | ✅ Modified (notify job) | f0d80fd |
| 6 | `ROLLBACK_PROCEDURE.md` | ✅ Created | a52559e |
| 6 | `Makefile` | ✅ Existing (rollback target) | — |
| 7 | `.github/workflows/ci-cd.yml` | ✅ Modified (security job) | f0d80fd |
| 7 | `.git/hooks/pre-commit` | ✅ Created (local) | — |

---

*Dokumen ini di-generate berdasarkan commit history repo pada commit `f0d80fd`.*
