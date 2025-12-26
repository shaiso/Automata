// Package mq предоставляет инфраструктуру для работы с RabbitMQ.
//
// Структура:
//   - connection.go — управление соединением с RabbitMQ (reconnect, graceful shutdown)
//   - topology.go   — объявление exchanges, queues, bindings
//   - publisher.go  — публикация сообщений в очереди
//   - consumer.go   — потребление сообщений из очередей
//
// Типы сообщений:
//   - run.pending      — новый run ожидает выполнения
//   - task.ready       — задача готова к выполнению
//   - task.completed   — задача завершена
//
// Exchanges:
//   - automata.runs    — события runs
//   - automata.tasks   — события tasks
//   - automata.dlq     — dead letter queue
package mq
