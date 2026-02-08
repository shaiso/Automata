package steps

import (
	"context"
	"errors"
	"time"

	"github.com/shaiso/Automata/internal/engine"
)

// Ошибки шагов.
var (
	// ErrStepNotFound — тип шага не найден в реестре.
	ErrStepNotFound = errors.New("step type not found")

	// ErrInvalidConfig — невалидная конфигурация шага.
	ErrInvalidConfig = errors.New("invalid step config")

	// ErrStepTimeout — шаг превысил таймаут.
	ErrStepTimeout = errors.New("step execution timeout")

	// ErrStepCancelled — выполнение шага отменено.
	ErrStepCancelled = errors.New("step execution cancelled")
)

// Step — интерфейс для типов шагов.
//
// Каждый тип шага (http, delay, transform, parallel) реализует этот интерфейс.
type Step interface {
	// Type возвращает тип шага.
	Type() string

	// Execute выполняет шаг и возвращает результат.
	// Шаг должен проверять ctx.Done() для graceful shutdown.
	Execute(ctx context.Context, req *Request) (*Response, error)
}

// Request — входные данные для выполнения шага.
type Request struct {
	// StepID — идентификатор шага.
	StepID string

	// Config — конфигурация шага (уже отрендеренная через engine.RenderConfig).
	Config map[string]any

	// Context — контекст с данными предыдущих шагов.
	// Используется для доступа к outputs других шагов внутри step.
	TemplateContext *engine.Context

	// Timeout — таймаут выполнения шага.
	// Если 0, используется таймаут по умолчанию.
	Timeout time.Duration
}

// Response — результат выполнения шага.
type Response struct {
	// Outputs — выходные данные шага.
	// Доступны в следующих шагах через {{ .Steps.stepID.Outputs.field }}
	Outputs map[string]any
}

// NewRequest создаёт новый Request.
func NewRequest(stepID string, config map[string]any, tmplCtx *engine.Context, timeout time.Duration) *Request {
	if config == nil {
		config = make(map[string]any)
	}
	return &Request{
		StepID:          stepID,
		Config:          config,
		TemplateContext: tmplCtx,
		Timeout:         timeout,
	}
}

// NewResponse создаёт новый Response с outputs.
func NewResponse(outputs map[string]any) *Response {
	if outputs == nil {
		outputs = make(map[string]any)
	}
	return &Response{
		Outputs: outputs,
	}
}

// EmptyResponse возвращает пустой Response.
func EmptyResponse() *Response {
	return &Response{
		Outputs: make(map[string]any),
	}
}

// GetConfigString извлекает строковое значение из конфига.
func GetConfigString(config map[string]any, key string) string {
	if v, ok := config[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetConfigInt извлекает числовое значение из конфига.
func GetConfigInt(config map[string]any, key string) int {
	if v, ok := config[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

// GetConfigBool извлекает булево значение из конфига.
func GetConfigBool(config map[string]any, key string, defaultVal bool) bool {
	if v, ok := config[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetConfigMap извлекает map из конфига.
func GetConfigMap(config map[string]any, key string) map[string]any {
	if v, ok := config[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// GetConfigMapString извлекает map[string]string из конфига.
func GetConfigMapString(config map[string]any, key string) map[string]string {
	if v, ok := config[key]; ok {
		switch m := v.(type) {
		case map[string]string:
			return m
		case map[string]any:
			result := make(map[string]string)
			for k, val := range m {
				if s, ok := val.(string); ok {
					result[k] = s
				}
			}
			return result
		}
	}
	return nil
}
