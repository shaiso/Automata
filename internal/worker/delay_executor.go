package worker

import (
	"context"
	"time"

	"github.com/shaiso/Automata/internal/domain"
)

// DelayExecutor — executor для шага типа "delay".
//
// Ожидает указанное количество секунд. Поддерживает отмену через context.
//
// Config (из task.Payload):
//   - duration_sec (number): длительность задержки в секундах (default: 1)
type DelayExecutor struct{}

// Execute выполняет задержку.
func (e *DelayExecutor) Execute(ctx context.Context, task *domain.Task) (*ExecutionResult, error) {
	// Извлекаем duration_sec из payload
	durationSec := 1.0
	if val, ok := task.Payload["duration_sec"]; ok {
		switch v := val.(type) {
		case float64:
			durationSec = v
		case int:
			durationSec = float64(v)
		}
	}

	if durationSec <= 0 {
		durationSec = 1
	}

	duration := time.Duration(durationSec * float64(time.Second))

	// Context-aware ожидание
	select {
	case <-time.After(duration):
		return &ExecutionResult{
			Outputs: map[string]any{"delayed_sec": durationSec},
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
