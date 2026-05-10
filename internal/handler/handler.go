// Package handler — HTTP handlers menggunakan standard library routing.
package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/service"
)

const version = "1.0.0"

// Handler mengelola semua HTTP endpoint.
type Handler struct {
	svc *service.TaskService
}

// New membuat instance Handler baru.
func New(svc *service.TaskService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mendaftarkan semua route.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", method(http.MethodGet, h.Health))
	mux.HandleFunc("/api/v1/tasks", h.tasksRoot)
	mux.HandleFunc("/api/v1/tasks/", h.taskByID)
	mux.HandleFunc("/api/v1/stats", method(http.MethodGet, h.GetStats))
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.HealthResponse{
		Status:    "ok",
		Service:   "taskflow-api",
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	tasks, err := h.svc.GetAll(statusFilter)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if tasks == nil {
		tasks = []model.Task{}
	}
	writeJSON(w, http.StatusOK, model.TaskListResponse{
		Tasks:          tasks,
		Total:          len(tasks),
		CompletionRate: service.CalculateCompletionRate(tasks),
	})
}

func (h *Handler) CreateTask(w http.ResponseWriter, r *http.Request) {
	var req model.CreateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request body harus berupa JSON yang valid")
		return
	}
	task, err := h.svc.Create(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	task, err := h.svc.GetByID(taskID(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) UpdateTask(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "request body harus berupa JSON yang valid")
		return
	}
	task, err := h.svc.Update(taskID(r), req)
	if err != nil {
		if strings.Contains(err.Error(), "tidak ditemukan") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.svc.Delete(taskID(r))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "task berhasil dihapus",
		"deleted_task": deleted,
	})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "gagal mengambil statistik")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ErrorResponse{Error: msg})
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func (h *Handler) tasksRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/v1/tasks" {
		writeError(w, http.StatusNotFound, "endpoint tidak ditemukan")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.ListTasks(w, r)
	case http.MethodPost:
		h.CreateTask(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
	}
}

func (h *Handler) taskByID(w http.ResponseWriter, r *http.Request) {
	if taskID(r) == "" {
		writeError(w, http.StatusNotFound, "task id tidak valid")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.GetTask(w, r)
	case http.MethodPut:
		h.UpdateTask(w, r)
	case http.MethodDelete:
		h.DeleteTask(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
	}
}

func method(methodName string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != methodName {
			writeError(w, http.StatusMethodNotAllowed, "method tidak diizinkan")
			return
		}
		next(w, r)
	}
}

func taskID(r *http.Request) string {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	if strings.Contains(id, "/") {
		return ""
	}
	return id
}
