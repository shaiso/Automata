package domain

import (
	"time"

	"github.com/google/uuid"
)

// Run — экземпляр выполнения flow.
//
// Run создаётся когда:
// - Пользователь запускает flow вручную (через API/CLI)
// - Scheduler создаёт run по расписанию
// - Другой flow запускает этот как под-процесс (future)
//
// Каждый run выполняет конкретную версию flow и имеет свой набор tasks.
type Run struct {
	// ID — уникальный идентификатор run.
	ID uuid.UUID `json:"id"`

	// FlowID — ссылка на flow, который выполняется.
	FlowID uuid.UUID `json:"flow_id"`

	// Version — версия flow, которая выполняется.
	Version int `json:"version"`

	// Status — текущий статус выполнения.
	Status RunStatus `json:"status"`

	// Inputs — входные параметры, переданные при запуске.
	// Соответствуют FlowSpec.Inputs.
	Inputs map[string]any `json:"inputs,omitempty"`

	// StartedAt — время начала выполнения (когда статус стал RUNNING).
	// Nil, если run ещё не начался.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// FinishedAt — время завершения (успешного или с ошибкой).
	// Nil, если run ещё выполняется.
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Error — текст ошибки, если run завершился с FAILED.
	Error string `json:"error,omitempty"`

	// IdempotencyKey — ключ идемпотентности для предотвращения дубликатов.
	// Например, для scheduled runs: "{schedule_id}_{next_due_at}"
	IdempotencyKey string `json:"idempotency_key,omitempty"`

	// IsSandbox — флаг, указывающий что это тестовый запуск (sandbox mode).
	IsSandbox bool `json:"is_sandbox,omitempty"`

	// CreatedAt — время создания run.
	CreatedAt time.Time `json:"created_at"`
}

// Duration возвращает продолжительность выполнения.
// Возвращает 0, если run ещё не завершён.
func (r *Run) Duration() time.Duration {
	if r.StartedAt == nil || r.FinishedAt == nil {
		return 0
	}
	return r.FinishedAt.Sub(*r.StartedAt)
}

// IsFinished возвращает true, если run завершён (в любом статусе).
func (r *Run) IsFinished() bool {
	return r.Status.IsTerminal()
}

// MarkRunning переводит run в статус RUNNING.
func (r *Run) MarkRunning() {
	now := time.Now()
	r.Status = RunStatusRunning
	r.StartedAt = &now
}

// MarkSucceeded переводит run в статус SUCCEEDED.
func (r *Run) MarkSucceeded() {
	now := time.Now()
	r.Status = RunStatusSucceeded
	r.FinishedAt = &now
}

// MarkFailed переводит run в статус FAILED с ошибкой.
func (r *Run) MarkFailed(err string) {
	now := time.Now()
	r.Status = RunStatusFailed
	r.FinishedAt = &now
	r.Error = err
}

// MarkCancelled переводит run в статус CANCELLED.
func (r *Run) MarkCancelled() {
	now := time.Now()
	r.Status = RunStatusCancelled
	r.FinishedAt = &now
}
