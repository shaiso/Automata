package domain

import (
	"time"

	"github.com/google/uuid"
)

// Task — отдельная единица работы внутри run.
//
// Task создаётся Orchestrator'ом когда:
// - Run стартует (для шагов без зависимостей)
// - Зависимости шага удовлетворены (предыдущие tasks завершились)
//
// Task выполняется Worker'ом.
type Task struct {
	// ID — уникальный идентификатор task.
	ID uuid.UUID `json:"id"`

	// RunID — ссылка на родительский run.
	RunID uuid.UUID `json:"run_id"`

	// StepID — ID шага из FlowSpec (соответствует StepDef.ID).
	StepID string `json:"step_id"`

	// Name — имя шага (для удобства, копия StepDef.Name).
	Name string `json:"name"`

	// Type — тип шага: "http", "delay", "transform", "parallel".
	Type string `json:"type"`

	// Attempt — номер попытки (начиная с 1).
	// Увеличивается при retry.
	Attempt int `json:"attempt"`

	// Status — текущий статус task.
	Status TaskStatus `json:"status"`

	// Payload — входные данные для task (конфигурация шага + данные из предыдущих шагов).
	// Это то, что Worker получает для выполнения.
	Payload map[string]any `json:"payload,omitempty"`

	// Outputs — результаты выполнения task.
	// Заполняется Worker'ом после успешного выполнения.
	// Используется следующими шагами через {{ .steps.X.outputs.Y }}
	Outputs map[string]any `json:"outputs,omitempty"`

	// ResultRef — ссылка на результат (если слишком большой для хранения inline).
	// Например: "s3://bucket/runs/abc/tasks/xyz/result.json"
	ResultRef string `json:"result_ref,omitempty"`

	// StartedAt — время начала выполнения.
	StartedAt *time.Time `json:"started_at,omitempty"`

	// FinishedAt — время завершения.
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	// Error — текст ошибки при неудаче.
	Error string `json:"error,omitempty"`

	// CreatedAt — время создания task.
	CreatedAt time.Time `json:"created_at"`
}

// Duration возвращает продолжительность выполнения.
func (t *Task) Duration() time.Duration {
	if t.StartedAt == nil || t.FinishedAt == nil {
		return 0
	}
	return t.FinishedAt.Sub(*t.StartedAt)
}

// IsFinished возвращает true, если task завершён.
func (t *Task) IsFinished() bool {
	return t.Status.IsTerminal()
}

// MarkRunning переводит task в статус RUNNING.
func (t *Task) MarkRunning() {
	now := time.Now()
	t.Status = TaskStatusRunning
	t.StartedAt = &now
	t.Attempt++
}

// MarkSucceeded переводит task в статус SUCCEEDED с результатами.
func (t *Task) MarkSucceeded(outputs map[string]any) {
	now := time.Now()
	t.Status = TaskStatusSucceeded
	t.FinishedAt = &now
	t.Outputs = outputs
}

// MarkFailed переводит task в статус FAILED с ошибкой.
func (t *Task) MarkFailed(err string) {
	now := time.Now()
	t.Status = TaskStatusFailed
	t.FinishedAt = &now
	t.Error = err
}

// ResetForRetry подготавливает task для повторной попытки.
// Сбрасывает статус в QUEUED, очищает ошибку.
func (t *Task) ResetForRetry() {
	t.Status = TaskStatusQueued
	t.StartedAt = nil
	t.FinishedAt = nil
	t.Error = ""
	// Attempt увеличится при следующем MarkRunning()
}

// CanRetry проверяет, можно ли сделать ещё одну попытку.
func (t *Task) CanRetry(maxAttempts int) bool {
	return t.Attempt < maxAttempts
}
