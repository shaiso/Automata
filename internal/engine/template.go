package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// Context — контекст для рендеринга шаблонов.
//
// Используется в Go templates для доступа к данным:
//   - {{ .Inputs.param_name }}
//   - {{ .Steps.step_id.Outputs.field }}
//   - {{ .Env.VAR_NAME }}
type Context struct {
	// Inputs — входные параметры run.
	Inputs map[string]any `json:"inputs"`

	// Steps — результаты выполненных шагов.
	Steps map[string]*StepContext `json:"steps"`

	// Env — переменные окружения.
	Env map[string]string `json:"env"`
}

// StepContext — результат выполнения шага для использования в шаблонах.
type StepContext struct {
	// Outputs — выходные данные шага.
	Outputs map[string]any `json:"outputs"`

	// Status — статус выполнения: "SUCCEEDED", "FAILED".
	Status string `json:"status"`
}

// NewContext создаёт новый контекст с входными параметрами.
func NewContext(inputs map[string]any) *Context {
	if inputs == nil {
		inputs = make(map[string]any)
	}
	return &Context{
		Inputs: inputs,
		Steps:  make(map[string]*StepContext),
		Env:    make(map[string]string),
	}
}

// AddStepResult добавляет результат выполнения шага в контекст.
func (c *Context) AddStepResult(stepID string, outputs map[string]any, status string) {
	if outputs == nil {
		outputs = make(map[string]any)
	}
	c.Steps[stepID] = &StepContext{
		Outputs: outputs,
		Status:  status,
	}
}

// SetEnv устанавливает переменную окружения.
func (c *Context) SetEnv(key, value string) {
	c.Env[key] = value
}

// templateFuncs — дополнительные функции для шаблонов.
var templateFuncs = template.FuncMap{
	// json — сериализует значение в JSON строку
	"json": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return string(b)
	},

	// default — возвращает значение по умолчанию, если первый аргумент пустой
	"default": func(def, val any) any {
		if val == nil {
			return def
		}
		if s, ok := val.(string); ok && s == "" {
			return def
		}
		return val
	},

	// coalesce — возвращает первое непустое значение
	"coalesce": func(values ...any) any {
		for _, v := range values {
			if v != nil {
				if s, ok := v.(string); ok && s == "" {
					continue
				}
				return v
			}
		}
		return nil
	},

	// toJSON — алиас для json
	"toJSON": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	},

	// fromJSON — парсит JSON строку
	"fromJSON": func(s string) any {
		var result any
		if err := json.Unmarshal([]byte(s), &result); err != nil {
			return nil
		}
		return result
	},

	// join — объединяет слайс строк
	"join": func(sep string, items []string) string {
		return strings.Join(items, sep)
	},

	// split — разбивает строку на слайс
	"split": func(sep, s string) []string {
		return strings.Split(s, sep)
	},

	// contains — проверяет, содержит ли строка подстроку
	"contains": strings.Contains,

	// hasPrefix — проверяет префикс строки
	"hasPrefix": strings.HasPrefix,

	// hasSuffix — проверяет суффикс строки
	"hasSuffix": strings.HasSuffix,

	// lower — приводит к нижнему регистру
	"lower": strings.ToLower,

	// upper — приводит к верхнему регистру
	"upper": strings.ToUpper,

	// trim — удаляет пробелы по краям
	"trim": strings.TrimSpace,

	// replace — заменяет подстроку
	"replace": strings.ReplaceAll,
}

// Render рендерит строковый шаблон с контекстом.
//
// Шаблон может содержать Go template выражения:
//
//	{{ .Inputs.param }}
//	{{ .Steps.fetch.Outputs.data }}
//	{{ if .Steps.validate.Outputs.is_valid }}...{{ end }}
func Render(tmpl string, ctx *Context) (string, error) {
	// Проверяем, содержит ли строка шаблонные выражения
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}

	t, err := template.New("").Funcs(templateFuncs).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateParse, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTemplateRender, err)
	}

	return buf.String(), nil
}

// RenderValue рендерит произвольное значение.
// Рекурсивно обрабатывает map и slice.
func RenderValue(value any, ctx *Context) (any, error) {
	if value == nil {
		return nil, nil
	}

	switch v := value.(type) {
	case string:
		return Render(v, ctx)

	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			rendered, err := RenderValue(val, ctx)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
		return result, nil

	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			rendered, err := RenderValue(val, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil

	case map[string]string:
		result := make(map[string]string, len(v))
		for key, val := range v {
			rendered, err := Render(val, ctx)
			if err != nil {
				return nil, err
			}
			result[key] = rendered
		}
		return result, nil

	case []string:
		result := make([]string, len(v))
		for i, val := range v {
			rendered, err := Render(val, ctx)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		}
		return result, nil

	default:
		// Для остальных типов (int, float, bool) возвращаем как есть
		return value, nil
	}
}

// RenderConfig рендерит конфигурацию шага.
// Это обёртка над RenderValue для map[string]any.
func RenderConfig(config map[string]any, ctx *Context) (map[string]any, error) {
	if config == nil {
		return make(map[string]any), nil
	}

	rendered, err := RenderValue(config, ctx)
	if err != nil {
		return nil, err
	}

	result, ok := rendered.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: expected map, got %T", ErrTemplateRender, rendered)
	}

	return result, nil
}

// RenderCondition рендерит и вычисляет условие.
// Возвращает true, если условие выполняется.
func RenderCondition(condition string, ctx *Context) (bool, error) {
	if condition == "" {
		return true, nil
	}

	// Оборачиваем условие в if, чтобы получить bool
	tmpl := fmt.Sprintf(`{{if %s}}true{{else}}false{{end}}`, condition)

	result, err := Render(tmpl, ctx)
	if err != nil {
		return false, err
	}

	return result == "true", nil
}

// MustRender рендерит шаблон и паникует при ошибке.
// Используется только для тестов.
func MustRender(tmpl string, ctx *Context) string {
	result, err := Render(tmpl, ctx)
	if err != nil {
		panic(err)
	}
	return result
}
