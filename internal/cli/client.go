package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// --- Response types (дублируются из api/dto.go, CLI не импортирует internal/api) ---

// FlowResponse — flow из API.
type FlowResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

// FlowVersionResponse — версия flow из API.
type FlowVersionResponse struct {
	FlowID    string         `json:"flow_id"`
	Version   int            `json:"version"`
	Spec      map[string]any `json:"spec"`
	CreatedAt string         `json:"created_at"`
}

// RunResponse — run из API.
type RunResponse struct {
	ID             string         `json:"id"`
	FlowID         string         `json:"flow_id"`
	Version        int            `json:"version"`
	Status         string         `json:"status"`
	Inputs         map[string]any `json:"inputs,omitempty"`
	StartedAt      string         `json:"started_at,omitempty"`
	FinishedAt     string         `json:"finished_at,omitempty"`
	Error          string         `json:"error,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	IsSandbox      bool           `json:"is_sandbox"`
	CreatedAt      string         `json:"created_at"`
}

// TaskResponse — task из API.
type TaskResponse struct {
	ID         string         `json:"id"`
	RunID      string         `json:"run_id"`
	StepID     string         `json:"step_id"`
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Attempt    int            `json:"attempt"`
	Status     string         `json:"status"`
	Payload    map[string]any `json:"payload,omitempty"`
	Outputs    map[string]any `json:"outputs,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Error      string         `json:"error,omitempty"`
	CreatedAt  string         `json:"created_at"`
}

// ScheduleResponse — schedule из API.
type ScheduleResponse struct {
	ID          string         `json:"id"`
	FlowID      string         `json:"flow_id"`
	Name        string         `json:"name"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	IntervalSec int            `json:"interval_sec,omitempty"`
	Timezone    string         `json:"timezone"`
	Enabled     bool           `json:"enabled"`
	NextDueAt   string         `json:"next_due_at,omitempty"`
	LastRunAt   string         `json:"last_run_at,omitempty"`
	LastRunID   string         `json:"last_run_id,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

// --- Request types ---

// UpdateFlowRequest — обновление flow.
type UpdateFlowRequest struct {
	Name     *string `json:"name,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// CreateRunRequest — создание run.
type CreateRunRequest struct {
	Inputs         map[string]any `json:"inputs,omitempty"`
	Version        *int           `json:"version,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	IsSandbox      bool           `json:"is_sandbox,omitempty"`
}

// CreateScheduleRequest — создание schedule.
type CreateScheduleRequest struct {
	Name        string         `json:"name"`
	CronExpr    string         `json:"cron_expr,omitempty"`
	IntervalSec int            `json:"interval_sec,omitempty"`
	Timezone    string         `json:"timezone,omitempty"`
	Enabled     bool           `json:"enabled"`
	Inputs      map[string]any `json:"inputs,omitempty"`
}

// UpdateScheduleRequest — обновление schedule.
type UpdateScheduleRequest struct {
	Name        *string `json:"name,omitempty"`
	CronExpr    *string `json:"cron_expr,omitempty"`
	IntervalSec *int    `json:"interval_sec,omitempty"`
	Timezone    *string `json:"timezone,omitempty"`
}

// ListRunsOpts — параметры фильтрации runs.
type ListRunsOpts struct {
	FlowID string
	Status string
	Limit  int
}

// --- API response wrappers ---

type dataResponse struct {
	Data json.RawMessage `json:"data"`
}

type listResponse struct {
	Data  json.RawMessage `json:"data"`
	Total int             `json:"total"`
}

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// --- Client ---

// Client — HTTP-клиент для Automata API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient создаёт клиент для API.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// --- Flows ---

// ListFlows возвращает все flows.
func (c *Client) ListFlows() ([]FlowResponse, error) {
	var flows []FlowResponse
	err := c.list("/api/v1/flows", nil, &flows)
	return flows, err
}

// CreateFlow создаёт новый flow.
func (c *Client) CreateFlow(name string) (*FlowResponse, error) {
	body := map[string]string{"name": name}
	var flow FlowResponse
	err := c.post("/api/v1/flows", body, &flow)
	return &flow, err
}

// GetFlow возвращает flow по ID.
func (c *Client) GetFlow(id string) (*FlowResponse, error) {
	var flow FlowResponse
	err := c.get("/api/v1/flows/"+id, &flow)
	return &flow, err
}

// UpdateFlow обновляет flow.
func (c *Client) UpdateFlow(id string, req UpdateFlowRequest) (*FlowResponse, error) {
	var flow FlowResponse
	err := c.put("/api/v1/flows/"+id, req, &flow)
	return &flow, err
}

// DeleteFlow удаляет flow.
func (c *Client) DeleteFlow(id string) error {
	return c.delete("/api/v1/flows/" + id)
}

// ListVersions возвращает версии flow.
func (c *Client) ListVersions(flowID string) ([]FlowVersionResponse, error) {
	var versions []FlowVersionResponse
	err := c.list("/api/v1/flows/"+flowID+"/versions", nil, &versions)
	return versions, err
}

// CreateVersion создаёт новую версию flow.
func (c *Client) CreateVersion(flowID string, spec json.RawMessage) (*FlowVersionResponse, error) {
	body := map[string]json.RawMessage{"spec": spec}
	var version FlowVersionResponse
	err := c.post("/api/v1/flows/"+flowID+"/versions", body, &version)
	return &version, err
}

// --- Runs ---

// ListRuns возвращает список runs с фильтрацией.
func (c *Client) ListRuns(opts ListRunsOpts) ([]RunResponse, error) {
	params := url.Values{}
	if opts.FlowID != "" {
		params.Set("flow_id", opts.FlowID)
	}
	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}

	var runs []RunResponse
	err := c.list("/api/v1/runs", params, &runs)
	return runs, err
}

// CreateRun создаёт run для flow.
func (c *Client) CreateRun(flowID string, req CreateRunRequest) (*RunResponse, error) {
	var run RunResponse
	err := c.post("/api/v1/flows/"+flowID+"/runs", req, &run)
	return &run, err
}

// GetRun возвращает run по ID.
func (c *Client) GetRun(id string) (*RunResponse, error) {
	var run RunResponse
	err := c.get("/api/v1/runs/"+id, &run)
	return &run, err
}

// CancelRun отменяет run.
func (c *Client) CancelRun(id string) (*RunResponse, error) {
	var run RunResponse
	err := c.post("/api/v1/runs/"+id+"/cancel", nil, &run)
	return &run, err
}

// ListTasks возвращает tasks для run.
func (c *Client) ListTasks(runID string) ([]TaskResponse, error) {
	var tasks []TaskResponse
	err := c.list("/api/v1/runs/"+runID+"/tasks", nil, &tasks)
	return tasks, err
}

// --- Schedules ---

// ListSchedules возвращает schedules. Если flowID не пустой — фильтрует.
func (c *Client) ListSchedules(flowID string) ([]ScheduleResponse, error) {
	params := url.Values{}
	if flowID != "" {
		params.Set("flow_id", flowID)
	}

	var schedules []ScheduleResponse
	err := c.list("/api/v1/schedules", params, &schedules)
	return schedules, err
}

// CreateSchedule создаёт schedule для flow.
func (c *Client) CreateSchedule(flowID string, req CreateScheduleRequest) (*ScheduleResponse, error) {
	var schedule ScheduleResponse
	err := c.post("/api/v1/flows/"+flowID+"/schedules", req, &schedule)
	return &schedule, err
}

// GetSchedule возвращает schedule по ID.
func (c *Client) GetSchedule(id string) (*ScheduleResponse, error) {
	var schedule ScheduleResponse
	err := c.get("/api/v1/schedules/"+id, &schedule)
	return &schedule, err
}

// UpdateSchedule обновляет schedule.
func (c *Client) UpdateSchedule(id string, req UpdateScheduleRequest) (*ScheduleResponse, error) {
	var schedule ScheduleResponse
	err := c.put("/api/v1/schedules/"+id, req, &schedule)
	return &schedule, err
}

// DeleteSchedule удаляет schedule.
func (c *Client) DeleteSchedule(id string) error {
	return c.delete("/api/v1/schedules/" + id)
}

// EnableSchedule включает schedule.
func (c *Client) EnableSchedule(id string) (*ScheduleResponse, error) {
	var schedule ScheduleResponse
	body := map[string]bool{"enabled": true}
	err := c.put("/api/v1/schedules/"+id+"/enabled", body, &schedule)
	return &schedule, err
}

// DisableSchedule выключает schedule.
func (c *Client) DisableSchedule(id string) (*ScheduleResponse, error) {
	var schedule ScheduleResponse
	body := map[string]bool{"enabled": false}
	err := c.put("/api/v1/schedules/"+id+"/enabled", body, &schedule)
	return &schedule, err
}

// --- HTTP helpers ---

func (c *Client) get(path string, result any) error {
	return c.doData(http.MethodGet, path, nil, result)
}

func (c *Client) post(path string, body any, result any) error {
	return c.doData(http.MethodPost, path, body, result)
}

func (c *Client) put(path string, body any, result any) error {
	return c.doData(http.MethodPut, path, body, result)
}

func (c *Client) delete(path string) error {
	resp, err := c.do(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.checkError(resp)
}

func (c *Client) list(path string, params url.Values, result any) error {
	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	resp, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.checkError(resp); err != nil {
		return err
	}

	var lr listResponse
	if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return json.Unmarshal(lr.Data, result)
}

func (c *Client) doData(method, path string, body any, result any) error {
	resp, err := c.do(method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := c.checkError(resp); err != nil {
		return err
	}

	// 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var dr dataResponse
	if err := json.NewDecoder(resp.Body).Decode(&dr); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result != nil {
		return json.Unmarshal(dr.Data, result)
	}
	return nil
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func (c *Client) checkError(resp *http.Response) error {
	if resp.StatusCode < 400 {
		return nil
	}

	var er errorResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	return fmt.Errorf("%s: %s", er.Error.Code, er.Error.Message)
}
