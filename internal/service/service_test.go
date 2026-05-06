package service_test

import (
	"testing"

	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/repository"
	"github.com/taskflow/api/internal/service"
)

func newSvc() *service.TaskService {
	return service.NewTaskService(repository.NewMemoryRepository())
}

// ── [BUG] CalculateCompletionRate ────────────────────────────────────────────
// BUG #1: Integer division — hasil selalu 0 (kecuali semua task selesai).

func TestCalculateCompletionRate(t *testing.T) {
	tests := []struct {
		name  string
		tasks []model.Task
		want  float64
		isBug bool
	}{
		{
			name:  "tidak ada task",
			tasks: []model.Task{},
			want:  0,
		},
		{
			name:  "semua done → 100%",
			tasks: []model.Task{{Status: model.StatusDone}, {Status: model.StatusDone}},
			want:  100.0,
		},
		{
			// [BUG] 1/3 dengan integer division = 0, bukan 33.33
			name: "[BUG] sepertiga selesai → 33.33%",
			tasks: []model.Task{
				{Status: model.StatusDone},
				{Status: model.StatusTodo},
				{Status: model.StatusTodo},
			},
			want:  33.33,
			isBug: true,
		},
		{
			// [BUG] 1/2 dengan integer division = 0, bukan 50.0
			name:  "[BUG] setengah selesai → 50%",
			tasks: []model.Task{{Status: model.StatusDone}, {Status: model.StatusTodo}},
			want:  50.0,
			isBug: true,
		},
		{
			name: "tidak ada yang selesai → 0%",
			tasks: []model.Task{
				{Status: model.StatusTodo},
				{Status: model.StatusInProgress},
			},
			want: 0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.CalculateCompletionRate(tc.tasks)
			// Toleransi 0.01 untuk floating point
			diff := got - tc.want
			if diff < 0 {
				diff = -diff
			}
			if diff > 0.01 {
				if tc.isBug {
					t.Errorf("BUG TERDETEKSI — CalculateCompletionRate() = %.2f, want %.2f\n"+
						"  → Integer division: %d/%d = 0 (bukan %.2f)\n"+
						"  → Perbaiki: gunakan float64(completed)/float64(len(tasks))*100",
						got, tc.want, len(tc.tasks)/2, len(tc.tasks), tc.want)
				} else {
					t.Errorf("CalculateCompletionRate() = %.2f, want %.2f", got, tc.want)
				}
			}
		})
	}
}

// ── Create ───────────────────────────────────────────────────────────────────

func TestCreate(t *testing.T) {
	svc := newSvc()

	t.Run("sukses dengan default priority", func(t *testing.T) {
		task, err := svc.Create(model.CreateTaskRequest{Title: "Belajar Go"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if task.Title != "Belajar Go" {
			t.Errorf("Title = %q, want %q", task.Title, "Belajar Go")
		}
		if task.Status != model.StatusTodo {
			t.Errorf("Status = %q, want todo", task.Status)
		}
		if task.Priority != model.PriorityMedium {
			t.Errorf("Priority = %q, want medium (default)", task.Priority)
		}
		if task.ID == "" {
			t.Error("ID tidak boleh kosong")
		}
	})

	t.Run("title kosong ditolak", func(t *testing.T) {
		_, err := svc.Create(model.CreateTaskRequest{Title: ""})
		if err == nil {
			t.Error("Create() harus error jika title kosong")
		}
	})

	t.Run("title spasi saja ditolak", func(t *testing.T) {
		_, err := svc.Create(model.CreateTaskRequest{Title: "   "})
		if err == nil {
			t.Error("Create() harus error jika title hanya spasi")
		}
	})

	t.Run("priority invalid ditolak", func(t *testing.T) {
		_, err := svc.Create(model.CreateTaskRequest{Title: "T", Priority: "extreme"})
		if err == nil {
			t.Error("Create() harus error untuk priority tidak valid")
		}
	})

	t.Run("priority high sukses", func(t *testing.T) {
		task, err := svc.Create(model.CreateTaskRequest{Title: "Urgent", Priority: model.PriorityHigh})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if task.Priority != model.PriorityHigh {
			t.Errorf("Priority = %q, want high", task.Priority)
		}
	})

	t.Run("setiap task ID unik", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 50; i++ {
			task, _ := svc.Create(model.CreateTaskRequest{Title: "Task"})
			if ids[task.ID] {
				t.Errorf("ID duplikat ditemukan: %s", task.ID)
			}
			ids[task.ID] = true
		}
	})
}

// ── Update ───────────────────────────────────────────────────────────────────

func TestUpdate(t *testing.T) {
	svc := newSvc()

	t.Run("update status ke done mengisi completed_at", func(t *testing.T) {
		task, _ := svc.Create(model.CreateTaskRequest{Title: "Selesaikan"})
		statusDone := model.StatusDone
		updated, err := svc.Update(task.ID, model.UpdateTaskRequest{Status: &statusDone})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.CompletedAt == nil {
			t.Error("CompletedAt harus terisi setelah status = done")
		}
	})

	t.Run("update task tidak ada → error", func(t *testing.T) {
		statusDone := model.StatusDone
		_, err := svc.Update("id-tidak-ada", model.UpdateTaskRequest{Status: &statusDone})
		if err == nil {
			t.Error("Update() harus error untuk ID tidak ada")
		}
	})

	t.Run("update status invalid → error", func(t *testing.T) {
		task, _ := svc.Create(model.CreateTaskRequest{Title: "T"})
		s := model.Status("invalid")
		_, err := svc.Update(task.ID, model.UpdateTaskRequest{Status: &s})
		if err == nil {
			t.Error("Update() harus error untuk status tidak valid")
		}
	})
}

// ── [CICD] Full Task Lifecycle ────────────────────────────────────────────────
// [CICD] Simulasi integration test: create → get → update → delete.
// Jenis test ini dijalankan otomatis setelah deploy ke staging.

func TestTaskFullLifecycle(t *testing.T) {
	svc := newSvc()

	// 1. Create
	task, err := svc.Create(model.CreateTaskRequest{
		Title:    "Pipeline Lifecycle Test",
		Priority: model.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("Create() gagal: %v", err)
	}

	// 2. Get
	got, err := svc.GetByID(task.ID)
	if err != nil || got.ID != task.ID {
		t.Fatalf("GetByID() gagal setelah create")
	}

	// 3. Update ke in_progress
	s := model.StatusInProgress
	got, err = svc.Update(task.ID, model.UpdateTaskRequest{Status: &s})
	if err != nil || got.Status != model.StatusInProgress {
		t.Fatalf("Update() ke in_progress gagal")
	}

	// 4. Update ke done
	done := model.StatusDone
	got, err = svc.Update(task.ID, model.UpdateTaskRequest{Status: &done})
	if err != nil || got.CompletedAt == nil {
		t.Fatalf("Update() ke done gagal atau CompletedAt nil")
	}

	// 5. Stats harus menunjukkan 1 done
	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats() gagal: %v", err)
	}
	if stats.ByStatus["done"] != 1 {
		t.Errorf("Stats.ByStatus[done] = %d, want 1", stats.ByStatus["done"])
	}

	// 6. Delete
	_, err = svc.Delete(task.ID)
	if err != nil {
		t.Fatalf("Delete() gagal: %v", err)
	}

	// 7. Pastikan sudah terhapus
	if _, err = svc.GetByID(task.ID); err == nil {
		t.Error("GetByID() harus error setelah task dihapus")
	}
}

// ── [CICD] Rollback Simulation ───────────────────────────────────────────────

func TestRollbackStatusSimulation(t *testing.T) {
	svc := newSvc()
	task, _ := svc.Create(model.CreateTaskRequest{Title: "Rollback Test"})

	// Simulasi: deploy berhasil → update ke in_progress
	s := model.StatusInProgress
	svc.Update(task.ID, model.UpdateTaskRequest{Status: &s}) //nolint

	// Deployment bermasalah → rollback ke todo
	todo := model.StatusTodo
	rolled, err := svc.Update(task.ID, model.UpdateTaskRequest{Status: &todo})
	if err != nil {
		t.Fatalf("Rollback gagal: %v", err)
	}
	if rolled.Status != model.StatusTodo {
		t.Errorf("Setelah rollback, status = %q, want todo", rolled.Status)
	}
}

func TestConcurrentCreate_UniqueIDs(t *testing.T) {
	svc := newSvc()
	ids := make(chan string, 100)
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		go func() {
			task, err := svc.Create(model.CreateTaskRequest{Title: "Concurrent Task"})
			if err != nil {
				errs <- err
				return
			}
			ids <- task.ID
			errs <- nil
		}()
	}

	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		id := <-ids
		if id == "" {
			t.Fatal("Create() returned empty ID")
		}
		if seen[id] {
			t.Fatalf("duplicate ID from concurrent create: %s", id)
		}
		seen[id] = true
	}
}

func TestGetStats_CompletionRate(t *testing.T) {
	svc := newSvc()

	// Create 4 tasks: 1 done, 2 todo, 1 in_progress
	task1, _ := svc.Create(model.CreateTaskRequest{Title: "Done 1"})
	done := model.StatusDone
	svc.Update(task1.ID, model.UpdateTaskRequest{Status: &done})
	svc.Create(model.CreateTaskRequest{Title: "Todo 1"})
	svc.Create(model.CreateTaskRequest{Title: "Todo 2"})
	task4, _ := svc.Create(model.CreateTaskRequest{Title: "InProgress 1"})
	inProgress := model.StatusInProgress
	svc.Update(task4.ID, model.UpdateTaskRequest{Status: &inProgress})

	// Update first task to done
	tasks, _ := svc.GetAll("")
	for _, task := range tasks {
		if task.Title == "Done 1" {
			done := model.StatusDone
			svc.Update(task.ID, model.UpdateTaskRequest{Status: &done})
		}
	}

	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.Total != 4 {
		t.Errorf("Total = %d, want 4", stats.Total)
	}
	// 1 done out of 4 = 25%
	if stats.CompletionRate != 25.0 {
		t.Errorf("CompletionRate = %.2f, want 25.0", stats.CompletionRate)
	}
}
