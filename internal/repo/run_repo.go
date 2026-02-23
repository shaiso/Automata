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

// RunRepo — репозиторий для работы с runs.
type RunRepo struct {
	pool *pgxpool.Pool
}

// NewRunRepo создаёт новый RunRepo.
func NewRunRepo(pool *pgxpool.Pool) *RunRepo {
	return &RunRepo{pool: pool}
}

// Create создаёт новый run.
func (r *RunRepo) Create(ctx context.Context, run *domain.Run) error {
	inputsJSON, err := json.Marshal(run.Inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	query := `
		INSERT INTO runs (id, flow_id, version, status, inputs, idempotency_key, is_sandbox, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = r.pool.Exec(ctx, query,
		run.ID,
		run.FlowID,
		run.Version,
		run.Status,
		inputsJSON,
		nullString(run.IdempotencyKey),
		run.IsSandbox,
		run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert run: %w", err)
	}
	return nil
}

// GetByID возвращает run по ID.
func (r *RunRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	query := `
		SELECT id, flow_id, version, status, inputs, started_at, finished_at,
		       error, idempotency_key, is_sandbox, created_at
		FROM runs
		WHERE id = $1
	`
	return r.scanRun(r.pool.QueryRow(ctx, query, id))
}

// GetByIdempotencyKey возвращает run по ключу идемпотентности.
func (r *RunRepo) GetByIdempotencyKey(ctx context.Context, flowID uuid.UUID, key string) (*domain.Run, error) {
	query := `
		SELECT id, flow_id, version, status, inputs, started_at, finished_at,
		       error, idempotency_key, is_sandbox, created_at
		FROM runs
		WHERE flow_id = $1 AND idempotency_key = $2
	`
	return r.scanRun(r.pool.QueryRow(ctx, query, flowID, key))
}

// List возвращает список runs с фильтрацией.
func (r *RunRepo) List(ctx context.Context, filter RunFilter) ([]domain.Run, error) {
	query := `
		SELECT id, flow_id, version, status, inputs, started_at, finished_at,
		       error, idempotency_key, is_sandbox, created_at
		FROM runs
		WHERE ($1::uuid IS NULL OR flow_id = $1)
		  AND ($2::text IS NULL OR status = $2::run_status)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query,
		nullUUID(filter.FlowID),
		nullString(string(filter.Status)),
		filter.Limit,
		filter.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		run, err := r.scanRunFromRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

// Update обновляет run.
func (r *RunRepo) Update(ctx context.Context, run *domain.Run) error {
	query := `
		UPDATE runs
		SET status = $2, started_at = $3, finished_at = $4, error = $5
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query,
		run.ID,
		run.Status,
		run.StartedAt,
		run.FinishedAt,
		nullString(run.Error),
	)
	if err != nil {
		return fmt.Errorf("update run: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPending возвращает runs в статусе PENDING.
func (r *RunRepo) ListPending(ctx context.Context, limit int) ([]domain.Run, error) {
	query := `
		SELECT id, flow_id, version, status, inputs, started_at, finished_at,
		       error, idempotency_key, is_sandbox, created_at
		FROM runs
		WHERE status = 'PENDING'
		ORDER BY created_at ASC
		LIMIT $1
	`
	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending runs: %w", err)
	}
	defer rows.Close()

	var runs []domain.Run
	for rows.Next() {
		run, err := r.scanRunFromRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, rows.Err()
}

// --- Helpers ---

// RunFilter — параметры фильтрации runs.
type RunFilter struct {
	FlowID *uuid.UUID
	Status domain.RunStatus
	Limit  int
	Offset int
}

// scanRun сканирует одну строку в Run.
func (r *RunRepo) scanRun(row pgx.Row) (*domain.Run, error) {
	var run domain.Run
	var inputsJSON []byte
	var idempotencyKey *string
	var runError *string

	err := row.Scan(
		&run.ID,
		&run.FlowID,
		&run.Version,
		&run.Status,
		&inputsJSON,
		&run.StartedAt,
		&run.FinishedAt,
		&runError,
		&idempotencyKey,
		&run.IsSandbox,
		&run.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan run: %w", err)
	}

	if inputsJSON != nil {
		if err := json.Unmarshal(inputsJSON, &run.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}
	}

	if idempotencyKey != nil {
		run.IdempotencyKey = *idempotencyKey
	}
	if runError != nil {
		run.Error = *runError
	}

	return &run, nil
}

// scanRunFromRows сканирует строку из rows в Run.
func (r *RunRepo) scanRunFromRows(rows pgx.Rows) (*domain.Run, error) {
	var run domain.Run
	var inputsJSON []byte
	var idempotencyKey *string
	var runError *string

	err := rows.Scan(
		&run.ID,
		&run.FlowID,
		&run.Version,
		&run.Status,
		&inputsJSON,
		&run.StartedAt,
		&run.FinishedAt,
		&runError,
		&idempotencyKey,
		&run.IsSandbox,
		&run.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan run: %w", err)
	}

	if inputsJSON != nil {
		if err := json.Unmarshal(inputsJSON, &run.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}
	}

	if idempotencyKey != nil {
		run.IdempotencyKey = *idempotencyKey
	}
	if runError != nil {
		run.Error = *runError
	}

	return &run, nil
}

// nullString возвращает nil для пустой строки (для NULL в БД).
func nullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nullUUID возвращает nil для пустого UUID.
func nullUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	return id
}
