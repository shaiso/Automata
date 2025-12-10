package domain

import (
	"time"

	"github.com/google/uuid"
)

// Flow — определение рабочего процесса.
//
// Flow — это "шаблон" или "рецепт" автоматизации.
// Один flow может иметь множество версий (FlowVersion).
// Каждый запуск (Run) выполняет конкретную версию flow.
type Flow struct {
	// ID — уникальный идентификатор flow.
	ID uuid.UUID `json:"id"`

	// Name — уникальное имя flow (например, "sync-orders", "daily-report").
	// Используется для удобной идентификации пользователем.
	Name string `json:"name"`

	// IsActive — флаг активности. Неактивные flows не запускаются по расписанию.
	IsActive bool `json:"is_active"`

	// CreatedAt — время создания flow.
	CreatedAt time.Time `json:"created_at"`
}

// FlowVersion — версия flow с конкретной спецификацией.
//
// Версионирование позволяет:
// - Отслеживать историю изменений
// - Откатываться к предыдущим версиям
// - Запускать старые версии для сравнения
type FlowVersion struct {
	// FlowID — ссылка на родительский flow.
	FlowID uuid.UUID `json:"flow_id"`

	// Version — номер версии (1, 2, 3, ...).
	// Автоинкремент при создании новой версии.
	Version int `json:"version"`

	// Spec — спецификация flow в формате JSON.
	// Содержит шаги, зависимости, настройки retry и т.д.
	Spec FlowSpec `json:"spec"`

	// CreatedAt — время создания версии.
	CreatedAt time.Time `json:"created_at"`
}

// FlowSpec — спецификация flow (содержимое JSONB поля spec).
//
// Это "программа" для Automata — описание того, что нужно выполнить.
type FlowSpec struct {
	// Version — версия формата спецификации (для обратной совместимости).
	Version string `json:"version,omitempty"`

	// Name — имя flow (дублирует Flow.Name для удобства).
	Name string `json:"name,omitempty"`

	// Description — описание назначения flow.
	Description string `json:"description,omitempty"`

	// Inputs — входные параметры flow.
	// Ключ — имя параметра, значение — его определение.
	Inputs map[string]InputDef `json:"inputs,omitempty"`

	// Defaults — настройки по умолчанию для всех шагов.
	Defaults *StepDefaults `json:"defaults,omitempty"`

	// Steps — список шагов для выполнения.
	Steps []StepDef `json:"steps"`

	// OnFailure — обработчик ошибок (выполняется при падении flow).
	OnFailure *StepDef `json:"on_failure,omitempty"`
}

// InputDef — определение входного параметра.
type InputDef struct {
	// Type — тип параметра: "string", "number", "boolean", "object".
	Type string `json:"type"`

	// Required — обязательный ли параметр.
	Required bool `json:"required,omitempty"`

	// Default — значение по умолчанию.
	Default any `json:"default,omitempty"`

	// Description — описание параметра.
	Description string `json:"description,omitempty"`
}

// StepDefaults — настройки по умолчанию для шагов.
type StepDefaults struct {
	// Retry — политика повторных попыток.
	Retry *RetryPolicy `json:"retry,omitempty"`

	// TimeoutSec — таймаут выполнения в секундах.
	TimeoutSec int `json:"timeout_sec,omitempty"`
}

// StepDef — определение шага в flow.
type StepDef struct {
	// ID — уникальный идентификатор шага в рамках flow.
	// Используется в depends_on и для ссылок на результаты.
	ID string `json:"id"`

	// Name — человекочитаемое имя шага.
	Name string `json:"name,omitempty"`

	// Type — тип шага: "http", "delay", "transform", "parallel".
	Type string `json:"type"`

	// DependsOn — список ID шагов, от которых зависит этот шаг.
	// Шаг начнёт выполнение только после успешного завершения всех зависимостей.
	DependsOn []string `json:"depends_on,omitempty"`

	// Condition — условие выполнения (Go template, возвращающий bool).
	// Например: "{{ .steps.validate.outputs.is_valid == true }}"
	Condition string `json:"condition,omitempty"`

	// Config — конфигурация шага (зависит от типа).
	// Для http: method, url, headers, body
	// Для delay: duration_sec
	Config map[string]any `json:"config,omitempty"`

	// Outputs — маппинг результатов шага для использования в следующих шагах.
	// Ключ — имя output, значение — Go template для извлечения.
	Outputs map[string]string `json:"outputs,omitempty"`

	// Retry — политика повторных попыток для этого шага.
	// Переопределяет defaults.retry.
	Retry *RetryPolicy `json:"retry,omitempty"`

	// TimeoutSec — таймаут для этого шага.
	// Переопределяет defaults.timeout_sec.
	TimeoutSec int `json:"timeout_sec,omitempty"`

	// Branches — ветки для параллельного выполнения (только для type="parallel").
	Branches []Branch `json:"branches,omitempty"`
}

// RetryPolicy — политика повторных попыток.
type RetryPolicy struct {
	// MaxAttempts — максимальное количество попыток (включая первую).
	MaxAttempts int `json:"max_attempts,omitempty"`

	// Backoff — стратегия задержки: "fixed", "exponential".
	Backoff string `json:"backoff,omitempty"`

	// InitialDelayMs — начальная задержка в миллисекундах.
	InitialDelayMs int `json:"initial_delay_ms,omitempty"`

	// MaxDelayMs — максимальная задержка в миллисекундах.
	MaxDelayMs int `json:"max_delay_ms,omitempty"`

	// OnStatus — HTTP статусы, при которых делать retry (для http шагов).
	OnStatus []int `json:"on_status,omitempty"`
}

// Branch — ветка параллельного выполнения.
type Branch struct {
	// ID — идентификатор ветки.
	ID string `json:"id"`

	// Steps — шаги внутри ветки.
	Steps []StepDef `json:"steps"`
}
