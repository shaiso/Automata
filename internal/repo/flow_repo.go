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

// FlowRepo — репозиторий для работы с flows и flow_versions.
type FlowRepo struct {
	pool *pgxpool.Pool
}

// NewFlowRepo создаёт новый FlowRepo.
func NewFlowRepo(pool *pgxpool.Pool) *FlowRepo {
	return &FlowRepo{pool: pool}
}

// --- Flow CRUD ---

// Create создаёт новый flow.
func (r *FlowRepo) Create(ctx context.Context, flow *domain.Flow) error {
	query := `
		INSERT INTO flows (id, name, is_active, created_at)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query,
		flow.ID,
		flow.Name,
		flow.IsActive,
		flow.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert flow: %w", err)
	}
	return nil
}

// GetByID возвращает flow по ID.
func (r *FlowRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Flow, error) {
	query := `
		SELECT id, name, is_active, created_at
		FROM flows
		WHERE id = $1
	`
	var flow domain.Flow
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&flow.ID,
		&flow.Name,
		&flow.IsActive,
		&flow.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get flow by id: %w", err)
	}
	return &flow, nil
}

// GetByName возвращает flow по имени.
func (r *FlowRepo) GetByName(ctx context.Context, name string) (*domain.Flow, error) {
	query := `
		SELECT id, name, is_active, created_at
		FROM flows
		WHERE name = $1
	`
	var flow domain.Flow
	err := r.pool.QueryRow(ctx, query, name).Scan(
		&flow.ID,
		&flow.Name,
		&flow.IsActive,
		&flow.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get flow by name: %w", err)
	}
	return &flow, nil
}

// List возвращает список всех flows.
func (r *FlowRepo) List(ctx context.Context) ([]domain.Flow, error) {
	query := `
		SELECT id, name, is_active, created_at
		FROM flows
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list flows: %w", err)
	}
	defer rows.Close()

	var flows []domain.Flow
	for rows.Next() {
		var flow domain.Flow
		if err := rows.Scan(
			&flow.ID,
			&flow.Name,
			&flow.IsActive,
			&flow.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan flow: %w", err)
		}
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

// Update обновляет flow.
func (r *FlowRepo) Update(ctx context.Context, flow *domain.Flow) error {
	query := `
		UPDATE flows
		SET name = $2, is_active = $3
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query, flow.ID, flow.Name, flow.IsActive)
	if err != nil {
		return fmt.Errorf("update flow: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет flow (каскадно удалит versions, runs, schedules).
func (r *FlowRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM flows WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete flow: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- FlowVersion CRUD ---

// CreateVersion создаёт новую версию flow.
// Версия автоматически инкрементируется.
func (r *FlowRepo) CreateVersion(ctx context.Context, flowID uuid.UUID, spec domain.FlowSpec) (*domain.FlowVersion, error) {
	// Сериализуем spec в JSON
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}

	// Получаем следующий номер версии
	var nextVersion int
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1
		FROM flow_versions
		WHERE flow_id = $1
	`, flowID).Scan(&nextVersion)
	if err != nil {
		return nil, fmt.Errorf("get next version: %w", err)
	}

	// Создаём версию
	var version domain.FlowVersion
	err = r.pool.QueryRow(ctx, `
		INSERT INTO flow_versions (flow_id, version, spec, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING flow_id, version, spec, created_at
	`, flowID, nextVersion, specJSON).Scan(
		&version.FlowID,
		&version.Version,
		&specJSON,
		&version.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert flow version: %w", err)
	}

	// Десериализуем spec обратно
	if err := json.Unmarshal(specJSON, &version.Spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}

	return &version, nil
}

// GetVersion возвращает конкретную версию flow.
func (r *FlowRepo) GetVersion(ctx context.Context, flowID uuid.UUID, version int) (*domain.FlowVersion, error) {
	query := `
		SELECT flow_id, version, spec, created_at
		FROM flow_versions
		WHERE flow_id = $1 AND version = $2
	`
	var fv domain.FlowVersion
	var specJSON []byte
	err := r.pool.QueryRow(ctx, query, flowID, version).Scan(
		&fv.FlowID,
		&fv.Version,
		&specJSON,
		&fv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get flow version: %w", err)
	}

	if err := json.Unmarshal(specJSON, &fv.Spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}

	return &fv, nil
}

// GetLatestVersion возвращает последнюю версию flow.
func (r *FlowRepo) GetLatestVersion(ctx context.Context, flowID uuid.UUID) (*domain.FlowVersion, error) {
	query := `
		SELECT flow_id, version, spec, created_at
		FROM flow_versions
		WHERE flow_id = $1
		ORDER BY version DESC
		LIMIT 1
	`
	var fv domain.FlowVersion
	var specJSON []byte
	err := r.pool.QueryRow(ctx, query, flowID).Scan(
		&fv.FlowID,
		&fv.Version,
		&specJSON,
		&fv.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest flow version: %w", err)
	}

	if err := json.Unmarshal(specJSON, &fv.Spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}

	return &fv, nil
}

// ListVersions возвращает все версии flow.
func (r *FlowRepo) ListVersions(ctx context.Context, flowID uuid.UUID) ([]domain.FlowVersion, error) {
	query := `
		SELECT flow_id, version, spec, created_at
		FROM flow_versions
		WHERE flow_id = $1
		ORDER BY version DESC
	`
	rows, err := r.pool.Query(ctx, query, flowID)
	if err != nil {
		return nil, fmt.Errorf("list flow versions: %w", err)
	}
	defer rows.Close()

	var versions []domain.FlowVersion
	for rows.Next() {
		var fv domain.FlowVersion
		var specJSON []byte
		if err := rows.Scan(
			&fv.FlowID,
			&fv.Version,
			&specJSON,
			&fv.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan flow version: %w", err)
		}

		if err := json.Unmarshal(specJSON, &fv.Spec); err != nil {
			return nil, fmt.Errorf("unmarshal spec: %w", err)
		}

		versions = append(versions, fv)
	}
	return versions, rows.Err()
}
