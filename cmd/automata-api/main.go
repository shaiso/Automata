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

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		reqTotal.Inc()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok %s", time.Since(startTime))
	})

	mux.Handle("/metrics", promhttp.Handler())

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
