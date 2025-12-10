// Package scheduler содержит логику планировщика задач.
//
// Scheduler отвечает за:
//   - Leader election через PostgreSQL advisory locks
//   - Опрос таблицы schedules на предмет due задач
//   - Создание runs для готовых к выполнению schedules
//   - Вычисление next_due_at (cron или interval)
//
// Только один экземпляр scheduler может быть лидером в один момент времени.
package scheduler
