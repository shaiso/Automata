package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shaiso/Automata/internal/api"
	"github.com/shaiso/Automata/internal/repo"
	"github.com/shaiso/Automata/internal/telemetry"
)

var (
	startTime = time.Now()
	reqTotal  = promauto.NewCounter(prometheus.CounterOpts{
		Name: "automata_api_http_requests_total",
		Help: "Total HTTP requests handled by automata_api",
	})
)

func main() {
	// Инициализируем structured logging
	logger := telemetry.SetupLogger()
	logger.Info("starting automata-api")

	// Подключаемся к базе данных
	pool, err := repo.NewPool(context.Background())
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("connected to database")

	// Создаём репозитории
	flowRepo := repo.NewFlowRepo(pool)
	runRepo := repo.NewRunRepo(pool)
	taskRepo := repo.NewTaskRepo(pool)
	scheduleRepo := repo.NewScheduleRepo(pool)

	// Создаём API handler
	handler := api.NewHandler(api.Config{
		FlowRepo:     flowRepo,
		RunRepo:      runRepo,
		TaskRepo:     taskRepo,
		ScheduleRepo: scheduleRepo,
		Publisher:    nil, // Подключим позже, когда будет RabbitMQ
		Logger:       logger,
	})

	mux := http.NewServeMux()

	// Health и metrics
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		reqTotal.Inc()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok %s", time.Since(startTime))
	})
	mux.Handle("/metrics", promhttp.Handler())

	// Регистрируем API маршруты
	handler.RegisterRoutes(mux)

	addr := ":8080"
	if v := os.Getenv("API_PORT"); v != "" {
		addr = ":" + v
	}

	// Создаём HTTP сервер с возможностью graceful shutdown
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Запускаем сервер в горутине
	go func() {
		logger.Info("listening", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Ожидаем сигнал завершения
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	<-ctx.Done()
	logger.Info("shutting down")

	// Graceful shutdown с таймаутом 10 секунд
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("stopped")
}
