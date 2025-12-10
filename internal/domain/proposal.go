package domain

import (
	"time"

	"github.com/google/uuid"
)

// Proposal — предложение изменений в flow (PR-workflow).
//
// Proposal позволяет:
// - Создать черновик изменений
// - Протестировать в sandbox перед применением
// - Получить одобрение (review)
// - Применить изменения (создать новую версию flow)
//
// Жизненный цикл:
//
//	DRAFT → PENDING_REVIEW → APPROVED → APPLIED
//	                       ↘ REJECTED
type Proposal struct {
	// ID — уникальный идентификатор proposal.
	ID uuid.UUID `json:"id"`

	// FlowID — ссылка на flow, который изменяем.
	FlowID uuid.UUID `json:"flow_id"`

	// BaseVersion — версия flow, от которой отталкиваемся.
	// Nil, если это первая версия (flow ещё без версий).
	BaseVersion *int `json:"base_version,omitempty"`

	// ProposedSpec — предлагаемая спецификация.
	ProposedSpec FlowSpec `json:"proposed_spec"`

	// Status — текущий статус proposal.
	Status ProposalStatus `json:"status"`

	// Title — заголовок изменений.
	Title string `json:"title,omitempty"`

	// Description — описание изменений.
	Description string `json:"description,omitempty"`

	// CreatedBy — кто создал proposal.
	CreatedBy string `json:"created_by,omitempty"`

	// CreatedAt — время создания.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt — время последнего обновления.
	UpdatedAt time.Time `json:"updated_at"`

	// --- Review ---

	// ReviewedBy — кто провёл review.
	ReviewedBy string `json:"reviewed_by,omitempty"`

	// ReviewedAt — время review.
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`

	// ReviewComment — комментарий к review.
	ReviewComment string `json:"review_comment,omitempty"`

	// --- Sandbox ---

	// SandboxRunID — ID run'а в sandbox режиме.
	SandboxRunID *uuid.UUID `json:"sandbox_run_id,omitempty"`

	// SandboxResult — результат тестового запуска.
	SandboxResult *SandboxResult `json:"sandbox_result,omitempty"`

	// --- Applied ---

	// AppliedVersion — версия flow, которая была создана при применении.
	AppliedVersion *int `json:"applied_version,omitempty"`

	// AppliedAt — время применения.
	AppliedAt *time.Time `json:"applied_at,omitempty"`
}

// SandboxResult — результат тестового выполнения в sandbox.
type SandboxResult struct {
	// RunID — ID sandbox run'а.
	RunID uuid.UUID `json:"run_id"`

	// Status — итоговый статус.
	Status RunStatus `json:"status"`

	// StartedAt — время начала.
	StartedAt time.Time `json:"started_at"`

	// FinishedAt — время завершения.
	FinishedAt time.Time `json:"finished_at"`

	// Steps — результаты по каждому шагу.
	Steps []StepResult `json:"steps"`

	// Summary — сводка по результатам.
	Summary SandboxSummary `json:"summary"`
}

// StepResult — результат выполнения шага в sandbox.
type StepResult struct {
	// StepID — ID шага.
	StepID string `json:"step_id"`

	// Status — статус выполнения.
	Status TaskStatus `json:"status"`

	// Duration — продолжительность выполнения.
	DurationMs int64 `json:"duration_ms"`

	// ExpectedOutput — ожидаемый результат (из записанного ранее).
	ExpectedOutput map[string]any `json:"expected_output,omitempty"`

	// ActualOutput — фактический результат.
	ActualOutput map[string]any `json:"actual_output,omitempty"`

	// Diff — различия между ожидаемым и фактическим.
	Diff []OutputDiff `json:"diff,omitempty"`

	// Error — ошибка, если шаг упал.
	Error string `json:"error,omitempty"`
}

// OutputDiff — различие в результатах.
type OutputDiff struct {
	// Path — путь к полю (например, "data.items[0].price").
	Path string `json:"path"`

	// Expected — ожидаемое значение.
	Expected any `json:"expected"`

	// Actual — фактическое значение.
	Actual any `json:"actual"`
}

// SandboxSummary — сводка по sandbox запуску.
type SandboxSummary struct {
	// TotalSteps — общее количество шагов.
	TotalSteps int `json:"total_steps"`

	// Succeeded — количество успешных.
	Succeeded int `json:"succeeded"`

	// Failed — количество упавших.
	Failed int `json:"failed"`

	// WithDiffs — количество с различиями в результатах.
	WithDiffs int `json:"with_diffs"`
}

// CanEdit возвращает true, если proposal можно редактировать.
func (p *Proposal) CanEdit() bool {
	return p.Status == ProposalStatusDraft
}

// CanSubmit возвращает true, если proposal можно отправить на review.
func (p *Proposal) CanSubmit() bool {
	return p.Status == ProposalStatusDraft
}

// CanApply возвращает true, если proposal можно применить.
func (p *Proposal) CanApply() bool {
	return p.Status == ProposalStatusApproved
}

// Submit отправляет proposal на review.
func (p *Proposal) Submit() {
	p.Status = ProposalStatusPendingReview
	p.UpdatedAt = time.Now()
}

// Approve одобряет proposal.
func (p *Proposal) Approve(reviewer, comment string) {
	now := time.Now()
	p.Status = ProposalStatusApproved
	p.ReviewedBy = reviewer
	p.ReviewedAt = &now
	p.ReviewComment = comment
	p.UpdatedAt = now
}

// Reject отклоняет proposal.
func (p *Proposal) Reject(reviewer, comment string) {
	now := time.Now()
	p.Status = ProposalStatusRejected
	p.ReviewedBy = reviewer
	p.ReviewedAt = &now
	p.ReviewComment = comment
	p.UpdatedAt = now
}

// MarkApplied отмечает proposal как применённый.
func (p *Proposal) MarkApplied(version int) {
	now := time.Now()
	p.Status = ProposalStatusApplied
	p.AppliedVersion = &version
	p.AppliedAt = &now
	p.UpdatedAt = now
}
