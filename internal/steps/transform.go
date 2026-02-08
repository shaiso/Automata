package steps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shaiso/Automata/internal/engine"
)

const (
	// StepTypeTransform — тип шага трансформации.
	StepTypeTransform = "transform"

	// Ключ конфигурации.
	configMappings = "mappings"
)

// TransformStep — шаг трансформации данных.
//
// Применяет Go templates для преобразования данных из предыдущих шагов.
//
// Конфигурация:
//
//	{
//	    "mappings": {
//	        "total": "{{ len .Steps.fetch.Outputs.items }}",
//	        "first_item": "{{ index .Steps.fetch.Outputs.items 0 }}",
//	        "ids": "{{ range .Steps.fetch.Outputs.items }}{{ .id }},{{ end }}"
//	    }
//	}
//
// Outputs: результаты рендеринга mappings
//
//	{
//	    "total": "10",
//	    "first_item": "{...}",
//	    "ids": "1,2,3,4,5,"
//	}
type TransformStep struct{}

// NewTransformStep создаёт новый TransformStep.
func NewTransformStep() *TransformStep {
	return &TransformStep{}
}

// Type возвращает тип шага.
func (s *TransformStep) Type() string {
	return StepTypeTransform
}

// Execute выполняет трансформацию данных.
func (s *TransformStep) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Проверяем context
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrStepCancelled, ctx.Err())
	default:
	}

	// Получаем mappings
	mappings := s.parseMappings(req.Config)
	if len(mappings) == 0 {
		// Нет mappings — возвращаем пустой результат
		return EmptyResponse(), nil
	}

	// Проверяем, что есть контекст для шаблонов
	tmplCtx := req.TemplateContext
	if tmplCtx == nil {
		tmplCtx = engine.NewContext(nil)
	}

	// Рендерим каждый mapping
	outputs := make(map[string]any, len(mappings))
	for key, tmpl := range mappings {
		rendered, err := engine.Render(tmpl, tmplCtx)
		if err != nil {
			return nil, fmt.Errorf("transform %s: %w", key, err)
		}

		// Пробуем распарсить результат как JSON
		outputs[key] = s.parseValue(rendered)
	}

	return &Response{Outputs: outputs}, nil
}

// parseMappings извлекает mappings из конфигурации.
func (s *TransformStep) parseMappings(config map[string]any) map[string]string {
	raw := config[configMappings]
	if raw == nil {
		return nil
	}

	switch m := raw.(type) {
	case map[string]string:
		return m

	case map[string]any:
		result := make(map[string]string, len(m))
		for key, val := range m {
			if str, ok := val.(string); ok {
				result[key] = str
			}
		}
		return result

	default:
		return nil
	}
}

// parseValue пытается распарсить строку как JSON.
// Если не получается — возвращает строку как есть.
func (s *TransformStep) parseValue(value string) any {
	// Пробуем как JSON object
	var obj map[string]any
	if err := json.Unmarshal([]byte(value), &obj); err == nil {
		return obj
	}

	// Пробуем как JSON array
	var arr []any
	if err := json.Unmarshal([]byte(value), &arr); err == nil {
		return arr
	}

	// Пробуем как JSON number
	var num json.Number
	if err := json.Unmarshal([]byte(value), &num); err == nil {
		// Пробуем как int
		if i, err := num.Int64(); err == nil {
			return i
		}
		// Иначе как float
		if f, err := num.Float64(); err == nil {
			return f
		}
	}

	// Пробуем как JSON bool
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}

	// Возвращаем как строку
	return value
}
