// Package service — business logic task dengan repository interface.
package service

import (
	"fmt"
	"time"

	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/repository"
	"github.com/taskflow/api/internal/validator"
)

// TaskService mengelola business logic task.
type TaskService struct {
	repo repository.TaskRepository
}

// NewTaskService membuat instance baru TaskService.
func NewTaskService(repo repository.TaskRepository) *TaskService {
	return &TaskService{repo: repo}
}

// Create membuat task baru setelah memvalidasi input.
func (s *TaskService) Create(req model.CreateTaskRequest) (model.Task, error) {
	if !validator.IsNotEmpty(req.Title) {
		return model.Task{}, fmt.Errorf("title tidak boleh kosong")
	}
	if !validator.MaxLength(req.Title, 200) {
		return model.Task{}, fmt.Errorf("title maksimal 200 karakter")
	}
	priority := req.Priority
	if priority == "" {
		priority = model.PriorityMedium
	}
	if !validator.IsValidPriority(string(priority)) {
		return model.Task{}, fmt.Errorf("priority tidak valid: %s (pilihan: low, medium, high)", priority)
	}

	now := time.Now().UTC()
	task := model.Task{
		ID:          generateID(),
		Title:       req.Title,
		Description: req.Description,
		Priority:    priority,
		Status:      model.StatusTodo,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Save(task); err != nil {
		return model.Task{}, fmt.Errorf("gagal menyimpan task: %w", err)
	}
	return task, nil
}

// GetByID mengambil task berdasarkan ID.
func (s *TaskService) GetByID(id string) (model.Task, error) {
	task, ok, err := s.repo.FindByID(id)
	if err != nil {
		return model.Task{}, fmt.Errorf("error database: %w", err)
	}
	if !ok {
		return model.Task{}, fmt.Errorf("task dengan ID '%s' tidak ditemukan", id)
	}
	return task, nil
}

// GetAll mengambil semua task, opsional difilter berdasarkan status.
func (s *TaskService) GetAll(statusFilter string) ([]model.Task, error) {
	if statusFilter != "" {
		if !validator.IsValidStatus(statusFilter) {
			return nil, fmt.Errorf("status filter tidak valid: %s", statusFilter)
		}
		return s.repo.FindByStatus(model.Status(statusFilter))
	}
	return s.repo.FindAll()
}

// Update memperbarui field task.
func (s *TaskService) Update(id string, req model.UpdateTaskRequest) (model.Task, error) {
	task, ok, err := s.repo.FindByID(id)
	if err != nil {
		return model.Task{}, fmt.Errorf("error database: %w", err)
	}
	if !ok {
		return model.Task{}, fmt.Errorf("task dengan ID '%s' tidak ditemukan", id)
	}

	if req.Title != nil {
		if !validator.IsNotEmpty(*req.Title) {
			return model.Task{}, fmt.Errorf("title tidak boleh kosong")
		}
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		if !validator.IsValidStatus(string(*req.Status)) {
			return model.Task{}, fmt.Errorf("status tidak valid: %s", *req.Status)
		}
		task.Status = *req.Status
		if task.Status == model.StatusDone && task.CompletedAt == nil {
			now := time.Now().UTC()
			task.CompletedAt = &now
		}
	}
	task.UpdatedAt = time.Now().UTC()
	if err := s.repo.Save(task); err != nil {
		return model.Task{}, fmt.Errorf("gagal update task: %w", err)
	}
	return task, nil
}

// Delete menghapus task berdasarkan ID.
func (s *TaskService) Delete(id string) (model.Task, error) {
	task, ok, err := s.repo.FindByID(id)
	if err != nil {
		return model.Task{}, fmt.Errorf("error database: %w", err)
	}
	if !ok {
		return model.Task{}, fmt.Errorf("task dengan ID '%s' tidak ditemukan", id)
	}
	if _, err := s.repo.Delete(id); err != nil {
		return model.Task{}, fmt.Errorf("gagal hapus task: %w", err)
	}
	return task, nil
}

// GetStats mengembalikan statistik semua task.
func (s *TaskService) GetStats() (model.StatsResponse, error) {
	tasks, err := s.repo.FindAll()
	if err != nil {
		return model.StatsResponse{}, fmt.Errorf("error database: %w", err)
	}
	stats := model.StatsResponse{
		Total: len(tasks),
		ByStatus: map[string]int{
			"todo":        0,
			"in_progress": 0,
			"done":        0,
		},
		ByPriority: map[string]int{
			"low":    0,
			"medium": 0,
			"high":   0,
		},
	}
	for _, t := range tasks {
		stats.ByStatus[string(t.Status)]++
		stats.ByPriority[string(t.Priority)]++
	}
	stats.CompletionRate = CalculateCompletionRate(tasks)
	return stats, nil
}

// CalculateCompletionRate menghitung persentase task berstatus "done".
//
// BUG #1: integer division — hasil selalu 0 kecuali semua task selesai.
// Contoh: 1 dari 3 task selesai → 1/3 = 0 (integer), bukan 33.33.
// Perbaiki: float64(completed) / float64(len(tasks)) * 100
func CalculateCompletionRate(tasks []model.Task) float64 {
	if len(tasks) == 0 {
		return 0
	}
	completed := 0
	for _, t := range tasks {
		if t.Status == model.StatusDone {
			completed++
		}
	}
	// BUG: integer division
	return float64(completed/len(tasks)) * 100
}

// generateID membuat ID unik berbasis timestamp + counter.
var counter int64

func generateID() string {
	counter++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), counter)
}
