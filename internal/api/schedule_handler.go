package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/repo"
)

// ListSchedules возвращает список schedules с фильтрацией.
// GET /api/v1/schedules?flow_id=...&enabled=...&limit=...&offset=...
func (h *Handler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	filter := repo.ScheduleFilter{}

	// Парсим query параметры
	if flowIDStr := r.URL.Query().Get("flow_id"); flowIDStr != "" {
		flowID, err := uuid.Parse(flowIDStr)
		if err != nil {
			BadRequest(w, "invalid flow_id")
			return
		}
		filter.FlowID = &flowID
	}

	if enabledStr := r.URL.Query().Get("enabled"); enabledStr != "" {
		enabled := enabledStr == "true"
		filter.Enabled = &enabled
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		filter.Limit = int(mustParseInt(limitStr, 50))
	} else {
		filter.Limit = 50
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset = int(mustParseInt(offsetStr, 0))
	}

	schedules, err := h.scheduleRepo.List(r.Context(), filter)
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]ScheduleResponse, len(schedules))
	for i := range schedules {
		result[i] = ScheduleFromDomain(&schedules[i])
	}

	List(w, result, len(result))
}

// CreateSchedule создаёт новый schedule для flow.
// POST /api/v1/flows/{id}/schedules
func (h *Handler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	flowID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	// Валидация
	if req.Name == "" {
		BadRequest(w, "name is required")
		return
	}

	if req.CronExpr == "" && req.IntervalSec <= 0 {
		BadRequest(w, "either cron_expr or interval_sec is required")
		return
	}

	// Проверяем, что flow существует
	_, err = h.flowRepo.GetByID(r.Context(), flowID)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	timezone := req.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	schedule := &domain.Schedule{
		ID:          uuid.New(),
		FlowID:      flowID,
		Name:        req.Name,
		CronExpr:    req.CronExpr,
		IntervalSec: req.IntervalSec,
		Timezone:    timezone,
		Enabled:     req.Enabled,
		Inputs:      req.Inputs,
	}

	if err := h.scheduleRepo.Create(r.Context(), schedule); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Created(w, ScheduleFromDomain(schedule))
}

// GetSchedule возвращает schedule по ID.
// GET /api/v1/schedules/{id}
func (h *Handler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid schedule id")
		return
	}

	schedule, err := h.scheduleRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "schedule not found") {
		return
	}

	Success(w, ScheduleFromDomain(schedule))
}

// UpdateSchedule обновляет schedule.
// PUT /api/v1/schedules/{id}
func (h *Handler) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid schedule id")
		return
	}

	var req UpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	schedule, err := h.scheduleRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "schedule not found") {
		return
	}

	if req.Name != nil {
		schedule.Name = *req.Name
	}
	if req.CronExpr != nil {
		schedule.CronExpr = *req.CronExpr
	}
	if req.IntervalSec != nil {
		schedule.IntervalSec = *req.IntervalSec
	}
	if req.Timezone != nil {
		schedule.Timezone = *req.Timezone
	}
	if req.Inputs != nil {
		schedule.Inputs = *req.Inputs
	}

	if err := h.scheduleRepo.Update(r.Context(), schedule); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Success(w, ScheduleFromDomain(schedule))
}

// DeleteSchedule удаляет schedule.
// DELETE /api/v1/schedules/{id}
func (h *Handler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid schedule id")
		return
	}

	if err := h.scheduleRepo.Delete(r.Context(), id); err != nil {
		if HandleRepoError(w, h.logger, err, "schedule not found") {
			return
		}
		InternalError(w, h.logger, err)
		return
	}

	NoContent(w)
}

// SetScheduleEnabled включает или выключает schedule.
// PUT /api/v1/schedules/{id}/enabled
func (h *Handler) SetScheduleEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid schedule id")
		return
	}

	var req SetEnabledRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	if err := h.scheduleRepo.SetEnabled(r.Context(), id, req.Enabled); err != nil {
		if HandleRepoError(w, h.logger, err, "schedule not found") {
			return
		}
		InternalError(w, h.logger, err)
		return
	}

	// Возвращаем обновлённый schedule
	schedule, err := h.scheduleRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "schedule not found") {
		return
	}

	Success(w, ScheduleFromDomain(schedule))
}
