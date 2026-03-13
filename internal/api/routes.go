package api

import (
	"net/http"
)

// RegisterRoutes регистрирует все маршруты API.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Middleware chain
	chain := Chain(
		Recovery(h.logger),
		Logging(h.logger),
	)

	// Flows
	mux.Handle("GET /api/v1/flows", chain(http.HandlerFunc(h.ListFlows)))
	mux.Handle("POST /api/v1/flows", chain(http.HandlerFunc(h.CreateFlow)))
	mux.Handle("GET /api/v1/flows/{id}", chain(http.HandlerFunc(h.GetFlow)))
	mux.Handle("PUT /api/v1/flows/{id}", chain(http.HandlerFunc(h.UpdateFlow)))
	mux.Handle("DELETE /api/v1/flows/{id}", chain(http.HandlerFunc(h.DeleteFlow)))

	// Flow Versions
	mux.Handle("GET /api/v1/flows/{id}/versions", chain(http.HandlerFunc(h.ListFlowVersions)))
	mux.Handle("POST /api/v1/flows/{id}/versions", chain(http.HandlerFunc(h.CreateFlowVersion)))
	mux.Handle("GET /api/v1/flows/{id}/versions/{version}", chain(http.HandlerFunc(h.GetFlowVersion)))

	// Runs
	mux.Handle("GET /api/v1/runs", chain(http.HandlerFunc(h.ListRuns)))
	mux.Handle("POST /api/v1/flows/{id}/runs", chain(http.HandlerFunc(h.CreateRun)))
	mux.Handle("GET /api/v1/runs/{id}", chain(http.HandlerFunc(h.GetRun)))
	mux.Handle("POST /api/v1/runs/{id}/cancel", chain(http.HandlerFunc(h.CancelRun)))
	mux.Handle("GET /api/v1/runs/{id}/tasks", chain(http.HandlerFunc(h.ListRunTasks)))

	// Schedules
	mux.Handle("GET /api/v1/schedules", chain(http.HandlerFunc(h.ListSchedules)))
	mux.Handle("POST /api/v1/flows/{id}/schedules", chain(http.HandlerFunc(h.CreateSchedule)))
	mux.Handle("GET /api/v1/schedules/{id}", chain(http.HandlerFunc(h.GetSchedule)))
	mux.Handle("PUT /api/v1/schedules/{id}", chain(http.HandlerFunc(h.UpdateSchedule)))
	mux.Handle("DELETE /api/v1/schedules/{id}", chain(http.HandlerFunc(h.DeleteSchedule)))
	mux.Handle("PUT /api/v1/schedules/{id}/enabled", chain(http.HandlerFunc(h.SetScheduleEnabled)))

	// Proposals
	mux.Handle("GET /api/v1/proposals", chain(http.HandlerFunc(h.ListProposals)))
	mux.Handle("POST /api/v1/flows/{id}/proposals", chain(http.HandlerFunc(h.CreateProposal)))
	mux.Handle("GET /api/v1/proposals/{id}", chain(http.HandlerFunc(h.GetProposal)))
	mux.Handle("PUT /api/v1/proposals/{id}", chain(http.HandlerFunc(h.UpdateProposal)))
	mux.Handle("DELETE /api/v1/proposals/{id}", chain(http.HandlerFunc(h.DeleteProposal)))
	mux.Handle("POST /api/v1/proposals/{id}/submit", chain(http.HandlerFunc(h.SubmitProposal)))
	mux.Handle("POST /api/v1/proposals/{id}/approve", chain(http.HandlerFunc(h.ApproveProposal)))
	mux.Handle("POST /api/v1/proposals/{id}/reject", chain(http.HandlerFunc(h.RejectProposal)))
	mux.Handle("POST /api/v1/proposals/{id}/apply", chain(http.HandlerFunc(h.ApplyProposal)))
	mux.Handle("POST /api/v1/proposals/{id}/sandbox", chain(http.HandlerFunc(h.RunSandbox)))
}
