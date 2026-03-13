package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/repo"
	"github.com/shaiso/Automata/internal/sandbox"
)

// ListProposals возвращает список proposals с фильтрацией.
// GET /api/v1/proposals?flow_id=...&status=...
func (h *Handler) ListProposals(w http.ResponseWriter, r *http.Request) {
	filter := repo.ProposalFilter{}

	if flowIDStr := r.URL.Query().Get("flow_id"); flowIDStr != "" {
		flowID, err := uuid.Parse(flowIDStr)
		if err != nil {
			BadRequest(w, "invalid flow_id")
			return
		}
		filter.FlowID = &flowID
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status := domain.ParseProposalStatus(statusStr)
		filter.Status = &status
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		filter.Limit = int(mustParseInt(limitStr, 50))
	} else {
		filter.Limit = 50
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		filter.Offset = int(mustParseInt(offsetStr, 0))
	}

	proposals, err := h.proposalRepo.List(r.Context(), filter)
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]ProposalResponse, len(proposals))
	for i, p := range proposals {
		result[i] = ProposalFromDomain(p)
	}

	List(w, result, len(result))
}

// CreateProposal создаёт новый proposal для flow.
// POST /api/v1/flows/{id}/proposals
func (h *Handler) CreateProposal(w http.ResponseWriter, r *http.Request) {
	flowID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	var req CreateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	if req.Title == "" {
		BadRequest(w, "title is required")
		return
	}

	// Проверяем, что flow существует
	_, err = h.flowRepo.GetByID(r.Context(), flowID)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	// Определяем base_version (последняя версия flow, если есть)
	var baseVersion *int
	latestVersion, err := h.flowRepo.GetLatestVersion(r.Context(), flowID)
	if err == nil {
		baseVersion = &latestVersion.Version
	}

	now := time.Now()
	proposal := &domain.Proposal{
		ID:           uuid.New(),
		FlowID:       flowID,
		BaseVersion:  baseVersion,
		ProposedSpec: req.Spec,
		Status:       domain.ProposalStatusDraft,
		Title:        req.Title,
		Description:  req.Description,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.proposalRepo.Create(r.Context(), proposal); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Created(w, ProposalFromDomain(*proposal))
}

// GetProposal возвращает proposal по ID.
// GET /api/v1/proposals/{id}
func (h *Handler) GetProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// UpdateProposal обновляет proposal (только DRAFT).
// PUT /api/v1/proposals/{id}
func (h *Handler) UpdateProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	var req UpdateProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	if !proposal.CanEdit() {
		InvalidState(w, "proposal can only be edited in DRAFT status")
		return
	}

	if req.Title != nil {
		proposal.Title = *req.Title
	}
	if req.Description != nil {
		proposal.Description = *req.Description
	}
	if req.Spec != nil {
		proposal.ProposedSpec = *req.Spec
	}
	proposal.UpdatedAt = time.Now()

	if err := h.proposalRepo.Update(r.Context(), proposal); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// DeleteProposal удаляет proposal (только DRAFT).
// DELETE /api/v1/proposals/{id}
func (h *Handler) DeleteProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	if !proposal.CanEdit() {
		InvalidState(w, "only DRAFT proposals can be deleted")
		return
	}

	if err := h.proposalRepo.Delete(r.Context(), id); err != nil {
		HandleRepoError(w, h.logger, err, "proposal not found")
		return
	}

	NoContent(w)
}

// SubmitProposal отправляет proposal на review (DRAFT → PENDING_REVIEW).
// POST /api/v1/proposals/{id}/submit
func (h *Handler) SubmitProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	if err := h.proposalRepo.Submit(r.Context(), id); err != nil {
		HandleRepoError(w, h.logger, err, "proposal not found")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// ApproveProposal одобряет proposal (PENDING_REVIEW → APPROVED).
// POST /api/v1/proposals/{id}/approve
func (h *Handler) ApproveProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	var req ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	if req.Reviewer == "" {
		BadRequest(w, "reviewer is required")
		return
	}

	if err := h.proposalRepo.Approve(r.Context(), id, req.Reviewer, req.Comment); err != nil {
		HandleRepoError(w, h.logger, err, "proposal not found")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// RejectProposal отклоняет proposal (PENDING_REVIEW → REJECTED).
// POST /api/v1/proposals/{id}/reject
func (h *Handler) RejectProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	var req ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	if req.Reviewer == "" {
		BadRequest(w, "reviewer is required")
		return
	}

	if err := h.proposalRepo.Reject(r.Context(), id, req.Reviewer, req.Comment); err != nil {
		HandleRepoError(w, h.logger, err, "proposal not found")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// ApplyProposal применяет proposal — создаёт новую версию flow (APPROVED → APPLIED).
// POST /api/v1/proposals/{id}/apply
func (h *Handler) ApplyProposal(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	if !proposal.CanApply() {
		InvalidState(w, "proposal must be APPROVED to apply")
		return
	}

	// Создаём новую версию flow из proposed_spec
	version, err := h.flowRepo.CreateVersion(r.Context(), proposal.FlowID, proposal.ProposedSpec)
	if err != nil {
		InternalError(w, h.logger, err)
		return
	}

	// Отмечаем proposal как применённый
	if err := h.proposalRepo.MarkApplied(r.Context(), id, version.Version); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	// Перечитываем proposal с обновлёнными полями
	proposal, err = h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	Success(w, ProposalFromDomain(*proposal))
}

// RunSandbox запускает тестовое выполнение proposal в sandbox.
// POST /api/v1/proposals/{id}/sandbox
func (h *Handler) RunSandbox(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid proposal id")
		return
	}

	proposal, err := h.proposalRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "proposal not found") {
		return
	}

	// Sandbox можно запускать в любом статусе кроме APPLIED
	if proposal.Status == domain.ProposalStatusApplied {
		InvalidState(w, "cannot run sandbox for applied proposal")
		return
	}

	// Определяем base version.
	// Если версий ещё нет — version = 0 (placeholder, оркестратор использует spec_override).
	var version int
	if proposal.BaseVersion != nil {
		version = *proposal.BaseVersion
	}

	// Создаём sandbox run с ProposedSpec в spec_override.
	// Оркестратор использует spec_override вместо загрузки из flow_versions.
	run := &domain.Run{
		ID:           uuid.New(),
		FlowID:       proposal.FlowID,
		Version:      version,
		Status:       domain.RunStatusPending,
		IsSandbox:    true,
		SpecOverride: &proposal.ProposedSpec,
	}

	if err := h.runRepo.Create(r.Context(), run); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	// Публикуем событие для оркестратора
	if h.publisher != nil {
		if err := h.publisher.PublishRunPending(r.Context(), run.ID); err != nil {
			h.logger.Warn("failed to publish run.pending for sandbox", "run_id", run.ID, "error", err)
		}
	}

	// Сохраняем предыдущий результат как baseline для сравнения
	baseline := proposal.SandboxResult

	// Ожидаем завершения sandbox run и собираем результат
	sandboxResult, err := h.sandboxCollector.WaitForResult(r.Context(), run.ID, 2*time.Second, 60*time.Second)
	if err != nil {
		h.logger.Warn("failed to collect sandbox result", "run_id", run.ID, "error", err)
	}

	// Сравниваем с предыдущим результатом (если есть)
	if sandboxResult != nil && baseline != nil {
		sandboxResult = sandbox.CompareWithBaseline(baseline, sandboxResult)
	}

	// Сохраняем sandbox_run_id и результат в proposal
	proposal.SandboxRunID = &run.ID
	proposal.SandboxResult = sandboxResult
	proposal.UpdatedAt = time.Now()
	if err := h.proposalRepo.Update(r.Context(), proposal); err != nil {
		h.logger.Warn("failed to update proposal sandbox result", "proposal_id", id, "error", err)
	}

	Success(w, ProposalFromDomain(*proposal))
}
