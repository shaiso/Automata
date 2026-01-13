// Package scheduler реализует логику планировщика задач.
//
// Scheduler периодически проверяет schedules с истекшим next_due_at
// и создаёт новые runs для выполнения.
//
// Структура:
//   - scheduler.go — основная логика Scheduler (Tick, processSchedule)
//   - cron.go      — парсинг cron-выражений и вычисление следующего времени
//
// Использование:
//
//	sched := scheduler.New(scheduler.Config{
//	    ScheduleRepo: scheduleRepo,
//	    RunRepo:      runRepo,
//	    FlowRepo:     flowRepo,
//	    Publisher:    publisher,  // опционально
//	    Logger:       logger,
//	})
//
//	// Вызывается каждый тик (обычно раз в секунду)
//	if err := sched.Tick(ctx); err != nil {
//	    logger.Error("scheduler tick failed", "error", err)
//	}
//
// Leader Election:
//
// Scheduler не реализует leader election самостоятельно.
// Это делается в main.go через pg_try_advisory_lock.
// Метод Tick() вызывается только лидером.
package scheduler
