package worker

import (
	"context"

	"github.com/shaiso/Automata/internal/domain"
)

// TransformExecutor — executor для шага типа "transform".
//
// Оркестратор уже отрендерил config через engine.RenderConfig()
// при создании task (handlers.go:dispatchStep). Поэтому task.Payload
// уже содержит готовые значения после подстановки шаблонов.
//
// Transform просто возвращает payload как outputs —
// это "pass-through с template expansion" для трансформации данных между шагами.
type TransformExecutor struct{}

// Execute возвращает payload как outputs.
func (e *TransformExecutor) Execute(_ context.Context, task *domain.Task) (*ExecutionResult, error) {
	outputs := task.Payload
	if outputs == nil {
		outputs = make(map[string]any)
	}

	return &ExecutionResult{
		Outputs: outputs,
	}, nil
}
