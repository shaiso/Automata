// Automata Worker — выполняет отдельные tasks.
//
// Worker:
//   - Получает tasks из RabbitMQ
//   - Выполняет в зависимости от типа (http, delay, transform)
//   - Реализует retry с exponential backoff
//   - Отправляет результат обратно
//
// Workers масштабируются горизонтально.
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
	logger.Info("starting automata-worker")

	// Ожидаем сигнал завершения
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// TODO: инициализация компонентов
	// - Подключение к PostgreSQL
	// - Подключение к RabbitMQ
	// - Регистрация step executors
	// - Запуск consumer для tasks.ready

	<-ctx.Done()
	logger.Info("shutting down automata-worker")
}
