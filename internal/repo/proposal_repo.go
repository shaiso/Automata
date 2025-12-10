package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shaiso/Automata/internal/domain"
)

// ProposalRepo — репозиторий для работы с proposals (PR-workflow).
type ProposalRepo struct {
	pool *pgxpool.Pool
}

// NewProposalRepo создаёт новый ProposalRepo.
func NewProposalRepo(pool *pgxpool.Pool) *ProposalRepo {
	return &ProposalRepo{pool: pool}
}

// ProposalFilter — фильтр для списка proposals.
type ProposalFilter struct {
	FlowID *uuid.UUID
	Status *domain.ProposalStatus
	Limit  int
	Offset int
}

// --- Базовые CRUD ---

// Create создаёт новый proposal.
func (r *ProposalRepo) Create(ctx context.Context, p *domain.Proposal) error {
	specJSON, err := json.Marshal(p.ProposedSpec)
	if err != nil {
		return fmt.Errorf("marshal proposed_spec: %w", err)
	}

	query := `
		INSERT INTO proposals (
			id, flow_id, base_version, proposed_spec, status,
			title, description, created_by, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.pool.Exec(ctx, query,
		p.ID,
		p.FlowID,
		p.BaseVersion,
		specJSON,
		p.Status.String(),
		p.Title,
		p.Description,
		p.CreatedBy,
		p.CreatedAt,
		p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert proposal: %w", err)
	}
	return nil
}

// GetByID возвращает proposal по ID.
func (r *ProposalRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Proposal, error) {
	query := `
		SELECT
			id, flow_id, base_version, proposed_spec, status,
			title, description, created_by, created_at, updated_at,
			reviewed_by, reviewed_at, review_comment,
			sandbox_run_id, sandbox_result,
			applied_version, applied_at
		FROM proposals
		WHERE id = $1
	`
	return r.scanProposal(r.pool.QueryRow(ctx, query, id))
}

// List возвращает список proposals с фильтрацией.
func (r *ProposalRepo) List(ctx context.Context, filter ProposalFilter) ([]domain.Proposal, error) {
	var conditions []string
	var args []any
	argNum := 1

	if filter.FlowID != nil {
		conditions = append(conditions, fmt.Sprintf("flow_id = $%d", argNum))
		args = append(args, *filter.FlowID)
		argNum++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argNum))
		args = append(args, filter.Status.String())
		argNum++
	}

	query := `
		SELECT
			id, flow_id, base_version, proposed_spec, status,
			title, description, created_by, created_at, updated_at,
			reviewed_by, reviewed_at, review_comment,
			sandbox_run_id, sandbox_result,
			applied_version, applied_at
		FROM proposals
	`

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list proposals: %w", err)
	}
	defer rows.Close()

	return r.scanProposals(rows)
}

// ListByFlowID возвращает proposals для конкретного flow.
func (r *ProposalRepo) ListByFlowID(ctx context.Context, flowID uuid.UUID) ([]domain.Proposal, error) {
	return r.List(ctx, ProposalFilter{FlowID: &flowID})
}

// ListByStatus возвращает proposals в определённом статусе.
func (r *ProposalRepo) ListByStatus(ctx context.Context, status domain.ProposalStatus) ([]domain.Proposal, error) {
	return r.List(ctx, ProposalFilter{Status: &status})
}

// Update обновляет proposal (основные поля).
func (r *ProposalRepo) Update(ctx context.Context, p *domain.Proposal) error {
	specJSON, err := json.Marshal(p.ProposedSpec)
	if err != nil {
		return fmt.Errorf("marshal proposed_spec: %w", err)
	}

	var sandboxResultJSON []byte
	if p.SandboxResult != nil {
		sandboxResultJSON, err = json.Marshal(p.SandboxResult)
		if err != nil {
			return fmt.Errorf("marshal sandbox_result: %w", err)
		}
	}

	query := `
		UPDATE proposals
		SET
			base_version = $2,
			proposed_spec = $3,
			status = $4,
			title = $5,
			description = $6,
			updated_at = $7,
			reviewed_by = $8,
			reviewed_at = $9,
			review_comment = $10,
			sandbox_run_id = $11,
			sandbox_result = $12,
			applied_version = $13,
			applied_at = $14
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query,
		p.ID,
		p.BaseVersion,
		specJSON,
		p.Status.String(),
		p.Title,
		p.Description,
		p.UpdatedAt,
		p.ReviewedBy,
		p.ReviewedAt,
		p.ReviewComment,
		p.SandboxRunID,
		sandboxResultJSON,
		p.AppliedVersion,
		p.AppliedAt,
	)
	if err != nil {
		return fmt.Errorf("update proposal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete удаляет proposal.
func (r *ProposalRepo) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM proposals WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete proposal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- PR-workflow операции ---

// Submit отправляет proposal на review (DRAFT → PENDING_REVIEW).
func (r *ProposalRepo) Submit(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE proposals
		SET status = 'PENDING_REVIEW', updated_at = NOW()
		WHERE id = $1 AND status = 'DRAFT'
	`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("submit proposal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

// Approve одобряет proposal (PENDING_REVIEW → APPROVED).
func (r *ProposalRepo) Approve(ctx context.Context, id uuid.UUID, reviewer, comment string) error {
	query := `
		UPDATE proposals
		SET status = 'APPROVED',
			reviewed_by = $2,
			reviewed_at = NOW(),
			review_comment = $3,
			updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING_REVIEW'
	`
	result, err := r.pool.Exec(ctx, query, id, reviewer, comment)
	if err != nil {
		return fmt.Errorf("approve proposal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

// Reject отклоняет proposal (PENDING_REVIEW → REJECTED).
func (r *ProposalRepo) Reject(ctx context.Context, id uuid.UUID, reviewer, comment string) error {
	query := `
		UPDATE proposals
		SET status = 'REJECTED',
			reviewed_by = $2,
			reviewed_at = NOW(),
			review_comment = $3,
			updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING_REVIEW'
	`
	result, err := r.pool.Exec(ctx, query, id, reviewer, comment)
	if err != nil {
		return fmt.Errorf("reject proposal: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

// SetSandboxResult сохраняет результат тестового запуска.
func (r *ProposalRepo) SetSandboxResult(ctx context.Context, id uuid.UUID, sandboxRunID uuid.UUID, result *domain.SandboxResult) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal sandbox_result: %w", err)
	}

	query := `
		UPDATE proposals
		SET sandbox_run_id = $2,
			sandbox_result = $3,
			updated_at = NOW()
		WHERE id = $1
	`
	res, err := r.pool.Exec(ctx, query, id, sandboxRunID, resultJSON)
	if err != nil {
		return fmt.Errorf("set sandbox result: %w", err)
	}
	if res.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkApplied отмечает proposal как применённый (APPROVED → APPLIED).
func (r *ProposalRepo) MarkApplied(ctx context.Context, id uuid.UUID, version int) error {
	query := `
		UPDATE proposals
		SET status = 'APPLIED',
			applied_version = $2,
			applied_at = NOW(),
			updated_at = NOW()
		WHERE id = $1 AND status = 'APPROVED'
	`
	result, err := r.pool.Exec(ctx, query, id, version)
	if err != nil {
		return fmt.Errorf("mark applied: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return nil
}

// --- Вспомогательные методы ---

// scanProposal сканирует одну строку в Proposal.
func (r *ProposalRepo) scanProposal(row pgx.Row) (*domain.Proposal, error) {
	var p domain.Proposal
	var specJSON []byte
	var sandboxResultJSON []byte
	var statusStr string

	err := row.Scan(
		&p.ID,
		&p.FlowID,
		&p.BaseVersion,
		&specJSON,
		&statusStr,
		&p.Title,
		&p.Description,
		&p.CreatedBy,
		&p.CreatedAt,
		&p.UpdatedAt,
		&p.ReviewedBy,
		&p.ReviewedAt,
		&p.ReviewComment,
		&p.SandboxRunID,
		&sandboxResultJSON,
		&p.AppliedVersion,
		&p.AppliedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan proposal: %w", err)
	}

	// Парсим status
	p.Status = domain.ParseProposalStatus(statusStr)

	// Парсим proposed_spec
	if err := json.Unmarshal(specJSON, &p.ProposedSpec); err != nil {
		return nil, fmt.Errorf("unmarshal proposed_spec: %w", err)
	}

	// Парсим sandbox_result (может быть null)
	if len(sandboxResultJSON) > 0 {
		var sr domain.SandboxResult
		if err := json.Unmarshal(sandboxResultJSON, &sr); err != nil {
			return nil, fmt.Errorf("unmarshal sandbox_result: %w", err)
		}
		p.SandboxResult = &sr
	}

	return &p, nil
}

// scanProposals сканирует несколько строк в []Proposal.
func (r *ProposalRepo) scanProposals(rows pgx.Rows) ([]domain.Proposal, error) {
	var proposals []domain.Proposal

	for rows.Next() {
		var p domain.Proposal
		var specJSON []byte
		var sandboxResultJSON []byte
		var statusStr string

		err := rows.Scan(
			&p.ID,
			&p.FlowID,
			&p.BaseVersion,
			&specJSON,
			&statusStr,
			&p.Title,
			&p.Description,
			&p.CreatedBy,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.ReviewedBy,
			&p.ReviewedAt,
			&p.ReviewComment,
			&p.SandboxRunID,
			&sandboxResultJSON,
			&p.AppliedVersion,
			&p.AppliedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan proposal: %w", err)
		}

		// Парсим status
		p.Status = domain.ParseProposalStatus(statusStr)

		// Парсим proposed_spec
		if err := json.Unmarshal(specJSON, &p.ProposedSpec); err != nil {
			return nil, fmt.Errorf("unmarshal proposed_spec: %w", err)
		}

		// Парсим sandbox_result (может быть null)
		if len(sandboxResultJSON) > 0 {
			var sr domain.SandboxResult
			if err := json.Unmarshal(sandboxResultJSON, &sr); err != nil {
				return nil, fmt.Errorf("unmarshal sandbox_result: %w", err)
			}
			p.SandboxResult = &sr
		}

		proposals = append(proposals, p)
	}

	return proposals, rows.Err()
}
