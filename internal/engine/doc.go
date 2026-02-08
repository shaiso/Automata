// Package engine предоставляет инструменты для работы с FlowSpec:
// валидацию, построение DAG и рендеринг Go templates.
//
// # Обзор
//
// Engine — это "мозг" системы, который:
//   - Валидирует FlowSpec перед выполнением
//   - Строит DAG (направленный ациклический граф) зависимостей
//   - Определяет порядок выполнения шагов
//   - Рендерит динамические данные через Go templates
//
// # Основные компоненты
//
// ## Валидация (parser.go)
//
// Функция Validate проверяет корректность FlowSpec:
//
//	err := engine.Validate(spec)
//	if err != nil {
//	    // spec невалиден
//	}
//
// Проверки:
//   - Steps не пустой
//   - Уникальные ID шагов
//   - Известные типы шагов (http, delay, transform, parallel)
//   - Все depends_on ссылаются на существующие шаги
//   - Нет self-dependency
//   - Для parallel: валидные branches
//
// ## DAG (dag.go)
//
// BuildDAG создаёт граф зависимостей и проверяет на циклы:
//
//	dag, err := engine.BuildDAG(spec)
//	if errors.Is(err, engine.ErrCyclicDependency) {
//	    // циклическая зависимость
//	}
//
// GetReadyNodes возвращает шаги, готовые к выполнению:
//
//	completed := map[string]bool{"step1": true}
//	running := map[string]bool{"step2": true}
//	ready := dag.GetReadyNodes(completed, running)
//
// Для parallel шагов DAG автоматически:
//   - Создаёт prefixed ID для шагов веток: parallel.branch_a.step1
//   - Добавляет виртуальный join узел для синхронизации
//
// ## Templates (template.go)
//
// Context хранит данные для рендеринга шаблонов:
//
//	ctx := engine.NewContext(run.Inputs)
//	ctx.AddStepResult("fetch", outputs, "SUCCEEDED")
//
// Render выполняет Go template:
//
//	result, err := engine.Render("Hello, {{ .Inputs.name }}", ctx)
//
// RenderConfig рендерит весь конфиг шага рекурсивно:
//
//	config, err := engine.RenderConfig(step.Config, ctx)
//
// Доступные данные в шаблонах:
//   - {{ .Inputs.xxx }} — входные параметры run
//   - {{ .Steps.stepID.Outputs.xxx }} — outputs предыдущих шагов
//   - {{ .Steps.stepID.Status }} — статус шага (SUCCEEDED, FAILED)
//
// # Использование в Orchestrator
//
// Типичный flow работы:
//
//	// 1. Валидация
//	if err := engine.Validate(&spec); err != nil {
//	    return err
//	}
//
//	// 2. Построение DAG
//	dag, err := engine.BuildDAG(&spec)
//	if err != nil {
//	    return err
//	}
//
//	// 3. Создание контекста
//	ctx := engine.NewContext(run.Inputs)
//
//	// 4. Цикл выполнения
//	for !dag.IsComplete(completed) {
//	    ready := dag.GetReadyNodes(completed, running)
//	    for _, node := range ready {
//	        config, _ := engine.RenderConfig(node.Step.Config, ctx)
//	        // создать и выполнить task
//	    }
//	    // после выполнения task
//	    ctx.AddStepResult(stepID, outputs, status)
//	    completed[stepID] = true
//	}
//
// # Файлы пакета
//
//   - errors.go   — определения ошибок (ErrEmptySteps, ErrCyclicDependency, etc.)
//   - parser.go   — валидация FlowSpec
//   - dag.go      — DAG структура и алгоритмы
//   - template.go — рендеринг Go templates
package engine
