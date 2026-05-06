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
		name    string
		tasks   []model.Task
		want    float64
		isBug   bool
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

// ── [TODO] Tambahkan test berikut ─────────────────────────────────────────────

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

func TestGetAll_WithStatusFilter(t *testing.T) {
	svc := newSvc()

	// Create tasks with different statuses
	task1, _ := svc.Create(model.CreateTaskRequest{Title: "Done 1"})
	done := model.StatusDone
	svc.Update(task1.ID, model.UpdateTaskRequest{Status: &done})

	task2, _ := svc.Create(model.CreateTaskRequest{Title: "Todo 1"})
	task3, _ := svc.Create(model.CreateTaskRequest{Title: "Todo 2"})

	task4, _ := svc.Create(model.CreateTaskRequest{Title: "InProgress 1"})
	inProgress := model.StatusInProgress
	svc.Update(task4.ID, model.UpdateTaskRequest{Status: &inProgress})

	// Test filter by done
	doneTasks, err := svc.GetAll("done")
	if err != nil {
		t.Fatalf("GetAll(done) error = %v", err)
	}
	if len(doneTasks) != 1 {
		t.Errorf("GetAll(done) = %d, want 1", len(doneTasks))
	}

	// Test filter by todo
	todoTasks, err := svc.GetAll("todo")
	if err != nil {
		t.Fatalf("GetAll(todo) error = %v", err)
	}
	if len(todoTasks) != 2 {
		t.Errorf("GetAll(todo) = %d, want 2", len(todoTasks))
	}

	// Test filter by in_progress
	inProgressTasks, err := svc.GetAll("in_progress")
	if err != nil {
		t.Fatalf("GetAll(in_progress) error = %v", err)
	}
	if len(inProgressTasks) != 1 {
		t.Errorf("GetAll(in_progress) = %d, want 1", len(inProgressTasks))
	}

	// Test no filter
	allTasks, err := svc.GetAll("")
	if err != nil {
		t.Fatalf("GetAll(\"\") error = %v", err)
	}
	if len(allTasks) != 4 {
		t.Errorf("GetAll(\"\") = %d, want 4", len(allTasks))
	}

	// Verify task IDs in todo filter
	todoIDs := make(map[string]bool)
	for _, task := range todoTasks {
		todoIDs[task.ID] = true
	}
	if !todoIDs[task2.ID] || !todoIDs[task3.ID] {
		t.Error("GetAll(todo) tidak mengembalikan task yang benar")
	}
}

func TestCreate_WithUnicodeTitle(t *testing.T) {
	svc := newSvc()

	title := "Belajar DevOps \U0001f680 \u2014 \u0645\u0631\u062d\u0628\u0627 \u2014 \u4f60\u597d"
	task, err := svc.Create(model.CreateTaskRequest{Title: title})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.Title != title {
		t.Errorf("Title = %q, want %q", task.Title, title)
	}
}

func TestDelete_AndVerifyStats(t *testing.T) {
	svc := newSvc()

	// Create 3 tasks: 1 done, 2 todo
	task1, _ := svc.Create(model.CreateTaskRequest{Title: "Done 1"})
	done := model.StatusDone
	svc.Update(task1.ID, model.UpdateTaskRequest{Status: &done})

	task2, _ := svc.Create(model.CreateTaskRequest{Title: "Todo 1"})
	task3, _ := svc.Create(model.CreateTaskRequest{Title: "Todo 2"})

	// Initial stats
	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.Total != 3 {
		t.Errorf("Total awal = %d, want 3", stats.Total)
	}
	if stats.ByStatus["done"] != 1 {
		t.Errorf("ByStatus[done] awal = %d, want 1", stats.ByStatus["done"])
	}
	if stats.ByStatus["todo"] != 2 {
		t.Errorf("ByStatus[todo] awal = %d, want 2", stats.ByStatus["todo"])
	}

	// Delete one todo task
	_, err = svc.Delete(task2.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify stats after delete
	stats, err = svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats() setelah delete error = %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("Total setelah delete = %d, want 2", stats.Total)
	}
	if stats.ByStatus["todo"] != 1 {
		t.Errorf("ByStatus[todo] setelah delete = %d, want 1", stats.ByStatus["todo"])
	}
	if stats.ByStatus["done"] != 1 {
		t.Errorf("ByStatus[done] setelah delete = %d, want 1", stats.ByStatus["done"])
	}

	// Delete done task
	_, err = svc.Delete(task1.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify stats after second delete
	stats, err = svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats() setelah delete kedua error = %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("Total setelah delete kedua = %d, want 1", stats.Total)
	}
	if stats.ByStatus["todo"] != 1 {
		t.Errorf("ByStatus[todo] setelah delete kedua = %d, want 1", stats.ByStatus["todo"])
	}
	if stats.ByStatus["done"] != 0 {
		t.Errorf("ByStatus[done] setelah delete kedua = %d, want 0", stats.ByStatus["done"])
	}

	// Verify deleted task not found
	_, err = svc.GetByID(task2.ID)
	if err == nil {
		t.Error("GetByID() harus error setelah task dihapus")
	}

	// Verify remaining task still exists
	remaining, err := svc.GetByID(task3.ID)
	if err != nil {
		t.Fatalf("GetByID() task yang tersisa error = %v", err)
	}
	if remaining.ID != task3.ID {
		t.Errorf("ID task tersisa = %q, want %q", remaining.ID, task3.ID)
	}
}
