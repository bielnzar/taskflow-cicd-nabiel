package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/taskflow/api/internal/handler"
	"github.com/taskflow/api/internal/model"
	"github.com/taskflow/api/internal/repository"
	"github.com/taskflow/api/internal/service"
)

// ── Test Setup ────────────────────────────────────────────────────────────────

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	repo := repository.NewTaskRepository()
	svc := service.NewTaskService(repo)
	h := handler.New(svc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func doRequest(t *testing.T, srv *httptest.Server, method, path string, body interface{}) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

// ── [BEST] Health Check ───────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodGet, "/health", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}
	var body model.HealthResponse
	decodeBody(t, resp, &body)
	if body.Status != "ok" {
		t.Errorf("health.status = %q, want ok", body.Status)
	}
	if body.Service == "" || body.Version == "" || body.Timestamp == "" {
		t.Error("health response harus memiliki service, version, timestamp")
	}
}

// ── [BEST] HTTP Conventions ──────────────────────────────────────────────────

func TestHTTPConventions(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	t.Run("POST task mengembalikan 201 Created", func(t *testing.T) {
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": "Test"})
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want 201", resp.StatusCode)
		}
	})

	t.Run("semua endpoint mengembalikan Content-Type JSON", func(t *testing.T) {
		endpoints := []string{"/health", "/api/v1/tasks", "/api/v1/stats"}
		for _, ep := range endpoints {
			resp := doRequest(t, srv, http.MethodGet, ep, nil)
			ct := resp.Header.Get("Content-Type")
			if ct == "" {
				t.Errorf("endpoint %s tidak mengembalikan Content-Type", ep)
			}
		}
	})

	t.Run("GET task tidak ada → 404 dengan field error", func(t *testing.T) {
		resp := doRequest(t, srv, http.MethodGet, "/api/v1/tasks/tidak-ada", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
		var body model.ErrorResponse
		decodeBody(t, resp, &body)
		if body.Error == "" {
			t.Error("response 404 harus memiliki field 'error'")
		}
	})

	t.Run("POST tanpa body → 400 dengan field error", func(t *testing.T) {
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks", nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("DELETE task mengembalikan deleted_task untuk audit trail", func(t *testing.T) {
		// Create dulu
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": "Akan Dihapus"})
		var task model.Task
		decodeBody(t, resp, &task)

		// Delete
		delResp := doRequest(t, srv, http.MethodDelete, "/api/v1/tasks/"+task.ID, nil)
		if delResp.StatusCode != http.StatusOK {
			t.Errorf("delete status = %d, want 200", delResp.StatusCode)
		}
		var result map[string]interface{}
		decodeBody(t, delResp, &result)
		if _, ok := result["deleted_task"]; !ok {
			t.Error("DELETE response harus ada field 'deleted_task' untuk audit trail")
		}
	})
}

// ── [CICD] Full Lifecycle via HTTP ───────────────────────────────────────────
// [CICD] Integration test: simulasi rangkaian request HTTP seperti di production.

func TestTaskLifecycleHTTP(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	// 1. Buat task
	resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks", map[string]interface{}{
		"title":    "CI/CD Integration Test",
		"priority": "high",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var task model.Task
	decodeBody(t, resp, &task)

	// 2. Verifikasi task ada
	getResp := doRequest(t, srv, http.MethodGet, "/api/v1/tasks/"+task.ID, nil)
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", getResp.StatusCode)
	}

	// 3. Update ke in_progress
	inProg := "in_progress"
	upResp := doRequest(t, srv, http.MethodPut, "/api/v1/tasks/"+task.ID,
		map[string]string{"status": inProg})
	if upResp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want 200", upResp.StatusCode)
	}

	// 4. Update ke done
	done := "done"
	doneResp := doRequest(t, srv, http.MethodPut, "/api/v1/tasks/"+task.ID,
		map[string]string{"status": done})
	var doneTask model.Task
	decodeBody(t, doneResp, &doneTask)
	if doneTask.CompletedAt == nil {
		t.Error("[CICD] completed_at harus terisi setelah status = done")
	}

	// 5. Stats harus konsisten
	statsResp := doRequest(t, srv, http.MethodGet, "/api/v1/stats", nil)
	var stats model.StatsResponse
	decodeBody(t, statsResp, &stats)
	if stats.ByStatus["done"] != 1 {
		t.Errorf("[CICD] stats.by_status.done = %d, want 1", stats.ByStatus["done"])
	}

	// 6. Delete dan verifikasi terhapus
	doRequest(t, srv, http.MethodDelete, "/api/v1/tasks/"+task.ID, nil)
	finalResp := doRequest(t, srv, http.MethodGet, "/api/v1/tasks/"+task.ID, nil)
	if finalResp.StatusCode != http.StatusNotFound {
		t.Error("[CICD] task seharusnya tidak ditemukan setelah dihapus")
	}
}

// ── [CICD] Smoke Test ─────────────────────────────────────────────────────────
// [CICD] Smoke test minimal — dijalankan otomatis setelah setiap deployment.
// Jika gagal → pipeline harus trigger rollback.

func TestSmokeTest(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	// Health check harus 200
	if resp := doRequest(t, srv, http.MethodGet, "/health", nil); resp.StatusCode != 200 {
		t.Fatalf("SMOKE FAIL: /health = %d, want 200", resp.StatusCode)
	}
	// Bisa create task
	if resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
		map[string]string{"title": "Smoke"}); resp.StatusCode != 201 {
		t.Fatalf("SMOKE FAIL: POST /tasks = %d, want 201", resp.StatusCode)
	}
	// Stats endpoint berjalan
	if resp := doRequest(t, srv, http.MethodGet, "/api/v1/stats", nil); resp.StatusCode != 200 {
		t.Fatalf("SMOKE FAIL: /stats = %d, want 200", resp.StatusCode)
	}
}

// ── [CICD] Rollback Idempotency ───────────────────────────────────────────────

func TestRollbackIdempotencyHTTP(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
		map[string]string{"title": "Idempotent"})
	var task model.Task
	decodeBody(t, resp, &task)

	// Update status yang sama berkali-kali (simulasi re-deploy)
	for i := 0; i < 5; i++ {
		upResp := doRequest(t, srv, http.MethodPut, "/api/v1/tasks/"+task.ID,
			map[string]string{"status": "in_progress"})
		if upResp.StatusCode != http.StatusOK {
			t.Errorf("update ke-%d gagal: status = %d", i+1, upResp.StatusCode)
		}
	}
}

// ── [SECOPS] Input Validation ─────────────────────────────────────────────────

func TestSecOpsInputValidation(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	t.Run("SQL injection di title — disimpan sebagai string biasa", func(t *testing.T) {
		payload := "Task'; DROP TABLE tasks; --"
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": payload})
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("SQL injection payload harus diterima sebagai string, status = %d", resp.StatusCode)
		}
		var task model.Task
		decodeBody(t, resp, &task)
		if task.Title != payload {
			t.Errorf("title tidak tersimpan dengan benar: %q", task.Title)
		}
	})

	t.Run("XSS payload di title — disimpan sebagai string biasa", func(t *testing.T) {
		payload := "<script>alert('xss')</script>"
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": payload})
		// API layer boleh menerima — sanitasi di frontend
		if resp.StatusCode >= 500 {
			t.Errorf("XSS payload menyebabkan server error: %d", resp.StatusCode)
		}
	})

	t.Run("title 10.000 karakter — tidak menyebabkan 500", func(t *testing.T) {
		longTitle := string(make([]byte, 10000))
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": longTitle})
		// Boleh 201 (disimpan) atau 400 (ditolak), asal bukan 500
		if resp.StatusCode == http.StatusInternalServerError {
			t.Error("title panjang tidak boleh menyebabkan 500 Internal Server Error")
		}
	})

	t.Run("unicode & emoji di title", func(t *testing.T) {
		title := "Belajar DevOps 🚀 — مرحبا — 你好"
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": title})
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("unicode title harus diterima, status = %d", resp.StatusCode)
		}
	})

	t.Run("title kosong → 400 bukan 500", func(t *testing.T) {
		resp := doRequest(t, srv, http.MethodPost, "/api/v1/tasks",
			map[string]string{"title": ""})
		if resp.StatusCode == http.StatusInternalServerError {
			t.Error("title kosong tidak boleh menyebabkan 500")
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("title kosong harus 400, got %d", resp.StatusCode)
		}
	})
}

// ── [SECOPS] Response Safety ──────────────────────────────────────────────────

func TestSecOpsResponseSafety(t *testing.T) {
	srv := newServer(t)
	defer srv.Close()

	t.Run("404 tidak expose stack trace Go", func(t *testing.T) {
		resp := doRequest(t, srv, http.MethodGet, "/api/v1/tasks/tidak-ada", nil)
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		body := buf.String()
		if contains(body, "goroutine") || contains(body, "panic") || contains(body, ".go:") {
			t.Error("respons 404 mengandung stack trace Go — informasi sensitif!")
		}
	})

	t.Run("error response selalu ada field error", func(t *testing.T) {
		errorCases := []struct {
			method string
			path   string
			body   interface{}
		}{
			{http.MethodPost, "/api/v1/tasks", map[string]string{"title": ""}},
			{http.MethodGet, "/api/v1/tasks/tidak-ada", nil},
			{http.MethodDelete, "/api/v1/tasks/tidak-ada", nil},
		}
		for _, ec := range errorCases {
			resp := doRequest(t, srv, ec.method, ec.path, ec.body)
			if resp.StatusCode < 400 {
				continue
			}
			var errResp model.ErrorResponse
			decodeBody(t, resp, &errResp)
			if errResp.Error == "" {
				t.Errorf("%s %s: respons error tidak memiliki field 'error'", ec.method, ec.path)
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > 0 && func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// ── [TODO] Wajib Ditambah Mahasiswa ─────────────────────────────────────────
// TODO: Tambahkan minimal 2 test baru:
// - TestListTasks_WithStatusFilter (GET /api/v1/tasks?status=done)
// - TestUpdateTask_TitleOnly (update hanya title tanpa ubah status)
// - TestStats_ConsistencyWithTaskList (total di /stats == total di /tasks)
// - TestCreateMultipleTasks_UniqueIDs (50 task, semua ID unik)
