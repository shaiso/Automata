// Package api содержит HTTP API сервер.
//
// Структура:
//   - handler.go          — Handler с DI (репозитории, publisher, logger)
//   - routes.go           — регистрация маршрутов
//   - middleware.go       — middleware (logging, recovery, CORS)
//   - response.go         — унифицированные JSON-ответы и обработка ошибок
//   - dto.go              — Data Transfer Objects (request/response)
//   - flow_handler.go     — обработчики для /flows
//   - run_handler.go      — обработчики для /runs
//   - schedule_handler.go — обработчики для /schedules
//   - proposal_handler.go — обработчики для /proposals (PR-workflow + sandbox)
//
// API предоставляет REST endpoints для управления flows, runs, schedules и proposals.
package api
