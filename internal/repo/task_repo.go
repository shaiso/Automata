package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shaiso/Automata/internal/domain"
)

// TaskRepo — репозиторий для работы с tasks.
type TaskRepo struct {
	pool *pgxpool.Pool
}

// NewTaskRepo создаёт новый TaskRepo.
func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{pool: pool}
}

// Create создаёт новый task.
func (r *TaskRepo) Create(ctx context.Context, task *domain.Task) error {
	payloadJSON, err := json.Marshal(task.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	query := `
		INSERT INTO tasks (id, run_id, step_id, name, type, attempt, status, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err = r.pool.Exec(ctx, query,
		task.ID,
		task.RunID,
		task.StepID,
		task.Name,
		task.Type,
		task.Attempt,
		task.Status,
		payloadJSON,
		task.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetByID возвращает task по ID.
func (r *TaskRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	query := `
		SELECT id, run_id, step_id, name, type, attempt, status, payload, outputs,
		       result_ref, started_at, finished_at, error, created_at
		FROM tasks
		WHERE id = $1
	`
	return r.scanTask(r.pool.QueryRow(ctx, query, id))
}

// ListByRunID возвращает все tasks для run.
func (r *TaskRepo) ListByRunID(ctx context.Context, runID uuid.UUID) ([]domain.Task, error) {
	query := `
		SELECT id, run_id, step_id, name, type, attempt, status, payload, outputs,
		       result_ref, started_at, finished_at, error, created_at
		FROM tasks
		WHERE run_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, query, runID)
	if err != nil {
		return nil, fmt.Errorf("list tasks by run_id: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		task, err := r.scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

// GetByRunAndStepID возвращает task по run_id и step_id.
func (r *TaskRepo) GetByRunAndStepID(ctx context.Context, runID uuid.UUID, stepID string) (*domain.Task, error) {
	query := `
		SELECT id, run_id, step_id, name, type, attempt, status, payload, outputs,
		       result_ref, started_at, finished_at, error, created_at
		FROM tasks
		WHERE run_id = $1 AND step_id = $2
	`
	return r.scanTask(r.pool.QueryRow(ctx, query, runID, stepID))
}

// Update обновляет task.
func (r *TaskRepo) Update(ctx context.Context, task *domain.Task) error {
	outputsJSON, err := json.Marshal(task.Outputs)
	if err != nil {
		return fmt.Errorf("marshal outputs: %w", err)
	}

	query := `
		UPDATE tasks
		SET attempt = $2, status = $3, outputs = $4, result_ref = $5,
		    started_at = $6, finished_at = $7, error = $8
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query,
		task.ID,
		task.Attempt,
		task.Status,
		outputsJSON,
		nullString(task.ResultRef),
		task.StartedAt,
		task.FinishedAt,
		nullString(task.Error),
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListQueued возвращает tasks в статусе QUEUED.
func (r *TaskRepo) ListQueued(ctx context.Context, limit int) ([]domain.Task, error) {
	query := `
		SELECT id, run_id, step_id, name, type, attempt, status, payload, outputs,
		       result_ref, started_at, finished_at, error, created_at
		FROM tasks
		WHERE status = 'QUEUED'
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list queued tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		task, err := r.scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, *task)
	}
	return tasks, rows.Err()
}

// CountByRunAndStatus возвращает количество tasks по статусу для run.
func (r *TaskRepo) CountByRunAndStatus(ctx context.Context, runID uuid.UUID, status domain.TaskStatus) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM tasks WHERE run_id = $1 AND status = $2
	`, runID, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count tasks: %w", err)
	}
	return count, nil
}

// --- Helpers ---

func (r *TaskRepo) scanTask(row pgx.Row) (*domain.Task, error) {
	var task domain.Task
	var payloadJSON, outputsJSON []byte
	var resultRef, taskError *string

	err := row.Scan(
		&task.ID,
		&task.RunID,
		&task.StepID,
		&task.Name,
		&task.Type,
		&task.Attempt,
		&task.Status,
		&payloadJSON,
		&outputsJSON,
		&resultRef,
		&task.StartedAt,
		&task.FinishedAt,
		&taskError,
		&task.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}

	if payloadJSON != nil {
		if err := json.Unmarshal(payloadJSON, &task.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	if outputsJSON != nil {
		if err := json.Unmarshal(outputsJSON, &task.Outputs); err != nil {
			return nil, fmt.Errorf("unmarshal outputs: %w", err)
		}
	}
	if resultRef != nil {
		task.ResultRef = *resultRef
	}
	if taskError != nil {
		task.Error = *taskError
	}

	return &task, nil
}

func (r *TaskRepo) scanTaskFromRows(rows pgx.Rows) (*domain.Task, error) {
	var task domain.Task
	var payloadJSON, outputsJSON []byte
	var resultRef, taskError *string

	err := rows.Scan(
		&task.ID,
		&task.RunID,
		&task.StepID,
		&task.Name,
		&task.Type,
		&task.Attempt,
		&task.Status,
		&payloadJSON,
		&outputsJSON,
		&resultRef,
		&task.StartedAt,
		&task.FinishedAt,
		&taskError,
		&task.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan task: %w", err)
	}

	if payloadJSON != nil {
		if err := json.Unmarshal(payloadJSON, &task.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	if outputsJSON != nil {
		if err := json.Unmarshal(outputsJSON, &task.Outputs); err != nil {
			return nil, fmt.Errorf("unmarshal outputs: %w", err)
		}
	}
	if resultRef != nil {
		task.ResultRef = *resultRef
	}
	if taskError != nil {
		task.Error = *taskError
	}

	return &task, nil
}
