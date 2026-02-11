package orchestrator

import "errors"

// Ошибки оркестратора.
var (
	// ErrRunNotFound — run не найден в БД.
	ErrRunNotFound = errors.New("run not found")

	// ErrFlowNotFound — flow или flow_version не найден.
	ErrFlowNotFound = errors.New("flow not found")

	// ErrVersionNotFound — версия flow не найдена.
	ErrVersionNotFound = errors.New("flow version not found")

	// ErrInvalidFlowSpec — FlowSpec не прошёл валидацию.
	ErrInvalidFlowSpec = errors.New("invalid flow spec")

	// ErrCyclicDependency — обнаружена циклическая зависимость в DAG.
	ErrCyclicDependency = errors.New("cyclic dependency in flow")

	// ErrRunAlreadyActive — run уже обрабатывается.
	ErrRunAlreadyActive = errors.New("run already being processed")

	// ErrRunNotActive — run не найден в активных (для обработки task.completed).
	ErrRunNotActive = errors.New("run not in active runs")

	// ErrRunNotPending — run не в статусе PENDING.
	ErrRunNotPending = errors.New("run is not in PENDING status")

	// ErrTaskNotFound — task не найден.
	ErrTaskNotFound = errors.New("task not found")

	// ErrStepNotFound — шаг не найден в DAG.
	ErrStepNotFound = errors.New("step not found in DAG")

	// ErrOrchestratorStopped — оркестратор остановлен.
	ErrOrchestratorStopped = errors.New("orchestrator stopped")
)
