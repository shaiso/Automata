package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shaiso/Automata/internal/domain"
)

// ScheduleRepo — репозиторий для работы с schedules.
type ScheduleRepo struct {
	pool *pgxpool.Pool
}

// NewScheduleRepo создаёт новый ScheduleRepo.
func NewScheduleRepo(pool *pgxpool.Pool) *ScheduleRepo {
	return &ScheduleRepo{pool: pool}
}

// Create создаёт новый schedule.
func (r *ScheduleRepo) Create(ctx context.Context, schedule *domain.Schedule) error {
	inputsJSON, err := json.Marshal(schedule.Inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	query := `
		INSERT INTO schedules (id, flow_id, name, cron_expr, interval_sec, timezone,
		                       enabled, next_due_at, inputs, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.pool.Exec(ctx, query,
		schedule.ID,
		schedule.FlowID,
		nullString(schedule.Name),
		nullString(schedule.CronExpr),
		nullInt(schedule.IntervalSec),
		schedule.Timezone,
		schedule.Enabled,
		schedule.NextDueAt,
		inputsJSON,
		schedule.CreatedAt,
		schedule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert schedule: %w", err)
	}
	return nil
}

// GetByID возвращает schedule по ID.
func (r *ScheduleRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Schedule, error) {
	query := `
		SELECT id, flow_id, name, cron_expr, interval_sec, timezone, enabled,
		       next_due_at, last_run_at, last_run_id, inputs, created_at, updated_at
		FROM schedules
		WHERE id = $1
	`
	return r.scanSchedule(r.pool.QueryRow(ctx, query, id))
}

// List возвращает список schedules с фильтрацией.
func (r *ScheduleRepo) List(ctx context.Context, filter ScheduleFilter) ([]domain.Schedule, error) {
	query := `
		SELECT id, flow_id, name, cron_expr, interval_sec, timezone, enabled,
		       next_due_at, last_run_at, last_run_id, inputs, created_at, updated_at
		FROM schedules
		WHERE ($1::uuid IS NULL OR flow_id = $1)
		  AND ($2::boolean IS NULL OR enabled = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.pool.Query(ctx, query,
		nullUUID(filter.FlowID),
		filter.Enabled,
		filter.Limit,
		filter.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}
	defer rows.Close()

	var schedules []domain.Schedule
	for rows.Next() {
		schedule, err := r.scanScheduleFromRows(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}
	return schedules, rows.Err()
}

// ListDue возвращает schedules, готовые к выполнению.
func (r *ScheduleRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]domain.Schedule, error) {
	query := `
		SELECT id, flow_id, name, cron_expr, interval_sec, timezone, enabled,
		       next_due_at, last_run_at, last_run_id, inputs, created_at, updated_at
		FROM schedules
		WHERE enabled = true
		  AND next_due_at IS NOT NULL
		  AND next_due_at <= $1
		ORDER BY next_due_at ASC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due schedules: %w", err)
	}
	defer rows.Close()

	var schedules []domain.Schedule
	for rows.Next() {
		schedule, err := r.scanScheduleFromRows(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, *schedule)
	}
	return schedules, rows.Err()
}

// Update обновляет schedule.
func (r *ScheduleRepo) Update(ctx context.Context, schedule *domain.Schedule) error {
	inputsJSON, err := json.Marshal(schedule.Inputs)
	if err != nil {
		return fmt.Errorf("marshal inputs: %w", err)
	}

	query := `
		UPDATE schedules
		SET name = $2, cron_expr = $3, interval_sec = $4, timezone = $5,
		    enabled = $6, next_due_at = $7, last_run_at = $8, last_run_id = $9,
		    inputs = $10, updated_at = $11
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query,
		schedule.ID,
		nullString(schedule.Name),
		nullString(schedule.CronExpr),
		nullInt(schedule.IntervalSec),
		schedule.Timezone,
		schedule.Enabled,
		schedule.NextDueAt,
		schedule.LastRunAt,
		schedule.LastRunID,
		inputsJSON,
		schedule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update schedule: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет schedule.
func (r *ScheduleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete schedule: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetEnabled включает/выключает schedule.
func (r *ScheduleRepo) SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE schedules SET enabled = $2, updated_at = NOW() WHERE id = $1
	`, id, enabled)
	if err != nil {
		return fmt.Errorf("set enabled: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Helpers ---

// ScheduleFilter — параметры фильтрации schedules.
type ScheduleFilter struct {
	FlowID  *uuid.UUID
	Enabled *bool
	Limit   int
	Offset  int
}

func (r *ScheduleRepo) scanSchedule(row pgx.Row) (*domain.Schedule, error) {
	var s domain.Schedule
	var name, cronExpr *string
	var intervalSec *int
	var inputsJSON []byte

	err := row.Scan(
		&s.ID,
		&s.FlowID,
		&name,
		&cronExpr,
		&intervalSec,
		&s.Timezone,
		&s.Enabled,
		&s.NextDueAt,
		&s.LastRunAt,
		&s.LastRunID,
		&inputsJSON,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan schedule: %w", err)
	}

	if name != nil {
		s.Name = *name
	}
	if cronExpr != nil {
		s.CronExpr = *cronExpr
	}
	if intervalSec != nil {
		s.IntervalSec = *intervalSec
	}
	if inputsJSON != nil {
		if err := json.Unmarshal(inputsJSON, &s.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}
	}

	return &s, nil
}

func (r *ScheduleRepo) scanScheduleFromRows(rows pgx.Rows) (*domain.Schedule, error) {
	var s domain.Schedule
	var name, cronExpr *string
	var intervalSec *int
	var inputsJSON []byte

	err := rows.Scan(
		&s.ID,
		&s.FlowID,
		&name,
		&cronExpr,
		&intervalSec,
		&s.Timezone,
		&s.Enabled,
		&s.NextDueAt,
		&s.LastRunAt,
		&s.LastRunID,
		&inputsJSON,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan schedule: %w", err)
	}

	if name != nil {
		s.Name = *name
	}
	if cronExpr != nil {
		s.CronExpr = *cronExpr
	}
	if intervalSec != nil {
		s.IntervalSec = *intervalSec
	}
	if inputsJSON != nil {
		if err := json.Unmarshal(inputsJSON, &s.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal inputs: %w", err)
		}
	}

	return &s, nil
}

// nullInt возвращает nil для нулевого int.
func nullInt(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}
