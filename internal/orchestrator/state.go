package orchestrator

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/engine"
)

// RunState — состояние выполнения одного run в памяти.
//
// RunState создаётся когда Orchestrator начинает обработку run
// и удаляется когда run завершается (SUCCEEDED/FAILED/CANCELLED).
//
// Содержит:
//   - Кэш данных из БД (Run, FlowVersion)
//   - Построенный DAG
//   - Контекст для шаблонов (с outputs завершённых шагов)
//   - Отслеживание статуса каждого шага
type RunState struct {
	// Run — данные run из БД.
	Run *domain.Run

	// FlowVersion — версия flow с FlowSpec.
	FlowVersion *domain.FlowVersion

	// DAG — граф зависимостей шагов.
	DAG *engine.DAG

	// Context — контекст для рендеринга шаблонов.
	// Содержит Inputs и Outputs завершённых шагов.
	Context *engine.Context

	// completed — завершённые шаги (stepID → true).
	completed map[string]bool

	// running — шаги в процессе выполнения (stepID → true).
	running map[string]bool

	// failed — упавшие шаги (stepID → true).
	failed map[string]bool

	// tasks — созданные tasks (stepID → Task).
	tasks map[string]*domain.Task

	// mu — мьютекс для потокобезопасного доступа.
	mu sync.RWMutex
}

// NewRunState создаёт новый RunState.
func NewRunState(run *domain.Run, version *domain.FlowVersion) *RunState {
	return &RunState{
		Run:         run,
		FlowVersion: version,
		completed:   make(map[string]bool),
		running:     make(map[string]bool),
		failed:      make(map[string]bool),
		tasks:       make(map[string]*domain.Task),
	}
}

// Initialize инициализирует RunState: валидирует FlowSpec, строит DAG, создаёт Context.
func (s *RunState) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	spec := &s.FlowVersion.Spec

	// 1. Валидация FlowSpec
	if err := engine.Validate(spec); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidFlowSpec, err)
	}

	// 2. Построение DAG
	dag, err := engine.BuildDAG(spec)
	if err != nil {
		return fmt.Errorf("build DAG: %w", err)
	}
	s.DAG = dag

	// 3. Создание контекста с inputs
	s.Context = engine.NewContext(s.Run.Inputs)

	return nil
}

// GetReadySteps возвращает шаги, готовые к выполнению.
// Шаг готов, если все его зависимости завершены и он ещё не запущен.
func (s *RunState) GetReadySteps() []*engine.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.DAG.GetReadyNodes(s.completed, s.running)
}

// MarkStepRunning помечает шаг как выполняющийся.
func (s *RunState) MarkStepRunning(stepID string, task *domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running[stepID] = true
	s.tasks[stepID] = task
}

// MarkStepCompleted помечает шаг как успешно завершённый.
// Добавляет outputs в Context для использования в следующих шагах.
func (s *RunState) MarkStepCompleted(stepID string, outputs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, stepID)
	s.completed[stepID] = true

	// Добавляем результат в контекст
	s.Context.AddStepResult(stepID, outputs, string(domain.TaskStatusSucceeded))
}

// MarkStepFailed помечает шаг как упавший.
func (s *RunState) MarkStepFailed(stepID string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.running, stepID)
	s.failed[stepID] = true

	// Добавляем результат в контекст (со статусом FAILED)
	s.Context.AddStepResult(stepID, nil, string(domain.TaskStatusFailed))
}

// IsStepRunning проверяет, выполняется ли шаг.
func (s *RunState) IsStepRunning(stepID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.running[stepID]
}

// IsStepCompleted проверяет, завершён ли шаг.
func (s *RunState) IsStepCompleted(stepID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.completed[stepID]
}

// GetTask возвращает task для шага.
func (s *RunState) GetTask(stepID string) *domain.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tasks[stepID]
}

// SetTask устанавливает task для шага.
func (s *RunState) SetTask(stepID string, task *domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[stepID] = task
}

// IsComplete проверяет, все ли шаги завершены (успешно или с ошибкой).
func (s *RunState) IsComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Проверяем, что все исполняемые узлы завершены
	for _, node := range s.DAG.GetExecutableNodes() {
		if !s.completed[node.ID] && !s.failed[node.ID] {
			return false
		}
	}
	return true
}

// HasFailed проверяет, есть ли упавшие шаги.
func (s *RunState) HasFailed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.failed) > 0
}

// GetFailedSteps возвращает список упавших шагов.
func (s *RunState) GetFailedSteps() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	steps := make([]string, 0, len(s.failed))
	for stepID := range s.failed {
		steps = append(steps, stepID)
	}
	return steps
}

// RunID возвращает ID run.
func (s *RunState) RunID() uuid.UUID {
	return s.Run.ID
}

// FlowID возвращает ID flow.
func (s *RunState) FlowID() uuid.UUID {
	return s.Run.FlowID
}

// Stats возвращает статистику выполнения.
func (s *RunState) Stats() RunStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.DAG.GetExecutableNodes())
	return RunStats{
		TotalSteps:     total,
		CompletedSteps: len(s.completed),
		RunningSteps:   len(s.running),
		FailedSteps:    len(s.failed),
		PendingSteps:   total - len(s.completed) - len(s.running) - len(s.failed),
	}
}

// RunStats — статистика выполнения run.
type RunStats struct {
	TotalSteps     int
	CompletedSteps int
	RunningSteps   int
	FailedSteps    int
	PendingSteps   int
}

// RestoreFromTasks восстанавливает состояние из списка tasks (после рестарта).
func (s *RunState) RestoreFromTasks(tasks []domain.Task) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range tasks {
		task := &tasks[i]
		s.tasks[task.StepID] = task

		switch task.Status {
		case domain.TaskStatusSucceeded:
			s.completed[task.StepID] = true
			s.Context.AddStepResult(task.StepID, task.Outputs, string(domain.TaskStatusSucceeded))

		case domain.TaskStatusFailed:
			s.failed[task.StepID] = true
			s.Context.AddStepResult(task.StepID, nil, string(domain.TaskStatusFailed))

		case domain.TaskStatusRunning:
			s.running[task.StepID] = true

		case domain.TaskStatusQueued:
			// Task в очереди — ничего не делаем, будет обработан worker'ом
		}
	}
}
