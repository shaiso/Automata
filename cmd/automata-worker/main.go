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
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
	"github.com/shaiso/Automata/internal/telemetry"
	"github.com/shaiso/Automata/internal/worker"
)

func main() {
	// Инициализируем structured logging
	logger := telemetry.SetupLogger()
	logger.Info("starting automata-worker")

	// graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// DB pool
	pool, err := repo.NewPool(ctx)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("database connected")

	// Создаём репозитории
	taskRepo := repo.NewTaskRepo(pool)
	runRepo := repo.NewRunRepo(pool)
	flowRepo := repo.NewFlowRepo(pool)

	// RabbitMQ
	var publisher *mq.Publisher
	var mqConn *mq.Connection
	mqURL := os.Getenv("RABBITMQ_URL")
	if mqURL == "" {
		mqURL = "amqp://automata:automata@localhost:5672/"
	}

	mqConn, err = mq.NewConnection(mqURL, logger)
	if err != nil {
		logger.Warn("RabbitMQ not available, running in polling-only mode", "error", err)
	} else {
		defer mqConn.Close()
		logger.Info("RabbitMQ connected")

		// Создаём топологию
		if err := mq.SetupTopology(ctx, mqConn); err != nil {
			logger.Warn("failed to setup topology", "error", err)
		}

		publisher = mq.NewPublisher(mqConn, logger)
	}

	// Создаём worker
	w := worker.New(worker.Config{
		TaskRepo:  taskRepo,
		RunRepo:   runRepo,
		FlowRepo:  flowRepo,
		Publisher: publisher,
		Conn:      mqConn,
		Logger:    logger,
	})

	// Запускаем worker
	if err := w.Start(ctx); err != nil {
		logger.Error("failed to start worker", "error", err)
		os.Exit(1)
	}

	// HTTP mux: /healthz + /metrics
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	port := ":8082"
	if v := os.Getenv("WORKER_PORT"); v != "" {
		port = ":" + v
	}

	go func() {
		logger.Info("listening", "addr", port)
		if err := http.ListenAndServe(port, mux); err != nil {
			logger.Error("http server error", "error", err)
			cancel()
		}
	}()

	// Ожидаем сигнал завершения
	<-ctx.Done()

	// Останавливаем worker
	w.Stop()
	logger.Info("automata-worker stopped")
}
