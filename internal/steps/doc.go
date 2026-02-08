// Package steps содержит реализации типов шагов для выполнения в workflow.
//
// # Обзор
//
// Steps — это исполнители конкретных типов шагов. Каждый шаг:
//   - Получает конфигурацию (уже отрендеренную через engine.RenderConfig)
//   - Выполняет действие (HTTP запрос, задержка, трансформация)
//   - Возвращает outputs для использования в следующих шагах
//
// # Интерфейс Step
//
// Все шаги реализуют интерфейс Step:
//
//	type Step interface {
//	    Type() string
//	    Execute(ctx context.Context, req *Request) (*Response, error)
//	}
//
// Request содержит:
//   - StepID — идентификатор шага
//   - Config — конфигурация (map[string]any)
//   - TemplateContext — контекст для дополнительного рендеринга
//   - Timeout — таймаут выполнения
//
// Response содержит:
//   - Outputs — результаты выполнения (map[string]any)
//
// # Registry
//
// Registry — фабрика для получения Step по типу:
//
//	registry := steps.DefaultRegistry()  // http, delay, transform, parallel
//	step, err := registry.Get("http")
//	if err != nil {
//	    // неизвестный тип
//	}
//
// # Типы шагов
//
// ## HTTP (http.go)
//
// Выполняет HTTP запросы к внешним API.
//
// Конфигурация:
//
//	{
//	    "method": "POST",
//	    "url": "https://api.example.com/data",
//	    "headers": {"Authorization": "Bearer xxx"},
//	    "body": {"key": "value"},
//	    "follow_redirects": true,
//	    "validate_ssl": true,
//	    "timeout_sec": 30
//	}
//
// Outputs:
//
//	{
//	    "status_code": 200,
//	    "headers": {"Content-Type": "application/json"},
//	    "body": {...}  // parsed JSON или string
//	}
//
// ## Delay (delay.go)
//
// Пауза между шагами.
//
// Конфигурация:
//
//	{"duration_sec": 5}   // или
//	{"duration_ms": 500}
//
// Outputs:
//
//	{"duration_ms": 5000}
//
// ## Transform (transform.go)
//
// Трансформация данных через Go templates.
//
// Конфигурация:
//
//	{
//	    "mappings": {
//	        "total": "{{ len .Steps.fetch.Outputs.items }}",
//	        "first_id": "{{ index .Steps.fetch.Outputs.items 0 }}"
//	    }
//	}
//
// Outputs — результаты рендеринга каждого mapping:
//
//	{"total": 10, "first_id": "abc123"}
//
// ## Parallel (parallel.go)
//
// Параллельное выполнение веток.
//
// ВАЖНО: ParallelStep сам не выполняет вложенные шаги.
// Orchestrator "разворачивает" parallel в DAG и выполняет
// шаги веток как отдельные tasks.
//
// Конфигурация задаётся через domain.StepDef.Branches:
//
//	{
//	    "id": "parallel_step",
//	    "type": "parallel",
//	    "branches": [
//	        {"id": "branch_a", "steps": [...]},
//	        {"id": "branch_b", "steps": [...]}
//	    ]
//	}
//
// Outputs (собираются через AggregateParallelOutputs):
//
//	{
//	    "branch_a": {"step1": {...}, "step2": {...}},
//	    "branch_b": {"step1": {...}}
//	}
//
// Вспомогательные функции:
//   - AggregateParallelOutputs — собирает outputs веток
//   - ExtractBranchOutputs — извлекает outputs ветки
//   - ExtractStepOutputs — извлекает outputs шага из ветки
//
// # Использование
//
// Типичный flow в Worker:
//
//	// 1. Получить Step из Registry
//	registry := steps.DefaultRegistry()
//	step, err := registry.Get(task.Type)
//
//	// 2. Подготовить Request
//	req := &steps.Request{
//	    StepID:  task.StepID,
//	    Config:  task.Payload,
//	    Timeout: 30 * time.Second,
//	}
//
//	// 3. Выполнить с context
//	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
//	defer cancel()
//
//	resp, err := step.Execute(ctx, req)
//	if err != nil {
//	    // обработка ошибки
//	}
//
//	// 4. Использовать outputs
//	task.Outputs = resp.Outputs
//
// # Обработка ошибок
//
// Шаги возвращают типизированные ошибки:
//
//	var (
//	    ErrStepCancelled   // context cancelled
//	    ErrInvalidConfig   // неверная конфигурация
//	    ErrHTTPRequest     // ошибка HTTP запроса
//	    ErrHTTPStatus      // HTTP статус >= 400
//	)
//
// Retry логика находится в Orchestrator, шаги просто возвращают ошибки.
//
// # Файлы пакета
//
//   - step.go      — интерфейс Step, Request, Response, ошибки
//   - registry.go  — Registry для получения Step по типу
//   - http.go      — HTTPStep
//   - delay.go     — DelayStep
//   - transform.go — TransformStep
//   - parallel.go  — ParallelStep и helper функции
package steps