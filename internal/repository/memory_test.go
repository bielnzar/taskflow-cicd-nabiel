package repository_test

import (
	"fmt"
	"testing"

	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/repository"
)

func newRepo(t *testing.T) *repository.MemoryRepository {
	t.Helper()
	return repository.NewMemoryRepository()
}

func saveTask(t *testing.T, r *repository.MemoryRepository, id, title string, s model.Status) model.Task {
	t.Helper()
	task := model.Task{ID: id, Title: title, Status: s, Priority: model.PriorityMedium}
	if err := r.Save(task); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	return task
}

// ── [BUG] FindByStatus ───────────────────────────────────────────────────────
// BUG #2: filter menggunakan != → mengembalikan hasil TERBALIK.

func TestFindByStatus_HanyaTodo(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "1", "Todo A", model.StatusTodo)
	saveTask(t, r, "2", "Todo B", model.StatusTodo)
	saveTask(t, r, "3", "Done C", model.StatusDone)

	got, err := r.FindByStatus(model.StatusTodo)
	if err != nil {
		t.Fatalf("FindByStatus error: %v", err)
	}
	// [BUG] mengembalikan 1 (Done C), bukan 2 (Todo A & B)
	if len(got) != 2 {
		t.Errorf("BUG TERDETEKSI — FindByStatus(todo) = %d task, want 2\n"+
			"  Kondisi != mengembalikan task yang BUKAN todo\n"+
			"  Perbaiki: ubah != menjadi == di memory.go", len(got))
		return
	}
	for _, task := range got {
		if task.Status != model.StatusTodo {
			t.Errorf("FindByStatus(todo) mengembalikan status %q", task.Status)
		}
	}
}

func TestFindByStatus_HanyaDone(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "1", "A", model.StatusTodo)
	saveTask(t, r, "2", "B", model.StatusDone)
	saveTask(t, r, "3", "C", model.StatusInProgress)
	saveTask(t, r, "4", "D", model.StatusDone)

	got, err := r.FindByStatus(model.StatusDone)
	if err != nil {
		t.Fatalf("FindByStatus error: %v", err)
	}
	// [BUG] mengembalikan 2 (Todo+InProgress), bukan 2 Done
	if len(got) != 2 {
		t.Errorf("BUG — FindByStatus(done) = %d, want 2", len(got))
	}
}

func TestFindByStatus_KosongJikaStatusTidakAda(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "1", "A", model.StatusTodo)

	got, err := r.FindByStatus(model.StatusDone)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// [BUG] mengembalikan 1 (Todo), bukan 0
	if len(got) != 0 {
		t.Errorf("BUG — FindByStatus(done) saat hanya ada todo = %d, want 0", len(got))
	}
}

// ── FindAll ───────────────────────────────────────────────────────────────────

func TestFindAll(t *testing.T) {
	r := newRepo(t)
	if tasks, _ := r.FindAll(); len(tasks) != 0 {
		t.Errorf("repo baru harus kosong, got %d", len(tasks))
	}
	saveTask(t, r, "1", "A", model.StatusTodo)
	saveTask(t, r, "2", "B", model.StatusDone)
	if tasks, _ := r.FindAll(); len(tasks) != 2 {
		t.Errorf("FindAll() = %d, want 2", len(tasks))
	}
}

// ── FindByID ──────────────────────────────────────────────────────────────────

func TestFindByID(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "x-1", "Cari", model.StatusTodo)

	got, ok, err := r.FindByID("x-1")
	if err != nil || !ok {
		t.Fatalf("FindByID: ok=%v err=%v", ok, err)
	}
	if got.Title != "Cari" {
		t.Errorf("Title = %q, want Cari", got.Title)
	}

	_, ok, _ = r.FindByID("tidak-ada")
	if ok {
		t.Error("FindByID ID tidak ada harus false")
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDelete(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "d-1", "Hapus", model.StatusTodo)

	ok, err := r.Delete("d-1")
	if !ok || err != nil {
		t.Fatalf("Delete gagal: ok=%v err=%v", ok, err)
	}
	if _, found, _ := r.FindByID("d-1"); found {
		t.Error("task masih ada setelah dihapus")
	}
	if ok2, _ := r.Delete("d-1"); ok2 {
		t.Error("Delete yang sudah dihapus harus false")
	}
}

// ── [CICD] Concurrency — pipeline wajib: go test -race ./... ──────────────────

func TestConcurrentSave(t *testing.T) {
	r := newRepo(t)
	done := make(chan struct{}, 100)
	for i := 0; i < 100; i++ {
		go func(n int) {
			_ = r.Save(model.Task{
				ID:     fmt.Sprintf("c-%d", n),
				Title:  "Concurrent",
				Status: model.StatusTodo,
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	count, _ := r.Count()
	if count != 100 {
		t.Errorf("concurrent save: Count = %d, want 100", count)
	}
}

func TestCount_AfterDelete(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "c1", "Task 1", model.StatusTodo)
	saveTask(t, r, "c2", "Task 2", model.StatusDone)
	saveTask(t, r, "c3", "Task 3", model.StatusInProgress)

	count, _ := r.Count()
	if count != 3 {
		t.Errorf("Count = %d, want 3", count)
	}

	// Delete one
	r.Delete("c2")
	count, _ = r.Count()
	if count != 2 {
		t.Errorf("Count after delete = %d, want 2", count)
	}

	// Delete non-existent
	r.Delete("non-existent")
	count, _ = r.Count()
	if count != 2 {
		t.Errorf("Count after deleting non-existent = %d, want 2", count)
	}
}

func TestMemoryRepository_ClearCloseAndString(t *testing.T) {
	r := newRepo(t)
	saveTask(t, r, "s1", "Task 1", model.StatusTodo)

	if got := r.String(); got != "MemoryRepository{count: 1}" {
		t.Errorf("String() = %q, want count 1", got)
	}

	r.Clear()
	count, err := r.Count()
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() after Clear() = %d, want 0", count)
	}

	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}
