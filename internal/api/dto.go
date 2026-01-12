package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
)

// Flow DTOs

// CreateFlowRequest — запрос на создание flow.
type CreateFlowRequest struct {
	Name string `json:"name"`
}

// UpdateFlowRequest — запрос на обновление flow.
type UpdateFlowRequest struct {
	Name     *string `json:"name,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// FlowResponse — ответ с flow.
type FlowResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// FlowFromDomain конвертирует domain.Flow в FlowResponse.
func FlowFromDomain(f domain.Flow) FlowResponse {
	return FlowResponse{
		ID:        f.ID,
		Name:      f.Name,
		IsActive:  f.IsActive,
		CreatedAt: f.CreatedAt,
	}
}

// FlowVersion DTOs

// CreateFlowVersionRequest — запрос на создание версии flow.
type CreateFlowVersionRequest struct {
	Spec domain.FlowSpec `json:"spec"`
}

// FlowVersionResponse — ответ с версией flow.
type FlowVersionResponse struct {
	FlowID    uuid.UUID       `json:"flow_id"`
	Version   int             `json:"version"`
	Spec      domain.FlowSpec `json:"spec"`
	CreatedAt time.Time       `json:"created_at"`
}

// FlowVersionFromDomain конвертирует domain.FlowVersion в FlowVersionResponse.
func FlowVersionFromDomain(v domain.FlowVersion) FlowVersionResponse {
	return FlowVersionResponse{
		FlowID:    v.FlowID,
		Version:   v.Version,
		Spec:      v.Spec,
		CreatedAt: v.CreatedAt,
	}
}

// Run DTOs

// CreateRunRequest — запрос на создание run.
type CreateRunRequest struct {
	Inputs         map[string]any `json:"inputs,omitempty"`
	Version        *int           `json:"version,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	IsSandbox      bool           `json:"is_sandbox,omitempty"`
}

// RunResponse — ответ с run.
type RunResponse struct {
	ID             uuid.UUID      `json:"id"`
	FlowID         uuid.UUID      `json:"flow_id"`
	Version        int            `json:"version"`
	Status         string         `json:"status"`
	Inputs         map[string]any `json:"inputs,omitempty"`
	StartedAt      *time.Time     `json:"started_at,omitempty"`
	FinishedAt     *time.Time     `json:"finished_at,omitempty"`
	Error          string         `json:"error,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	IsSandbox      bool           `json:"is_sandbox"`
	CreatedAt      time.Time      `json:"created_at"`
}

// RunFromDomain конвертирует domain.Run в RunResponse.
func RunFromDomain(r domain.Run) RunResponse {
	return RunResponse{
		ID:             r.ID,
		FlowID:         r.FlowID,
		Version:        r.Version,
		Status:         string(r.Status),
		Inputs:         r.Inputs,
		StartedAt:      r.StartedAt,
		FinishedAt:     r.FinishedAt,
		Error:          r.Error,
		IdempotencyKey: r.IdempotencyKey,
		IsSandbox:      r.IsSandbox,
		CreatedAt:      r.CreatedAt,
	}
}

// Task DTOs

// TaskResponse — ответ с task.
type TaskResponse struct {
	ID         uuid.UUID      `json:"id"`
	RunID      uuid.UUID      `json:"run_id"`
	StepID     string         `json:"step_id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Attempt    int            `json:"attempt"`
	Status     string         `json:"status"`
	Payload    map[string]any `json:"payload,omitempty"`
	Outputs    map[string]any `json:"outputs,omitempty"`
	StartedAt  *time.Time     `json:"started_at,omitempty"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Error      string         `json:"error,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

// TaskFromDomain конвертирует domain.Task в TaskResponse.
func TaskFromDomain(t domain.Task) TaskResponse {
	return TaskResponse{
		ID:         t.ID,
		RunID:      t.RunID,
		StepID:     t.StepID,
		Name:       t.Name,
		Type:       t.Type,
		Attempt:    t.Attempt,
		Status:     string(t.Status),
		Payload:    t.Payload,
		Outputs:    t.Outputs,
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
		Error:      t.Error,
		CreatedAt:  t.CreatedAt,
	}
}

// Schedule DTOs

// CreateScheduleRequest — запрос на создание schedule.
type CreateScheduleRequest struct {
	Name        string         `json:"name"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	IntervalSec int            `json:"interval_sec,omitempty"`
	Timezone    string         `json:"timezone,omitempty"`
	Enabled     bool           `json:"enabled"`
	Inputs      map[string]any `json:"inputs,omitempty"`
}

// UpdateScheduleRequest — запрос на обновление schedule.
type UpdateScheduleRequest struct {
	Name        *string         `json:"name,omitempty"`
	CronExpr    *string         `json:"cron_expr,omitempty"`
	IntervalSec *int            `json:"interval_sec,omitempty"`
	Timezone    *string         `json:"timezone,omitempty"`
	Inputs      *map[string]any `json:"inputs,omitempty"`
}

// SetEnabledRequest — запрос на включение/выключение.
type SetEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// ScheduleResponse — ответ с schedule.
type ScheduleResponse struct {
	ID          uuid.UUID      `json:"id"`
	FlowID      uuid.UUID      `json:"flow_id"`
	Name        string         `json:"name"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	IntervalSec int            `json:"interval_sec,omitempty"`
	Timezone    string         `json:"timezone"`
	Enabled     bool           `json:"enabled"`
	NextDueAt   *time.Time     `json:"next_due_at,omitempty"`
	LastRunAt   *time.Time     `json:"last_run_at,omitempty"`
	LastRunID   *uuid.UUID     `json:"last_run_id,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ScheduleFromDomain конвертирует domain.Schedule в ScheduleResponse.
func ScheduleFromDomain(s *domain.Schedule) ScheduleResponse {
	if s == nil {
		return ScheduleResponse{}
	}
	return ScheduleResponse{
		ID:          s.ID,
		FlowID:      s.FlowID,
		Name:        s.Name,
		CronExpr:    s.CronExpr,
		IntervalSec: s.IntervalSec,
		Timezone:    s.Timezone,
		Enabled:     s.Enabled,
		NextDueAt:   s.NextDueAt,
		LastRunAt:   s.LastRunAt,
		LastRunID:   s.LastRunID,
		Inputs:      s.Inputs,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}
