// Automata Orchestrator — управляет выполнением runs.
//
// Orchestrator:
//   - Получает новые runs из RabbitMQ
//   - Парсит flow spec и строит DAG
//   - Создаёт tasks и отправляет их workers
//   - Отслеживает прогресс и финализирует runs
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/shaiso/Automata/internal/telemetry"
)

func main() {
	// Инициализируем structured logging
	logger := telemetry.SetupLogger()
	logger.Info("starting automata-orchestrator")

	// Ожидаем сигнал завершения
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// TODO: инициализация компонентов
	// - Подключение к PostgreSQL
	// - Подключение к RabbitMQ
	// - Запуск consumer для runs.pending
	// - Запуск consumer для tasks.completed

	<-ctx.Done()
	logger.Info("shutting down automata-orchestrator")
}
