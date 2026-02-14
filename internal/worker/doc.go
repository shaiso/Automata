// Package worker выполняет отдельные tasks.
//
// # Обзор
//
// Worker — stateless компонент системы Automata, который выполняет
// отдельные задачи (tasks), созданные Orchestrator'ом. Worker отвечает за:
//
//   - Получение tasks из очереди RabbitMQ (event-driven)
//   - Периодическую проверку queued tasks в БД (polling fallback)
//   - Выполнение task в зависимости от типа шага (http, delay, transform)
//   - Retry с exponential backoff при ошибках
//   - Отправку результата обратно в очередь tasks.completed
//
// Workers масштабируются горизонтально — несколько экземпляров
// потребляют из одной очереди tasks.ready.
//
// # Ключевые компоненты
//
// ## Worker
//
// Основная структура, управляющая жизненным циклом.
// Создаётся через New(cfg Config) и запускается методом Start(ctx).
//
//	w := worker.New(worker.Config{
//	    TaskRepo:  taskRepo,
//	    RunRepo:   runRepo,
//	    FlowRepo:  flowRepo,
//	    Publisher: publisher,
//	    Conn:      mqConn,
//	    Logger:    logger,
//	})
//
//	if err := w.Start(ctx); err != nil {
//	    log.Fatal(err)
//	}
//	defer w.Stop()
//
// ## Executor
//
// Интерфейс для выполнения конкретного типа шага:
//
//	type Executor interface {
//	    Execute(ctx context.Context, task *domain.Task) (*ExecutionResult, error)
//	}
//
// Реализации:
//   - HTTPExecutor — HTTP-запросы (GET/POST/PUT/DELETE, headers, body, timeout)
//   - DelayExecutor — задержка на указанное количество секунд
//   - TransformExecutor — трансформация данных (pass-through отрендеренного payload)
//
// ## Registry
//
// Реестр executor'ов по типу шага. NewRegistry() создаёт реестр
// с предустановленными executor'ами (http, delay, transform).
//
// # Обработка task
//
//  1. Получение task (из очереди или polling)
//  2. Загрузка task из БД, проверка статуса QUEUED
//  3. Перевод в RUNNING, инкремент Attempt
//  4. Загрузка RetryPolicy из FlowVersion
//  5. Выполнение через executeWithRetry
//  6. Успех → MarkSucceeded, publish TaskCompleted(SUCCEEDED)
//  7. Ошибка → MarkFailed, publish TaskCompleted(FAILED)
//
// # Retry
//
// Retry выполняется в процессе (in-process), а не через requeue в RabbitMQ.
// Это даёт точный контроль над backoff и подсчётом попыток.
//
// Стратегии backoff:
//   - "exponential": delay = initialDelay * 2^(attempt-1), capped at maxDelay
//   - "fixed": delay = initialDelay
//
// Для HTTP-шагов можно указать OnStatus — список HTTP-кодов, при которых retry.
//
// # Ошибки
//
// Пакет различает два уровня ошибок:
//   - Инфраструктурные (error от Execute) — сеть упала, DNS не резолвится
//   - Логические (ExecutionResult.Error) — HTTP 500, валидация не прошла
//
// Инфраструктурные всегда retriable. Логические — зависят от OnStatus.
package worker
