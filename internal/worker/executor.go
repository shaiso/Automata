package worker

import (
	"context"
	"fmt"

	"github.com/shaiso/Automata/internal/domain"
)

// Executor — интерфейс для выполнения конкретного типа шага.
//
// Реализации: HTTPExecutor, DelayExecutor, TransformExecutor.
//
// task.Payload содержит отрендеренную конфигурацию шага.
// ctx может содержать таймаут, установленный из StepDef.TimeoutSec.
type Executor interface {
	Execute(ctx context.Context, task *domain.Task) (*ExecutionResult, error)
}

// ExecutionResult — результат выполнения task.
type ExecutionResult struct {
	// Outputs — выходные данные выполнения.
	Outputs map[string]any

	// Error — сообщение об ошибке (логическая ошибка выполнения).
	// Инфраструктурные ошибки возвращаются через error в Execute().
	Error string
}

// Registry — реестр executor'ов по типу шага.
type Registry struct {
	executors map[string]Executor
}

// NewRegistry создаёт реестр с зарегистрированными executor'ами по умолчанию.
//
// Регистрирует: http, delay, transform.
// parallel обрабатывается оркестратором, воркер получает только leaf-tasks.
func NewRegistry() *Registry {
	r := &Registry{executors: make(map[string]Executor)}
	r.Register("http", &HTTPExecutor{})
	r.Register("delay", &DelayExecutor{})
	r.Register("transform", &TransformExecutor{})
	return r
}

// Register добавляет executor для типа шага.
func (r *Registry) Register(stepType string, executor Executor) {
	r.executors[stepType] = executor
}

// Get возвращает executor для типа шага.
func (r *Registry) Get(stepType string) (Executor, error) {
	executor, ok := r.executors[stepType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownStepType, stepType)
	}
	return executor, nil
}
