// Package repository — implementasi in-memory (digunakan untuk unit test).
package repository

import (
	"fmt"
	"sync"

	"github.com/taskflow/api/internal/model"
)

// Pastikan MemoryRepository mengimplementasikan interface TaskRepository.
var _ TaskRepository = (*MemoryRepository)(nil)

// MemoryRepository menyimpan task di memori (thread-safe via RWMutex).
type MemoryRepository struct {
	mu    sync.RWMutex
	tasks map[string]model.Task
}

// NewMemoryRepository membuat instance baru MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{tasks: make(map[string]model.Task)}
}

func (r *MemoryRepository) Save(task model.Task) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[task.ID] = task
	return nil
}

func (r *MemoryRepository) FindByID(id string) (model.Task, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	task, ok := r.tasks[id]
	return task, ok, nil
}

func (r *MemoryRepository) FindAll() ([]model.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]model.Task, 0, len(r.tasks))
	for _, t := range r.tasks {
		result = append(result, t)
	}
	return result, nil
}

// FindByStatus mengembalikan task yang statusnya sesuai parameter.
//
// BUG #2: kondisi menggunakan != seharusnya ==.
// Akibatnya fungsi mengembalikan task yang TIDAK memiliki status tersebut.
func (r *MemoryRepository) FindByStatus(status model.Status) ([]model.Task, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []model.Task
	for _, t := range r.tasks {
		if t.Status != status { // BUG: seharusnya == bukan !=
			result = append(result, t)
		}
	}
	return result, nil
}

func (r *MemoryRepository) Delete(id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return false, nil
	}
	delete(r.tasks, id)
	return true, nil
}

func (r *MemoryRepository) Count() (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tasks), nil
}

func (r *MemoryRepository) Close() error { return nil }

// Clear menghapus semua task — hanya untuk testing.
func (r *MemoryRepository) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = make(map[string]model.Task)
}

func (r *MemoryRepository) String() string {
	n, _ := r.Count()
	return fmt.Sprintf("MemoryRepository{count: %d}", n)
}
