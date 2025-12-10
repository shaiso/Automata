// Package api содержит HTTP API сервер.
//
// Структура:
//   - server.go     — настройка HTTP сервера
//   - routes.go     — регистрация маршрутов
//   - handlers/     — обработчики запросов
//   - middleware/   — middleware (logging, recovery, auth)
//   - dto/          — Data Transfer Objects (request/response)
//
// API предоставляет REST endpoints для управления flows, runs,
// schedules и proposals.
package api
