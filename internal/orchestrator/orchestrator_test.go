package orchestrator

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
)

// --- RunState Tests ---

func TestNewRunState(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{}

	state := NewRunState(run, version)

	if state.Run != run {
		t.Error("Run should be set")
	}
	if state.FlowVersion != version {
		t.Error("FlowVersion should be set")
	}
	if state.completed == nil {
		t.Error("completed map should be initialized")
	}
	if state.running == nil {
		t.Error("running map should be initialized")
	}
	if state.failed == nil {
		t.Error("failed map should be initialized")
	}
	if state.tasks == nil {
		t.Error("tasks map should be initialized")
	}
}

func TestRunState_Initialize_EmptySpec(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{},
		},
	}

	state := NewRunState(run, version)
	err := state.Initialize()

	// Empty spec should fail validation
	if err == nil {
		t.Error("expected error for empty spec")
	}
}

func TestRunState_Initialize_ValidSpec(t *testing.T) {
	run := &domain.Run{
		ID:     uuid.New(),
		Inputs: map[string]any{"key": "value"},
	}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
			},
		},
	}

	state := NewRunState(run, version)
	err := state.Initialize()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.DAG == nil {
		t.Error("DAG should be built")
	}
	if state.Context == nil {
		t.Error("Context should be created")
	}
	if state.Context.Inputs["key"] != "value" {
		t.Error("Context should have run inputs")
	}
}

func TestRunState_MarkStepRunning(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	state := NewRunState(run, &domain.FlowVersion{})
	task := &domain.Task{ID: uuid.New(), StepID: "step1"}

	state.MarkStepRunning("step1", task)

	if !state.IsStepRunning("step1") {
		t.Error("step1 should be running")
	}
	if state.GetTask("step1") != task {
		t.Error("task should be stored")
	}
}

func TestRunState_MarkStepCompleted(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Mark as running first
	task := &domain.Task{ID: uuid.New(), StepID: "step1"}
	state.MarkStepRunning("step1", task)

	// Mark as completed
	outputs := map[string]any{"result": "success"}
	state.MarkStepCompleted("step1", outputs)

	if state.IsStepRunning("step1") {
		t.Error("step1 should not be running")
	}
	if !state.IsStepCompleted("step1") {
		t.Error("step1 should be completed")
	}

	// Check context has outputs
	stepCtx := state.Context.Steps["step1"]
	if stepCtx == nil {
		t.Fatal("step context should exist")
	}
	if stepCtx.Outputs["result"] != "success" {
		t.Error("step outputs should be in context")
	}
	if stepCtx.Status != "SUCCEEDED" {
		t.Errorf("expected status SUCCEEDED, got %s", stepCtx.Status)
	}
}

func TestRunState_MarkStepFailed(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Mark as running first
	task := &domain.Task{ID: uuid.New(), StepID: "step1"}
	state.MarkStepRunning("step1", task)

	// Mark as failed
	state.MarkStepFailed("step1", "connection error")

	if state.IsStepRunning("step1") {
		t.Error("step1 should not be running")
	}
	if !state.HasFailed() {
		t.Error("state should have failed steps")
	}

	failedSteps := state.GetFailedSteps()
	if len(failedSteps) != 1 || failedSteps[0] != "step1" {
		t.Error("step1 should be in failed steps")
	}

	// Check context has status
	stepCtx := state.Context.Steps["step1"]
	if stepCtx == nil {
		t.Fatal("step context should exist")
	}
	if stepCtx.Status != "FAILED" {
		t.Errorf("expected status FAILED, got %s", stepCtx.Status)
	}
}

func TestRunState_IsComplete(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step2", Type: "delay", DependsOn: []string{"step1"}, Config: map[string]any{"duration_sec": 1}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Not complete initially
	if state.IsComplete() {
		t.Error("should not be complete initially")
	}

	// Complete step1
	state.MarkStepCompleted("step1", nil)
	if state.IsComplete() {
		t.Error("should not be complete with only step1 done")
	}

	// Complete step2
	state.MarkStepCompleted("step2", nil)
	if !state.IsComplete() {
		t.Error("should be complete with all steps done")
	}
}

func TestRunState_IsComplete_WithFailed(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Mark as failed
	state.MarkStepFailed("step1", "error")

	// Should be complete (failed counts as finished)
	if !state.IsComplete() {
		t.Error("should be complete when all steps are done (even if failed)")
	}
}

func TestRunState_GetReadySteps(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step2", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step3", Type: "delay", DependsOn: []string{"step1", "step2"}, Config: map[string]any{"duration_sec": 1}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Initially step1 and step2 are ready
	ready := state.GetReadySteps()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready steps, got %d", len(ready))
	}

	readyIDs := make(map[string]bool)
	for _, node := range ready {
		readyIDs[node.ID] = true
	}
	if !readyIDs["step1"] || !readyIDs["step2"] {
		t.Error("step1 and step2 should be ready")
	}

	// Mark step1 as running
	state.MarkStepRunning("step1", &domain.Task{})

	ready = state.GetReadySteps()
	if len(ready) != 1 {
		t.Errorf("expected 1 ready step, got %d", len(ready))
	}
	if ready[0].ID != "step2" {
		t.Errorf("expected step2 to be ready, got %s", ready[0].ID)
	}

	// Complete both step1 and step2
	state.MarkStepCompleted("step1", nil)
	state.MarkStepCompleted("step2", nil)

	ready = state.GetReadySteps()
	if len(ready) != 1 {
		t.Errorf("expected 1 ready step, got %d", len(ready))
	}
	if ready[0].ID != "step3" {
		t.Errorf("expected step3 to be ready, got %s", ready[0].ID)
	}
}

func TestRunState_Stats(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step2", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step3", Type: "delay", Config: map[string]any{"duration_sec": 1}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Initial stats
	stats := state.Stats()
	if stats.TotalSteps != 3 {
		t.Errorf("expected 3 total steps, got %d", stats.TotalSteps)
	}
	if stats.PendingSteps != 3 {
		t.Errorf("expected 3 pending steps, got %d", stats.PendingSteps)
	}
	if stats.RunningSteps != 0 {
		t.Errorf("expected 0 running steps, got %d", stats.RunningSteps)
	}
	if stats.CompletedSteps != 0 {
		t.Errorf("expected 0 completed steps, got %d", stats.CompletedSteps)
	}

	// Mark step1 running
	state.MarkStepRunning("step1", &domain.Task{})
	stats = state.Stats()
	if stats.RunningSteps != 1 {
		t.Errorf("expected 1 running step, got %d", stats.RunningSteps)
	}
	if stats.PendingSteps != 2 {
		t.Errorf("expected 2 pending steps, got %d", stats.PendingSteps)
	}

	// Complete step1, fail step2
	state.MarkStepCompleted("step1", nil)
	state.MarkStepFailed("step2", "error")
	stats = state.Stats()
	if stats.CompletedSteps != 1 {
		t.Errorf("expected 1 completed step, got %d", stats.CompletedSteps)
	}
	if stats.FailedSteps != 1 {
		t.Errorf("expected 1 failed step, got %d", stats.FailedSteps)
	}
	if stats.PendingSteps != 1 {
		t.Errorf("expected 1 pending step, got %d", stats.PendingSteps)
	}
}

func TestRunState_RestoreFromTasks(t *testing.T) {
	run := &domain.Run{ID: uuid.New()}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step2", Type: "http", Config: map[string]any{"url": "http://example.com"}},
				{ID: "step3", Type: "delay", Config: map[string]any{"duration_sec": 1}},
				{ID: "step4", Type: "delay", Config: map[string]any{"duration_sec": 1}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// Simulate tasks from DB
	tasks := []domain.Task{
		{
			ID:      uuid.New(),
			StepID:  "step1",
			Status:  domain.TaskStatusSucceeded,
			Outputs: map[string]any{"data": "result1"},
		},
		{
			ID:     uuid.New(),
			StepID: "step2",
			Status: domain.TaskStatusFailed,
		},
		{
			ID:     uuid.New(),
			StepID: "step3",
			Status: domain.TaskStatusRunning,
		},
		{
			ID:     uuid.New(),
			StepID: "step4",
			Status: domain.TaskStatusQueued,
		},
	}

	state.RestoreFromTasks(tasks)

	// Check step1 is completed
	if !state.IsStepCompleted("step1") {
		t.Error("step1 should be completed")
	}
	if state.Context.Steps["step1"].Outputs["data"] != "result1" {
		t.Error("step1 outputs should be in context")
	}

	// Check step2 is failed
	if !state.HasFailed() {
		t.Error("state should have failed steps")
	}
	failedSteps := state.GetFailedSteps()
	found := false
	for _, s := range failedSteps {
		if s == "step2" {
			found = true
			break
		}
	}
	if !found {
		t.Error("step2 should be in failed steps")
	}

	// Check step3 is running
	if !state.IsStepRunning("step3") {
		t.Error("step3 should be running")
	}

	// Check step4 is not in any state (queued)
	if state.IsStepCompleted("step4") || state.IsStepRunning("step4") {
		t.Error("step4 should not be completed or running")
	}

	// Check tasks are stored
	if state.GetTask("step1") == nil {
		t.Error("step1 task should be stored")
	}
}

func TestRunState_RunID(t *testing.T) {
	runID := uuid.New()
	run := &domain.Run{ID: runID}
	state := NewRunState(run, &domain.FlowVersion{})

	if state.RunID() != runID {
		t.Error("RunID should return run ID")
	}
}

func TestRunState_FlowID(t *testing.T) {
	flowID := uuid.New()
	run := &domain.Run{ID: uuid.New(), FlowID: flowID}
	state := NewRunState(run, &domain.FlowVersion{})

	if state.FlowID() != flowID {
		t.Error("FlowID should return flow ID")
	}
}

// --- Orchestrator Tests ---

func TestNew(t *testing.T) {
	orch := New(Config{})

	if orch.activeRuns == nil {
		t.Error("activeRuns should be initialized")
	}
	if orch.pollInterval != defaultPollInterval {
		t.Errorf("expected default poll interval %v, got %v", defaultPollInterval, orch.pollInterval)
	}
	if orch.batchSize != defaultBatchSize {
		t.Errorf("expected default batch size %d, got %d", defaultBatchSize, orch.batchSize)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	orch := New(Config{
		PollInterval: 5 * time.Second,
		BatchSize:    50,
	})

	if orch.pollInterval != 5*time.Second {
		t.Errorf("expected poll interval 5s, got %v", orch.pollInterval)
	}
	if orch.batchSize != 50 {
		t.Errorf("expected batch size 50, got %d", orch.batchSize)
	}
}

func TestOrchestrator_ActiveRuns(t *testing.T) {
	orch := New(Config{})

	runID := uuid.New()
	state := &RunState{
		Run: &domain.Run{ID: runID},
	}

	// Initially no active runs
	if orch.ActiveRunsCount() != 0 {
		t.Error("should have no active runs initially")
	}
	if orch.isRunActive(runID) {
		t.Error("run should not be active initially")
	}

	// Add active run
	err := orch.addActiveRun(state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if orch.ActiveRunsCount() != 1 {
		t.Error("should have 1 active run")
	}
	if !orch.isRunActive(runID) {
		t.Error("run should be active")
	}
	if orch.getActiveRun(runID) != state {
		t.Error("getActiveRun should return the state")
	}

	// Try to add same run again
	err = orch.addActiveRun(state)
	if err != ErrRunAlreadyActive {
		t.Errorf("expected ErrRunAlreadyActive, got %v", err)
	}

	// Remove active run
	orch.removeActiveRun(runID)

	if orch.ActiveRunsCount() != 0 {
		t.Error("should have no active runs after removal")
	}
	if orch.isRunActive(runID) {
		t.Error("run should not be active after removal")
	}
}

func TestOrchestrator_GetActiveRunStats(t *testing.T) {
	orch := New(Config{})

	runID := uuid.New()
	run := &domain.Run{ID: runID}
	version := &domain.FlowVersion{
		Spec: domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "step1", Type: "http", Config: map[string]any{"url": "http://example.com"}},
			},
		},
	}
	state := NewRunState(run, version)
	_ = state.Initialize()

	// No stats for non-existent run
	_, ok := orch.GetActiveRunStats(runID)
	if ok {
		t.Error("should not find stats for non-active run")
	}

	// Add run and get stats
	_ = orch.addActiveRun(state)
	stats, ok := orch.GetActiveRunStats(runID)
	if !ok {
		t.Fatal("should find stats for active run")
	}
	if stats.TotalSteps != 1 {
		t.Errorf("expected 1 total step, got %d", stats.TotalSteps)
	}
}

func TestOrchestrator_IsStopped(t *testing.T) {
	orch := New(Config{})

	if orch.IsStopped() {
		t.Error("should not be stopped initially")
	}

	// Set stopped state directly (simulating Stop() call)
	orch.stoppedMu.Lock()
	orch.stopped = true
	orch.stoppedMu.Unlock()

	if !orch.IsStopped() {
		t.Error("should be stopped")
	}
}
