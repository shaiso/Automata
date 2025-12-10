package domain

// RunStatus — статус выполнения run.
//
// Жизненный цикл:
//
//	PENDING → RUNNING → SUCCEEDED
//	                  ↘ FAILED
//	          (или) → CANCELLED (из PENDING или RUNNING)
type RunStatus string

const (
	// RunStatusPending — run создан, но ещё не начал выполняться.
	RunStatusPending RunStatus = "PENDING"

	// RunStatusRunning — run в процессе выполнения.
	RunStatusRunning RunStatus = "RUNNING"

	// RunStatusSucceeded — run успешно завершён.
	RunStatusSucceeded RunStatus = "SUCCEEDED"

	// RunStatusFailed — run завершился с ошибкой.
	RunStatusFailed RunStatus = "FAILED"

	// RunStatusCancelled — run отменён пользователем.
	RunStatusCancelled RunStatus = "CANCELLED"
)

// IsTerminal возвращает true, если статус финальный (run завершён).
func (s RunStatus) IsTerminal() bool {
	switch s {
	case RunStatusSucceeded, RunStatusFailed, RunStatusCancelled:
		return true
	default:
		return false
	}
}

// TaskStatus — статус выполнения task.
//
// Жизненный цикл:
//
//	QUEUED → RUNNING → SUCCEEDED
//	                 ↘ FAILED (может быть retry → обратно в QUEUED)
type TaskStatus string

const (
	// TaskStatusQueued — task в очереди, ожидает выполнения.
	TaskStatusQueued TaskStatus = "QUEUED"

	// TaskStatusRunning — task выполняется воркером.
	TaskStatusRunning TaskStatus = "RUNNING"

	// TaskStatusSucceeded — task успешно завершён.
	TaskStatusSucceeded TaskStatus = "SUCCEEDED"

	// TaskStatusFailed — task завершился с ошибкой (после всех retry).
	TaskStatusFailed TaskStatus = "FAILED"
)

// IsTerminal возвращает true, если статус финальный.
func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusSucceeded, TaskStatusFailed:
		return true
	default:
		return false
	}
}

// ProposalStatus — статус предложения изменений (PR-workflow).
//
// Жизненный цикл:
//
//	DRAFT → PENDING_REVIEW → APPROVED → APPLIED
//	                       ↘ REJECTED
type ProposalStatus string

const (
	// ProposalStatusDraft — черновик, можно редактировать.
	ProposalStatusDraft ProposalStatus = "DRAFT"

	// ProposalStatusPendingReview — отправлен на проверку.
	ProposalStatusPendingReview ProposalStatus = "PENDING_REVIEW"

	// ProposalStatusApproved — одобрен, можно применить.
	ProposalStatusApproved ProposalStatus = "APPROVED"

	// ProposalStatusRejected — отклонён.
	ProposalStatusRejected ProposalStatus = "REJECTED"

	// ProposalStatusApplied — применён (создана новая версия flow).
	ProposalStatusApplied ProposalStatus = "APPLIED"
)

// String возвращает строковое представление ProposalStatus.
func (s ProposalStatus) String() string {
	return string(s)
}

// ParseProposalStatus парсит строку в ProposalStatus.
func ParseProposalStatus(s string) ProposalStatus {
	switch s {
	case "DRAFT":
		return ProposalStatusDraft
	case "PENDING_REVIEW":
		return ProposalStatusPendingReview
	case "APPROVED":
		return ProposalStatusApproved
	case "REJECTED":
		return ProposalStatusRejected
	case "APPLIED":
		return ProposalStatusApplied
	default:
		return ProposalStatusDraft
	}
}
