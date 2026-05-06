package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/taskflow/api/internal/model"
)

type fakePostgresPool struct {
	tasks    map[string]model.Task
	execErr  error
	queryErr error
	rowErr   error
	closed   bool
	lastSQL  string
}

func newFakePostgresPool() *fakePostgresPool {
	return &fakePostgresPool{tasks: make(map[string]model.Task)}
}

func (p *fakePostgresPool) Ping(context.Context) error {
	return nil
}

func (p *fakePostgresPool) Exec(_ context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	p.lastSQL = sql
	if p.execErr != nil {
		return pgconn.CommandTag{}, p.execErr
	}
	switch {
	case strings.Contains(sql, "INSERT INTO tasks"):
		completedAt, _ := arguments[7].(*time.Time)
		p.tasks[arguments[0].(string)] = model.Task{
			ID:          arguments[0].(string),
			Title:       arguments[1].(string),
			Description: arguments[2].(string),
			Priority:    arguments[3].(model.Priority),
			Status:      arguments[4].(model.Status),
			CreatedAt:   arguments[5].(time.Time),
			UpdatedAt:   arguments[6].(time.Time),
			CompletedAt: completedAt,
		}
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.Contains(sql, "DELETE FROM tasks"):
		if _, ok := p.tasks[arguments[0].(string)]; ok {
			delete(p.tasks, arguments[0].(string))
			return pgconn.NewCommandTag("DELETE 1"), nil
		}
		return pgconn.NewCommandTag("DELETE 0"), nil
	case strings.Contains(sql, "TRUNCATE TABLE tasks"):
		p.tasks = make(map[string]model.Task)
		return pgconn.NewCommandTag("TRUNCATE TABLE"), nil
	default:
		return pgconn.NewCommandTag("CREATE TABLE"), nil
	}
}

func (p *fakePostgresPool) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	p.lastSQL = sql
	if p.queryErr != nil {
		return nil, p.queryErr
	}
	tasks := make([]model.Task, 0, len(p.tasks))
	for _, task := range p.tasks {
		if strings.Contains(sql, "WHERE status") {
			if task.Status != args[0].(model.Status) {
				continue
			}
		}
		tasks = append(tasks, task)
	}
	return &fakeRows{tasks: tasks, index: -1}, nil
}

func (p *fakePostgresPool) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	p.lastSQL = sql
	if p.rowErr != nil {
		return fakeRow{err: p.rowErr}
	}
	if strings.Contains(sql, "COUNT") {
		return fakeRow{count: len(p.tasks)}
	}
	task, ok := p.tasks[args[0].(string)]
	if !ok {
		return fakeRow{err: pgx.ErrNoRows}
	}
	return fakeRow{task: task}
}

func (p *fakePostgresPool) Close() {
	p.closed = true
}

type fakeRow struct {
	task  model.Task
	count int
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) == 1 {
		*dest[0].(*int) = r.count
		return nil
	}
	return assignTask(dest, r.task)
}

type fakeRows struct {
	tasks   []model.Task
	index   int
	err     error
	scanErr error
	closed  bool
}

func (r *fakeRows) Close() {
	r.closed = true
}

func (r *fakeRows) Err() error {
	return r.err
}

func (r *fakeRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT")
}

func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *fakeRows) Next() bool {
	if r.closed {
		return false
	}
	r.index++
	if r.index >= len(r.tasks) {
		r.closed = true
		return false
	}
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	if r.index < 0 || r.index >= len(r.tasks) {
		return errors.New("scan called without current row")
	}
	return assignTask(dest, r.tasks[r.index])
}

func (r *fakeRows) Values() ([]any, error) {
	return nil, nil
}

func (r *fakeRows) RawValues() [][]byte {
	return nil
}

func (r *fakeRows) Conn() *pgx.Conn {
	return nil
}

func assignTask(dest []any, task model.Task) error {
	*dest[0].(*string) = task.ID
	*dest[1].(*string) = task.Title
	*dest[2].(*string) = task.Description
	*dest[3].(*model.Priority) = task.Priority
	*dest[4].(*model.Status) = task.Status
	*dest[5].(*time.Time) = task.CreatedAt
	*dest[6].(*time.Time) = task.UpdatedAt
	*dest[7].(**time.Time) = task.CompletedAt
	return nil
}

func TestPostgresRepository_WithFakePool(t *testing.T) {
	pool := newFakePostgresPool()
	repo := &PostgresRepository{pool: pool}
	now := time.Now().UTC()
	doneAt := now.Add(time.Minute)

	if err := repo.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	tasks := []model.Task{
		{ID: "p1", Title: "Todo", Description: "A", Priority: model.PriorityLow, Status: model.StatusTodo, CreatedAt: now, UpdatedAt: now},
		{ID: "p2", Title: "Done", Description: "B", Priority: model.PriorityHigh, Status: model.StatusDone, CreatedAt: now, UpdatedAt: now, CompletedAt: &doneAt},
		{ID: "p3", Title: "Progress", Description: "C", Priority: model.PriorityMedium, Status: model.StatusInProgress, CreatedAt: now, UpdatedAt: now},
	}
	for _, task := range tasks {
		if err := repo.Save(task); err != nil {
			t.Fatalf("Save(%s) error = %v", task.ID, err)
		}
	}

	got, ok, err := repo.FindByID("p2")
	if err != nil || !ok {
		t.Fatalf("FindByID(p2) ok=%v err=%v", ok, err)
	}
	if got.Status != model.StatusDone || got.CompletedAt == nil {
		t.Fatalf("FindByID(p2) = %+v, want done with completed_at", got)
	}

	_, ok, err = repo.FindByID("missing")
	if err != nil || ok {
		t.Fatalf("FindByID(missing) ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("FindAll() len = %d, want 3", len(all))
	}

	done, err := repo.FindByStatus(model.StatusDone)
	if err != nil {
		t.Fatalf("FindByStatus(done) error = %v", err)
	}
	if len(done) != 1 || done[0].Status != model.StatusDone {
		t.Fatalf("FindByStatus(done) = %+v, want one done task", done)
	}
	if !strings.Contains(pool.lastSQL, "status = $1") || strings.Contains(pool.lastSQL, "status != $1") {
		t.Fatalf("FindByStatus SQL = %q, want equality filter", pool.lastSQL)
	}

	count, err := repo.Count()
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("Count() = %d, want 3", count)
	}

	deleted, err := repo.Delete("p1")
	if err != nil || !deleted {
		t.Fatalf("Delete(p1) deleted=%v err=%v", deleted, err)
	}
	deleted, err = repo.Delete("missing")
	if err != nil || deleted {
		t.Fatalf("Delete(missing) deleted=%v err=%v, want false nil", deleted, err)
	}

	repo.TruncateForTest(t)
	count, err = repo.Count()
	if err != nil || count != 0 {
		t.Fatalf("Count() after truncate = %d err=%v, want 0 nil", count, err)
	}

	if err := repo.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !pool.closed {
		t.Fatal("Close() did not close pool")
	}
}

func TestPostgresRepository_ErrorPaths(t *testing.T) {
	wantErr := errors.New("database failed")

	t.Run("NewPostgresRepository rejects invalid url", func(t *testing.T) {
		if _, err := NewPostgresRepository("://bad-url"); err == nil {
			t.Fatal("NewPostgresRepository() error = nil, want error")
		}
	})

	t.Run("exec errors are returned", func(t *testing.T) {
		repo := &PostgresRepository{pool: &fakePostgresPool{tasks: map[string]model.Task{}, execErr: wantErr}}
		if err := repo.Migrate(); err == nil {
			t.Fatal("Migrate() error = nil, want error")
		}
		if err := repo.Save(model.Task{ID: "x"}); err == nil {
			t.Fatal("Save() error = nil, want error")
		}
		if _, err := repo.Delete("x"); err == nil {
			t.Fatal("Delete() error = nil, want error")
		}
	})

	t.Run("query errors are wrapped", func(t *testing.T) {
		repo := &PostgresRepository{pool: &fakePostgresPool{tasks: map[string]model.Task{}, queryErr: wantErr}}
		if _, err := repo.FindAll(); err == nil {
			t.Fatal("FindAll() error = nil, want error")
		}
		if _, err := repo.FindByStatus(model.StatusDone); err == nil {
			t.Fatal("FindByStatus() error = nil, want error")
		}
	})

	t.Run("row errors are wrapped", func(t *testing.T) {
		repo := &PostgresRepository{pool: &fakePostgresPool{tasks: map[string]model.Task{}, rowErr: wantErr}}
		if _, _, err := repo.FindByID("x"); err == nil {
			t.Fatal("FindByID() error = nil, want error")
		}
		if _, err := repo.Count(); err == nil {
			t.Fatal("Count() error = nil, want error")
		}
	})

	t.Run("collectTasks propagates row errors", func(t *testing.T) {
		if _, err := collectTasks(&fakeRows{err: wantErr, index: -1}); err == nil {
			t.Fatal("collectTasks() error = nil, want row error")
		}
		if _, err := collectTasks(&fakeRows{tasks: []model.Task{{ID: "x"}}, index: -1, scanErr: wantErr}); err == nil {
			t.Fatal("collectTasks() error = nil, want scan error")
		}
	})
}

var _ postgresPool = (*fakePostgresPool)(nil)
var _ pgx.Rows = (*fakeRows)(nil)
var _ pgx.Row = fakeRow{}
