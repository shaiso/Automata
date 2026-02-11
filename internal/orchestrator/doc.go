// Package orchestrator управляет выполнением runs.
//
// # Обзор
//
// Orchestrator — центральный компонент системы Automata, который координирует
// выполнение рабочих процессов (flows). Он отвечает за:
//
//   - Получение новых runs из очереди RabbitMQ (event-driven)
//   - Периодическую проверку pending runs в БД (polling fallback)
//   - Валидацию FlowSpec и построение DAG для каждого run
//   - Создание tasks для готовых к выполнению шагов
//   - Отслеживание завершения tasks через RabbitMQ
//   - Финализацию runs (SUCCEEDED/FAILED/CANCELLED)
//
// # Архитектура
//
// Orchestrator использует гибридный подход к доставке событий:
//
//	┌──────────────────── Orchestrator ────────────────────┐
//	│                                                      │
//	│  Входы:                                              │
//	│    [runs.pending] ──► handleRunPending──┐            │
//	│    [tasks.done]   ──► handleTaskDone  ──┤            │
//	│    polling (10s)  ──► poll            ──┘            │
//	│                                         │            │
//	│                                         ▼            │
//	│                                    processRun        │
//	│                                    processTask       │
//	│                                         │            │
//	│                                         ▼            │
//	│                              ┌── activeRuns ──┐      │
//	│                              │                │      │
//	│                          RunState #1    RunState #N  │
//	│                          (DAG, ctx)     (DAG, ctx)   │
//	│                                                      │
//	└──────────────────────────────────────────────────────┘
//
// # Ключевые компоненты
//
// ## Orchestrator
//
// Основная структура, управляющая жизненным циклом выполнения.
// Создаётся через New(cfg Config) и запускается методом Start(ctx).
//
//	orch := orchestrator.New(orchestrator.Config{
//	    RunRepo:   runRepo,
//	    TaskRepo:  taskRepo,
//	    FlowRepo:  flowRepo,
//	    Publisher: publisher,
//	    Conn:      mqConn,
//	    Logger:    logger,
//	})
//
//	if err := orch.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer orch.Stop()
//
// ## RunState
//
// Состояние выполнения одного run в памяти. Содержит:
//   - Run — данные из БД
//   - FlowVersion — версия flow с FlowSpec
//   - DAG — построенный граф зависимостей
//   - Context — контекст для рендеринга шаблонов
//   - Статусы шагов: completed, running, failed
//
// RunState создаётся при начале обработки run и удаляется при завершении.
// После рестарта Orchestrator восстанавливает состояние из БД.
//
// # Обработка событий
//
// ## run.pending
//
// Когда Scheduler создаёт новый run, он публикует событие run.pending.
// Orchestrator:
//  1. Загружает run из БД
//  2. Проверяет статус (должен быть PENDING)
//  3. Загружает FlowVersion
//  4. Создаёт RunState и инициализирует его (валидация, DAG, контекст)
//  5. Добавляет в activeRuns
//  6. Переводит run в RUNNING
//  7. Запускает готовые шаги (без зависимостей)
//
// ## task.completed
//
// Когда Worker завершает task, он публикует событие task.completed.
// Orchestrator:
//  1. Находит или восстанавливает RunState
//  2. Обновляет статус шага (completed/failed)
//  3. Добавляет outputs в Context
//  4. Проверяет завершение run (все шаги выполнены?)
//  5. Если run завершён — финализирует, иначе — запускает следующие готовые шаги
//
// # Polling Fallback
//
// Polling нужен для надёжности:
//   - При рестарте Orchestrator может пропустить события
//   - RabbitMQ может потерять сообщения при сбое
//
// Каждые N секунд (по умолчанию 10) Orchestrator:
//  1. Запрашивает pending runs из БД
//  2. Для каждого run, который не в activeRuns — запускает обработку
//
// # Восстановление после рестарта
//
// Если task.completed приходит для run, которого нет в памяти:
//  1. Загружаем run из БД (если не RUNNING — игнорируем)
//  2. Загружаем FlowVersion
//  3. Создаём RunState и инициализируем
//  4. Загружаем все tasks для этого run
//  5. Восстанавливаем состояние из tasks (RestoreFromTasks)
//  6. Добавляем в activeRuns
//  7. Продолжаем обработку
//
// # Потокобезопасность
//
// Orchestrator использует sync.RWMutex для защиты activeRuns.
// RunState использует свой sync.RWMutex для защиты внутреннего состояния.
// Это позволяет безопасно обрабатывать несколько событий параллельно.
//
// # Ошибки
//
// Пакет определяет типизированные ошибки:
//   - ErrRunNotFound — run не найден в БД
//   - ErrRunNotPending — run не в статусе PENDING
//   - ErrRunAlreadyActive — run уже обрабатывается
//   - ErrInvalidFlowSpec — FlowSpec не прошёл валидацию
//   - ErrTaskNotFound — task не найден
//   - ErrStepNotFound — шаг не найден в DAG
//
// # Пример flow выполнения
//
// Для flow с шагами: fetch → process → notify
//
//  1. run.pending → processRun()
//     - Создаём RunState, строим DAG
//     - GetReadySteps() → [fetch] (нет зависимостей)
//     - Создаём task для fetch, публикуем task.ready
//
//  2. task.completed (fetch, SUCCEEDED)
//     - MarkStepCompleted("fetch", outputs)
//     - GetReadySteps() → [process] (fetch завершён)
//     - Создаём task для process
//
//  3. task.completed (process, SUCCEEDED)
//     - MarkStepCompleted("process", outputs)
//     - GetReadySteps() → [notify]
//     - Создаём task для notify
//
//  4. task.completed (notify, SUCCEEDED)
//     - MarkStepCompleted("notify", outputs)
//     - IsComplete() → true
//     - completeRun(state, success=true)
//     - Run.Status = SUCCEEDED
package orchestrator
