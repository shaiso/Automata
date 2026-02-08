package steps

import (
	"context"
	"fmt"
	"time"
)

const (
	// StepTypeDelay — тип шага задержки.
	StepTypeDelay = "delay"

	// Ключи конфигурации delay.
	configDurationSec = "duration_sec"
	configDurationMs  = "duration_ms"
)

// DelayStep — шаг задержки.
//
// Приостанавливает выполнение на указанное время.
// Поддерживает graceful shutdown через context cancellation.
//
// Конфигурация:
//
//	{
//	    "duration_sec": 10,    // задержка в секундах
//	    // или
//	    "duration_ms": 5000    // задержка в миллисекундах
//	}
type DelayStep struct{}

// NewDelayStep создаёт новый DelayStep.
func NewDelayStep() *DelayStep {
	return &DelayStep{}
}

// Type возвращает тип шага.
func (s *DelayStep) Type() string {
	return StepTypeDelay
}

// Execute выполняет задержку.
func (s *DelayStep) Execute(ctx context.Context, req *Request) (*Response, error) {
	duration, err := s.parseDuration(req.Config)
	if err != nil {
		return nil, err
	}

	// Создаём таймер
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		// Контекст отменён — graceful shutdown
		return nil, fmt.Errorf("%w: %v", ErrStepCancelled, ctx.Err())
	case <-timer.C:
		// Задержка завершена
		return &Response{
			Outputs: map[string]any{
				"duration_ms": duration.Milliseconds(),
			},
		}, nil
	}
}

// parseDuration извлекает длительность из конфигурации.
func (s *DelayStep) parseDuration(config map[string]any) (time.Duration, error) {
	// Сначала проверяем duration_sec
	if sec := GetConfigInt(config, configDurationSec); sec > 0 {
		return time.Duration(sec) * time.Second, nil
	}

	// Затем проверяем duration_ms
	if ms := GetConfigInt(config, configDurationMs); ms > 0 {
		return time.Duration(ms) * time.Millisecond, nil
	}

	return 0, fmt.Errorf("%w: %s: duration_sec or duration_ms required",
		ErrInvalidConfig, StepTypeDelay)
}
