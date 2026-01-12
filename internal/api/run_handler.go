package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/repo"
)

// ListRuns возвращает список runs с фильтрацией.
// GET /api/v1/runs?flow_id=...&status=...&limit=...&offset=...
func (h *Handler) ListRuns(w http.ResponseWriter, r *http.Request) {
	filter := repo.RunFilter{}

	// Парсим query параметры
	if flowIDStr := r.URL.Query().Get("flow_id"); flowIDStr != "" {
		flowID, err := uuid.Parse(flowIDStr)
		if err != nil {
			BadRequest(w, "invalid flow_id")
			return
		}
		filter.FlowID = &flowID
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = domain.RunStatus(status)
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		var limit int
		if _, err := json.Number(limitStr).Int64(); err == nil {
			limit = int(mustParseInt(limitStr, 50))
		}
		filter.Limit = limit
	} else {
		filter.Limit = 50
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset = int(mustParseInt(offsetStr, 0))
	}

	runs, err := h.runRepo.List(r.Context(), filter)
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]RunResponse, len(runs))
	for i, run := range runs {
		result[i] = RunFromDomain(run)
	}

	List(w, result, len(result))
}

// CreateRun создаёт новый run для flow.
// POST /api/v1/flows/{id}/runs
func (h *Handler) CreateRun(w http.ResponseWriter, r *http.Request) {
	flowID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	var req CreateRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	// Проверяем, что flow существует
	flow, err := h.flowRepo.GetByID(r.Context(), flowID)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	// Определяем версию
	var version int
	if req.Version != nil {
		version = *req.Version
		// Проверяем, что версия существует
		_, err := h.flowRepo.GetVersion(r.Context(), flowID, version)
		if HandleRepoError(w, h.logger, err, "flow version not found") {
			return
		}
	} else {
		// Используем последнюю версию
		latestVersion, err := h.flowRepo.GetLatestVersion(r.Context(), flowID)
		if HandleRepoError(w, h.logger, err, "flow has no versions") {
			return
		}
		version = latestVersion.Version
	}

	// Проверяем idempotency key
	if req.IdempotencyKey != "" {
		existingRun, err := h.runRepo.GetByIdempotencyKey(r.Context(), flowID, req.IdempotencyKey)
		if err == nil && existingRun != nil {
			// Возвращаем существующий run
			Success(w, RunFromDomain(*existingRun))
			return
		}
	}

	run := &domain.Run{
		ID:             uuid.New(),
		FlowID:         flow.ID,
		Version:        version,
		Status:         domain.RunStatusPending,
		Inputs:         req.Inputs,
		IdempotencyKey: req.IdempotencyKey,
		IsSandbox:      req.IsSandbox,
	}

	if err := h.runRepo.Create(r.Context(), run); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	// Публикуем событие в очередь
	if h.publisher != nil {
		if err := h.publisher.PublishRunPending(r.Context(), run.ID); err != nil {
			h.logger.Warn("failed to publish run.pending", "run_id", run.ID, "error", err)
		}
	}

	Created(w, RunFromDomain(*run))
}

// GetRun возвращает run по ID.
// GET /api/v1/runs/{id}
func (h *Handler) GetRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid run id")
		return
	}

	run, err := h.runRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "run not found") {
		return
	}

	Success(w, RunFromDomain(*run))
}

// CancelRun отменяет run.
// POST /api/v1/runs/{id}/cancel
func (h *Handler) CancelRun(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid run id")
		return
	}

	run, err := h.runRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "run not found") {
		return
	}

	if run.IsFinished() {
		InvalidState(w, "run is already finished")
		return
	}

	run.MarkCancelled()

	if err := h.runRepo.Update(r.Context(), run); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Success(w, RunFromDomain(*run))
}

// ListRunTasks возвращает задачи run.
// GET /api/v1/runs/{id}/tasks
func (h *Handler) ListRunTasks(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid run id")
		return
	}

	// Проверяем, что run существует
	_, err = h.runRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "run not found") {
		return
	}

	tasks, err := h.taskRepo.ListByRunID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		result[i] = TaskFromDomain(t)
	}

	List(w, result, len(result))
}

// mustParseInt парсит строку в int с дефолтным значением.
func mustParseInt(s string, defaultVal int64) int64 {
	var n int64
	if _, err := json.Number(s).Int64(); err == nil {
		n, _ = json.Number(s).Int64()
		return n
	}
	return defaultVal
}
