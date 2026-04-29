//go:build integration

// Integration test untuk PostgresRepository.
// Dijalankan HANYA dengan: go test -tags=integration ./...
// Membutuhkan DATABASE_URL environment variable.
//
// Di pipeline CI, set postgres sebagai service container dan tambahkan:
//   DATABASE_URL=postgres://taskflow:secret@localhost:5432/taskflow?sslmode=disable
//
// Contoh GitHub Actions:
//   - run: go test -tags=integration -race ./...
//     env:
//       DATABASE_URL: ${{ env.DATABASE_URL }}
package repository_test

import (
	"os"
	"testing"

	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/repository"
)

func newPgRepo(t *testing.T) *repository.PostgresRepository {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL tidak di-set, skip integration test.\n" +
			"Set DATABASE_URL=postgres://... untuk menjalankan test ini.")
	}
	r, err := repository.NewPostgresRepository(dbURL)
	if err != nil {
		t.Fatalf("gagal konek ke postgres: %v", err)
	}
	if err := r.Migrate(); err != nil {
		t.Fatalf("migrate gagal: %v", err)
	}
	// Bersihkan tabel sebelum test
	t.Cleanup(func() { r.TruncateForTest(t) })
	r.TruncateForTest(t)
	return r
}

// ── [BUG] FindByStatus (Postgres) ────────────────────────────────────────────
// BUG #2 juga ada di postgres.go → SQL WHERE status != $1

func TestPostgres_FindByStatus_HanyaTodo(t *testing.T) {
	r := newPgRepo(t)

	tasks := []model.Task{
		{ID: "p1", Title: "Todo A", Status: model.StatusTodo, Priority: model.PriorityMedium},
		{ID: "p2", Title: "Todo B", Status: model.StatusTodo, Priority: model.PriorityMedium},
		{ID: "p3", Title: "Done C", Status: model.StatusDone, Priority: model.PriorityMedium},
	}
	for _, task := range tasks {
		if err := r.Save(task); err != nil {
			t.Fatalf("Save error: %v", err)
		}
	}

	got, err := r.FindByStatus(model.StatusTodo)
	if err != nil {
		t.Fatalf("FindByStatus error: %v", err)
	}
	// [BUG] SQL: WHERE status != 'todo' → mengembalikan Done C, bukan Todo A & B
	if len(got) != 2 {
		t.Errorf("BUG POSTGRES — FindByStatus(todo) = %d task, want 2\n"+
			"  SQL WHERE status != $1 mengembalikan yang BUKAN todo\n"+
			"  Perbaiki: ubah != menjadi = di postgres.go", len(got))
	}
}

func TestPostgres_FindByStatus_Kosong(t *testing.T) {
	r := newPgRepo(t)
	r.Save(model.Task{ID: "p1", Title: "A", Status: model.StatusTodo, Priority: model.PriorityMedium}) //nolint

	got, err := r.FindByStatus(model.StatusDone)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// [BUG] mengembalikan 1 (todo task), bukan 0
	if len(got) != 0 {
		t.Errorf("BUG POSTGRES — FindByStatus(done) = %d, want 0", len(got))
	}
}

// ── Integration: Full Lifecycle di PostgreSQL ─────────────────────────────────

func TestPostgres_FullLifecycle(t *testing.T) {
	r := newPgRepo(t)

	// Create
	task := model.Task{
		ID: "lifecycle-1", Title: "Integration Test",
		Status: model.StatusTodo, Priority: model.PriorityHigh,
	}
	if err := r.Save(task); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	// Read
	got, ok, err := r.FindByID("lifecycle-1")
	if err != nil || !ok {
		t.Fatalf("FindByID gagal: ok=%v err=%v", ok, err)
	}
	if got.Title != "Integration Test" {
		t.Errorf("Title = %q, want Integration Test", got.Title)
	}

	// Update status
	got.Status = model.StatusDone
	if err := r.Save(got); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	// Verify update
	updated, _, _ := r.FindByID("lifecycle-1")
	if updated.Status != model.StatusDone {
		t.Errorf("Status setelah update = %q, want done", updated.Status)
	}

	// Delete
	deleted, err := r.Delete("lifecycle-1")
	if err != nil || !deleted {
		t.Fatalf("Delete gagal: deleted=%v err=%v", deleted, err)
	}

	// Verify deleted
	if _, found, _ := r.FindByID("lifecycle-1"); found {
		t.Error("task masih ada setelah dihapus")
	}
}
