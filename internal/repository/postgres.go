// Package repository — implementasi PostgreSQL menggunakan pgx/v5.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/taskflow/api/internal/model"
)

// Pastikan PostgresRepository mengimplementasikan interface TaskRepository.
var _ TaskRepository = (*PostgresRepository)(nil)

// PostgresRepository menyimpan task di PostgreSQL menggunakan pgxpool.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository membuat koneksi pool ke PostgreSQL.
// databaseURL format: "postgres://user:pass@host:5432/dbname?sslmode=disable"
func NewPostgresRepository(databaseURL string) (*PostgresRepository, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("gagal membuat connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database tidak dapat dijangkau: %w", err)
	}
	return &PostgresRepository{pool: pool}, nil
}

// Migrate menjalankan skema database. Dipanggil saat startup.
func (r *PostgresRepository) Migrate() error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id           VARCHAR(64)  PRIMARY KEY,
			title        VARCHAR(200) NOT NULL,
			description  TEXT         NOT NULL DEFAULT '',
			priority     VARCHAR(20)  NOT NULL DEFAULT 'medium',
			status       VARCHAR(20)  NOT NULL DEFAULT 'todo',
			created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
			completed_at TIMESTAMPTZ  DEFAULT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_tasks_status   ON tasks(status);
		CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority);
	`)
	return err
}

func (r *PostgresRepository) Save(task model.Task) error {
	ctx := context.Background()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO tasks (id, title, description, priority, status, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			title        = EXCLUDED.title,
			description  = EXCLUDED.description,
			priority     = EXCLUDED.priority,
			status       = EXCLUDED.status,
			updated_at   = EXCLUDED.updated_at,
			completed_at = EXCLUDED.completed_at
	`, task.ID, task.Title, task.Description, task.Priority,
		task.Status, task.CreatedAt, task.UpdatedAt, task.CompletedAt)
	return err
}

func (r *PostgresRepository) FindByID(id string) (model.Task, bool, error) {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx,
		`SELECT id, title, description, priority, status, created_at, updated_at, completed_at
		 FROM tasks WHERE id = $1`, id)

	task, err := scanTask(row)
	if err == pgx.ErrNoRows {
		return model.Task{}, false, nil
	}
	if err != nil {
		return model.Task{}, false, fmt.Errorf("query FindByID: %w", err)
	}
	return task, true, nil
}

func (r *PostgresRepository) FindAll() ([]model.Task, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, description, priority, status, created_at, updated_at, completed_at
		 FROM tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query FindAll: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

// FindByStatus mengembalikan task yang statusnya sesuai parameter.
//
// BUG #2 (POSTGRES): query menggunakan operator != seharusnya =.
// Akibatnya query ini mengembalikan semua task yang BUKAN status tersebut.
func (r *PostgresRepository) FindByStatus(status model.Status) ([]model.Task, error) {
	ctx := context.Background()
	rows, err := r.pool.Query(ctx,
		`SELECT id, title, description, priority, status, created_at, updated_at, completed_at
		 FROM tasks WHERE status != $1 ORDER BY created_at DESC`, // BUG: != seharusnya =
		status)
	if err != nil {
		return nil, fmt.Errorf("query FindByStatus: %w", err)
	}
	defer rows.Close()
	return collectTasks(rows)
}

func (r *PostgresRepository) Delete(id string) (bool, error) {
	ctx := context.Background()
	tag, err := r.pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return false, fmt.Errorf("query Delete: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PostgresRepository) Count() (int, error) {
	ctx := context.Background()
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM tasks`).Scan(&count)
	return count, err
}

func (r *PostgresRepository) Close() error {
	r.pool.Close()
	return nil
}

// TruncateForTest menghapus semua data — HANYA untuk integration test.
func (r *PostgresRepository) TruncateForTest(t interface{ Helper(); Fatalf(string, ...interface{}) }) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), `TRUNCATE TABLE tasks`); err != nil {
		t.Fatalf("TruncateForTest error: %v", err)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func scanTask(row pgx.Row) (model.Task, error) {
	var t model.Task
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Priority,
		&t.Status, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
	)
	return t, err
}

func collectTasks(rows pgx.Rows) ([]model.Task, error) {
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Priority,
			&t.Status, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
		); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
