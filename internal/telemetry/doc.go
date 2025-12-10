// Package telemetry обеспечивает наблюдаемость системы.
//
// Включает:
//   - logging.go — structured logging через slog
//   - metrics.go — Prometheus метрики
//
// Все сервисы используют единый формат логирования
// и экспортируют метрики на /metrics endpoint.
package telemetry
