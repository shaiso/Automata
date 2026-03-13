package sandbox

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/repo"
)

// Collector собирает результаты sandbox run и формирует SandboxResult.
type Collector struct {
	runRepo  *repo.RunRepo
	taskRepo *repo.TaskRepo
}

// NewCollector создаёт новый Collector.
func NewCollector(runRepo *repo.RunRepo, taskRepo *repo.TaskRepo) *Collector {
	return &Collector{
		runRepo:  runRepo,
		taskRepo: taskRepo,
	}
}

// Collect собирает результат sandbox run.
// Возвращает nil, если run ещё не завершён.
func (c *Collector) Collect(ctx context.Context, runID uuid.UUID) (*domain.SandboxResult, error) {
	run, err := c.runRepo.GetByID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}

	if !run.Status.IsTerminal() {
		return nil, nil
	}

	tasks, err := c.taskRepo.ListByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	result := &domain.SandboxResult{
		RunID:  runID,
		Status: run.Status,
	}

	if run.StartedAt != nil {
		result.StartedAt = *run.StartedAt
	}
	if run.FinishedAt != nil {
		result.FinishedAt = *run.FinishedAt
	}

	summary := domain.SandboxSummary{
		TotalSteps: len(tasks),
	}

	steps := make([]domain.StepResult, 0, len(tasks))
	for _, t := range tasks {
		sr := domain.StepResult{
			StepID:       t.StepID,
			Status:       t.Status,
			ActualOutput: t.Outputs,
		}

		if t.StartedAt != nil && t.FinishedAt != nil {
			sr.DurationMs = t.FinishedAt.Sub(*t.StartedAt).Milliseconds()
		}

		if t.Error != "" {
			sr.Error = t.Error
		}

		switch t.Status {
		case domain.TaskStatusSucceeded:
			summary.Succeeded++
		case domain.TaskStatusFailed:
			summary.Failed++
		}

		steps = append(steps, sr)
	}

	result.Steps = steps
	result.Summary = summary

	return result, nil
}

// Differ выполняет deep diff двух map[string]any.
type Differ struct{}

// NewDiffer создаёт новый Differ.
func NewDiffer() *Differ {
	return &Differ{}
}

// Diff сравнивает expected и actual, возвращает список различий.
func (d *Differ) Diff(expected, actual map[string]any) []domain.OutputDiff {
	var diffs []domain.OutputDiff
	d.diffRecursive("", expected, actual, &diffs)
	return diffs
}

func (d *Differ) diffRecursive(prefix string, expected, actual map[string]any, diffs *[]domain.OutputDiff) {
	// Ключи из expected
	for key, expVal := range expected {
		path := joinPath(prefix, key)
		actVal, exists := actual[key]

		if !exists {
			*diffs = append(*diffs, domain.OutputDiff{
				Path:     path,
				Expected: expVal,
				Actual:   nil,
			})
			continue
		}

		d.diffValues(path, expVal, actVal, diffs)
	}

	// Ключи только в actual
	for key, actVal := range actual {
		path := joinPath(prefix, key)
		if _, exists := expected[key]; !exists {
			*diffs = append(*diffs, domain.OutputDiff{
				Path:     path,
				Expected: nil,
				Actual:   actVal,
			})
		}
	}
}

func (d *Differ) diffValues(path string, expected, actual any, diffs *[]domain.OutputDiff) {
	// Оба map — рекурсия
	expMap, expOk := expected.(map[string]any)
	actMap, actOk := actual.(map[string]any)
	if expOk && actOk {
		d.diffRecursive(path, expMap, actMap, diffs)
		return
	}

	// Оба slice
	expSlice, expOk := expected.([]any)
	actSlice, actOk := actual.([]any)
	if expOk && actOk {
		d.diffSlices(path, expSlice, actSlice, diffs)
		return
	}

	// Скалярное сравнение
	if !reflect.DeepEqual(expected, actual) {
		*diffs = append(*diffs, domain.OutputDiff{
			Path:     path,
			Expected: expected,
			Actual:   actual,
		})
	}
}

func (d *Differ) diffSlices(path string, expected, actual []any, diffs *[]domain.OutputDiff) {
	maxLen := len(expected)
	if len(actual) > maxLen {
		maxLen = len(actual)
	}

	for i := 0; i < maxLen; i++ {
		elemPath := fmt.Sprintf("%s[%d]", path, i)
		if i >= len(expected) {
			*diffs = append(*diffs, domain.OutputDiff{
				Path:     elemPath,
				Expected: nil,
				Actual:   actual[i],
			})
		} else if i >= len(actual) {
			*diffs = append(*diffs, domain.OutputDiff{
				Path:     elemPath,
				Expected: expected[i],
				Actual:   nil,
			})
		} else {
			d.diffValues(elemPath, expected[i], actual[i], diffs)
		}
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// WaitForResult ожидает завершения sandbox run и собирает результат.
// Polling с интервалом pollInterval, максимальное ожидание timeout.
func (c *Collector) WaitForResult(ctx context.Context, runID uuid.UUID, pollInterval, timeout time.Duration) (*domain.SandboxResult, error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, fmt.Errorf("timeout waiting for sandbox run %s", runID)
		case <-ticker.C:
			result, err := c.Collect(ctx, runID)
			if err != nil {
				return nil, err
			}
			if result != nil {
				return result, nil
			}
		}
	}
}

// CompareWithBaseline сравнивает результаты sandbox run с baseline (предыдущим run).
func CompareWithBaseline(baseline, sandbox *domain.SandboxResult) *domain.SandboxResult {
	if baseline == nil || sandbox == nil {
		return sandbox
	}

	differ := NewDiffer()

	// Создаём map step_id → StepResult для baseline
	baselineSteps := make(map[string]domain.StepResult, len(baseline.Steps))
	for _, s := range baseline.Steps {
		baselineSteps[s.StepID] = s
	}

	withDiffs := 0
	for i, step := range sandbox.Steps {
		baseStep, ok := baselineSteps[step.StepID]
		if !ok {
			continue
		}

		sandbox.Steps[i].ExpectedOutput = baseStep.ActualOutput

		if baseStep.ActualOutput != nil && step.ActualOutput != nil {
			diffs := differ.Diff(baseStep.ActualOutput, step.ActualOutput)
			sandbox.Steps[i].Diff = diffs
			if len(diffs) > 0 {
				withDiffs++
			}
		}
	}

	sandbox.Summary.WithDiffs = withDiffs

	return sandbox
}
