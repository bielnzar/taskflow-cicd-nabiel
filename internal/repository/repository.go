// Package repository mendefinisikan interface penyimpanan data.
package repository

import "github.com/taskflow/api/internal/model"

// TaskRepository adalah kontrak yang harus dipenuhi oleh semua implementasi storage.
// Saat ini ada dua implementasi:
//   - MemoryRepository  : penyimpanan in-memory, digunakan untuk unit test
//   - PostgresRepository: penyimpanan ke PostgreSQL, digunakan di production
type TaskRepository interface {
	Save(task model.Task) error
	FindByID(id string) (model.Task, bool, error)
	FindAll() ([]model.Task, error)
	FindByStatus(status model.Status) ([]model.Task, error)
	Delete(id string) (bool, error)
	Count() (int, error)
	Close() error
}
