// Package model mendefinisikan struktur data utama aplikasi Task Manager.
package model

import "time"

// Status merepresentasikan status sebuah task.
type Status string

// Priority merepresentasikan prioritas sebuah task.
type Priority string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// ValidStatuses adalah daftar status yang diizinkan.
var ValidStatuses = []Status{StatusTodo, StatusInProgress, StatusDone}

// ValidPriorities adalah daftar priority yang diizinkan.
var ValidPriorities = []Priority{PriorityLow, PriorityMedium, PriorityHigh}

// Task adalah entitas utama aplikasi.
type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    Priority   `json:"priority"`
	Status      Status     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// CreateTaskRequest adalah payload untuk membuat task baru.
type CreateTaskRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priority    Priority `json:"priority"`
}

// UpdateTaskRequest adalah payload untuk mengupdate task.
type UpdateTaskRequest struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Status      *Status  `json:"status,omitempty"`
}

// TaskListResponse adalah format respons untuk list task.
type TaskListResponse struct {
	Tasks          []Task  `json:"tasks"`
	Total          int     `json:"total"`
	CompletionRate float64 `json:"completion_rate_percent"`
}

// StatsResponse adalah format respons untuk statistik.
type StatsResponse struct {
	Total          int            `json:"total"`
	ByStatus       map[string]int `json:"by_status"`
	ByPriority     map[string]int `json:"by_priority"`
	CompletionRate float64        `json:"completion_rate_percent"`
}

// ErrorResponse adalah format respons untuk error.
type ErrorResponse struct {
	Error string `json:"error"`
}

// HealthResponse adalah format respons untuk health check.
type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}
