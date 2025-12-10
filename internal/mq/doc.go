// Package mq предоставляет интеграцию с RabbitMQ.
//
// Включает:
//   - connection.go — управление подключением с auto-reconnect
//   - publisher.go  — публикация сообщений в exchange
//   - consumer.go   — потребление сообщений из очередей
//   - topology.go   — декларация exchanges и queues
//   - messages.go   — типы сообщений (RunCreated, TaskReady, etc.)
package mq
