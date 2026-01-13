package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
	"github.com/shaiso/Automata/internal/scheduler"
	"github.com/shaiso/Automata/internal/telemetry"
)

const schedLockKey int64 = 424242

func main() {
	// Инициализируем structured logging
	logger := telemetry.SetupLogger()
	logger.Info("starting automata-scheduler")

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
	scheduleRepo := repo.NewScheduleRepo(pool)
	runRepo := repo.NewRunRepo(pool)
	flowRepo := repo.NewFlowRepo(pool)

	// RabbitMQ
	var publisher *mq.Publisher
	mqURL := os.Getenv("RABBITMQ_URL")
	if mqURL == "" {
		mqURL = "amqp://automata:automata@localhost:5672/"
	}

	mqConn, err := mq.NewConnection(mqURL, logger)
	if err != nil {
		logger.Warn("RabbitMQ not available, running without message publishing", "error", err)
	} else {
		defer mqConn.Close()
		logger.Info("RabbitMQ connected")

		// Создаём топологию
		if err := mq.SetupTopology(ctx, mqConn); err != nil {
			logger.Warn("failed to setup topology", "error", err)
		}

		publisher = mq.NewPublisher(mqConn, logger)
	}

	// Создаём scheduler
	sched := scheduler.New(scheduler.Config{
		ScheduleRepo: scheduleRepo,
		RunRepo:      runRepo,
		FlowRepo:     flowRepo,
		Publisher:    publisher,
		Logger:       logger,
		BatchSize:    100,
	})

	// HTTP mux: /healthz + /metrics
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// scheduler loop
	go func() {
		tk := time.NewTicker(1 * time.Second)
		defer tk.Stop()

		var hasLock bool
		defer func() {
			if hasLock {
				_, _ = pool.Exec(context.Background(), "select pg_advisory_unlock($1)", schedLockKey)
				logger.Info("released leader lock")
			}
		}()

		for {
			select {
			case <-tk.C:
				// пытаемся стать лидером (или подтвердить лидерство)
				var ok bool
				if !hasLock {
					if err := pool.QueryRow(ctx, "select pg_try_advisory_lock($1)", schedLockKey).Scan(&ok); err != nil {
						logger.Error("failed to acquire lock", "error", err)
						continue
					}
					if ok {
						hasLock = true
						logger.Info("acquired leader lock")
					}
				}

				if !hasLock {
					// не лидер — пропускаем тик
					continue
				}

				// лидер выполняет логику тика
				if err := sched.Tick(ctx); err != nil {
					logger.Error("scheduler tick failed", "error", err)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// serve
	port := ":8081"
	if v := os.Getenv("SCHED_PORT"); v != "" {
		port = ":" + v
	}

	logger.Info("listening", "addr", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		logger.Error("http server error", "error", err)
		cancel()
	}
}
