package steps

import (
	"context"
	"fmt"
)

const (
	// StepTypeParallel — тип шага параллельного выполнения.
	StepTypeParallel = "parallel"
)

// ParallelStep — шаг параллельного выполнения веток.
//
// ВАЖНО: Сам parallel шаг не выполняет вложенные шаги напрямую.
// Orchestrator "разворачивает" parallel в DAG и выполняет шаги веток
// как отдельные tasks.
//
// Этот step используется для:
// 1. Валидации конфигурации (через Registry)
// 2. Сбора outputs всех веток в единый результат
//
// Конфигурация задаётся через domain.StepDef.Branches:
//
//	{
//	    "id": "parallel_step",
//	    "type": "parallel",
//	    "branches": [
//	        {
//	            "id": "branch_a",
//	            "steps": [
//	                {"id": "step1", "type": "http", ...}
//	            ]
//	        },
//	        {
//	            "id": "branch_b",
//	            "steps": [
//	                {"id": "step1", "type": "delay", ...}
//	            ]
//	        }
//	    ]
//	}
//
// Outputs (собираются Orchestrator'ом):
//
//	{
//	    "branch_a": {
//	        "step1": { ... outputs step1 ... }
//	    },
//	    "branch_b": {
//	        "step1": { ... outputs step1 ... }
//	    }
//	}
type ParallelStep struct{}

// NewParallelStep создаёт новый ParallelStep.
func NewParallelStep() *ParallelStep {
	return &ParallelStep{}
}

// Type возвращает тип шага.
func (s *ParallelStep) Type() string {
	return StepTypeParallel
}

// Execute для parallel шага не делает ничего, т.к. выполнение
// координируется Orchestrator'ом через DAG.
//
// Этот метод вызывается только для "start" узла parallel,
// когда все зависимости удовлетворены. Он просто возвращает
// пустой результат — реальная работа происходит в вложенных шагах.
func (s *ParallelStep) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Проверяем context
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrStepCancelled, ctx.Err())
	default:
	}

	// Parallel step сам по себе ничего не делает
	// Orchestrator создаёт tasks для шагов внутри веток
	// и собирает их outputs в AggregateParallelOutputs
	return EmptyResponse(), nil
}

// AggregateParallelOutputs собирает outputs всех шагов веток
// в единый результат для parallel шага.
//
// Параметры:
//   - parallelID: ID parallel шага
//   - branchOutputs: map[branchID]map[stepID]outputs
//
// Возвращает outputs в формате:
//
//	{
//	    "branch_a": { "step1": {...}, "step2": {...} },
//	    "branch_b": { "step1": {...} }
//	}
func AggregateParallelOutputs(branchOutputs map[string]map[string]map[string]any) map[string]any {
	result := make(map[string]any, len(branchOutputs))

	for branchID, steps := range branchOutputs {
		branchResult := make(map[string]any, len(steps))
		for stepID, outputs := range steps {
			branchResult[stepID] = outputs
		}
		result[branchID] = branchResult
	}

	return result
}

// ExtractBranchOutputs извлекает outputs конкретной ветки из aggregated outputs.
func ExtractBranchOutputs(outputs map[string]any, branchID string) map[string]any {
	if branch, ok := outputs[branchID]; ok {
		if branchMap, ok := branch.(map[string]any); ok {
			return branchMap
		}
	}
	return nil
}

// ExtractStepOutputs извлекает outputs конкретного шага из ветки.
func ExtractStepOutputs(outputs map[string]any, branchID, stepID string) map[string]any {
	branchOutputs := ExtractBranchOutputs(outputs, branchID)
	if branchOutputs == nil {
		return nil
	}

	if step, ok := branchOutputs[stepID]; ok {
		if stepMap, ok := step.(map[string]any); ok {
			return stepMap
		}
	}
	return nil
}
