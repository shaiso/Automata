package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
)

// ListFlows возвращает список всех flows.
// GET /api/v1/flows
func (h *Handler) ListFlows(w http.ResponseWriter, r *http.Request) {
	flows, err := h.flowRepo.List(r.Context())
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]FlowResponse, len(flows))
	for i, f := range flows {
		result[i] = FlowFromDomain(f)
	}

	List(w, result, len(result))
}

// CreateFlow создаёт новый flow.
// POST /api/v1/flows
func (h *Handler) CreateFlow(w http.ResponseWriter, r *http.Request) {
	var req CreateFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	if req.Name == "" {
		BadRequest(w, "name is required")
		return
	}

	flow := &domain.Flow{
		ID:       uuid.New(),
		Name:     req.Name,
		IsActive: false,
	}

	if err := h.flowRepo.Create(r.Context(), flow); err != nil {
		if HandleRepoError(w, h.logger, err, "") {
			return
		}
		InternalError(w, h.logger, err)
		return
	}

	Created(w, FlowFromDomain(*flow))
}

// GetFlow возвращает flow по ID.
// GET /api/v1/flows/{id}
func (h *Handler) GetFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	flow, err := h.flowRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	Success(w, FlowFromDomain(*flow))
}

// UpdateFlow обновляет flow.
// PUT /api/v1/flows/{id}
func (h *Handler) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	var req UpdateFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	flow, err := h.flowRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	if req.Name != nil {
		flow.Name = *req.Name
	}
	if req.IsActive != nil {
		flow.IsActive = *req.IsActive
	}

	if err := h.flowRepo.Update(r.Context(), flow); err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Success(w, FlowFromDomain(*flow))
}

// DeleteFlow удаляет flow.
// DELETE /api/v1/flows/{id}
func (h *Handler) DeleteFlow(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	if err := h.flowRepo.Delete(r.Context(), id); err != nil {
		if HandleRepoError(w, h.logger, err, "flow not found") {
			return
		}
		InternalError(w, h.logger, err)
		return
	}

	NoContent(w)
}

// ListFlowVersions возвращает список версий flow.
// GET /api/v1/flows/{id}/versions
func (h *Handler) ListFlowVersions(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	// Проверяем, что flow существует
	_, err = h.flowRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	versions, err := h.flowRepo.ListVersions(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "") {
		return
	}

	result := make([]FlowVersionResponse, len(versions))
	for i, v := range versions {
		result[i] = FlowVersionFromDomain(v)
	}

	List(w, result, len(result))
}

// CreateFlowVersion создаёт новую версию flow.
// POST /api/v1/flows/{id}/versions
func (h *Handler) CreateFlowVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	var req CreateFlowVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		BadRequest(w, "invalid request body")
		return
	}

	// Проверяем, что flow существует
	_, err = h.flowRepo.GetByID(r.Context(), id)
	if HandleRepoError(w, h.logger, err, "flow not found") {
		return
	}

	version, err := h.flowRepo.CreateVersion(r.Context(), id, req.Spec)
	if err != nil {
		InternalError(w, h.logger, err)
		return
	}

	Created(w, FlowVersionFromDomain(*version))
}

// GetFlowVersion возвращает конкретную версию flow.
// GET /api/v1/flows/{id}/versions/{version}
func (h *Handler) GetFlowVersion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		BadRequest(w, "invalid flow id")
		return
	}

	versionNum, err := strconv.Atoi(r.PathValue("version"))
	if err != nil {
		BadRequest(w, "invalid version number")
		return
	}

	version, err := h.flowRepo.GetVersion(r.Context(), id, versionNum)
	if HandleRepoError(w, h.logger, err, "flow version not found") {
		return
	}

	Success(w, FlowVersionFromDomain(*version))
}
