package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageType — тип сообщения в очереди.
type MessageType string

// Типы сообщений.
const (
	MessageTypeRunPending    MessageType = "run.pending"
	MessageTypeTaskReady     MessageType = "task.ready"
	MessageTypeTaskCompleted MessageType = "task.completed"
)

// Publisher публикует сообщения в RabbitMQ.
type Publisher struct {
	conn   *Connection
	logger *slog.Logger
}

// NewPublisher создаёт новый Publisher.
func NewPublisher(conn *Connection, logger *slog.Logger) *Publisher {
	return &Publisher{
		conn:   conn,
		logger: logger,
	}
}

// Message — сообщение для публикации.
type Message struct {
	// ID — уникальный идентификатор сообщения.
	ID string `json:"id"`

	// Type — тип сообщения.
	Type MessageType `json:"type"`

	// Payload — полезная нагрузка.
	Payload any `json:"payload"`

	// Timestamp — время создания.
	Timestamp time.Time `json:"timestamp"`
}

// RunPendingPayload — payload для сообщения о новом run.
type RunPendingPayload struct {
	RunID uuid.UUID `json:"run_id"`
}

// TaskReadyPayload — payload для сообщения о готовой задаче.
type TaskReadyPayload struct {
	TaskID uuid.UUID `json:"task_id"`
	RunID  uuid.UUID `json:"run_id"`
}

// TaskCompletedPayload — payload для сообщения о завершённой задаче.
type TaskCompletedPayload struct {
	TaskID  uuid.UUID `json:"task_id"`
	RunID   uuid.UUID `json:"run_id"`
	StepID  string    `json:"step_id"`
	Status  string    `json:"status"` // SUCCEEDED или FAILED
	Error   string    `json:"error,omitempty"`
	Attempt int       `json:"attempt"`
}

// Publish публикует сообщение в указанный exchange с routing key.
func (p *Publisher) Publish(ctx context.Context, exchange Exchange, routingKey RoutingKey, msg *Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return p.conn.WithChannel(ctx, func(ch *amqp.Channel) error {
		err := ch.PublishWithContext(
			ctx,
			string(exchange),   // exchange
			string(routingKey), // routing key
			false,
			false,
			amqp.Publishing{
				ContentType:  "application/json",
				DeliveryMode: amqp.Persistent, // сообщение переживёт рестарт RabbitMQ
				MessageId:    msg.ID,
				Timestamp:    msg.Timestamp,
				Body:         body,
			},
		)
		if err != nil {
			return fmt.Errorf("publish to %s/%s: %w", exchange, routingKey, err)
		}

		p.logger.Debug("published message",
			"exchange", exchange,
			"routing_key", routingKey,
			"message_id", msg.ID,
			"type", msg.Type,
		)

		return nil
	})
}

// PublishRunPending публикует событие о новом run, ожидающем выполнения.
// Потребитель: Orchestrator.
func (p *Publisher) PublishRunPending(ctx context.Context, runID uuid.UUID) error {
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeRunPending,
		Payload:   RunPendingPayload{RunID: runID},
		Timestamp: time.Now(),
	}

	return p.Publish(ctx, ExchangeRuns, RoutingKeyPending, msg)
}

// PublishTaskReady публикует событие о задаче, готовой к выполнению.
// Потребитель: Worker.
func (p *Publisher) PublishTaskReady(ctx context.Context, taskID, runID uuid.UUID) error {
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeTaskReady,
		Payload:   TaskReadyPayload{TaskID: taskID, RunID: runID},
		Timestamp: time.Now(),
	}

	return p.Publish(ctx, ExchangeTasks, RoutingKeyReady, msg)
}

// PublishTaskCompleted публикует событие о завершённой задаче.
// Потребитель: Orchestrator.
func (p *Publisher) PublishTaskCompleted(ctx context.Context, payload TaskCompletedPayload) error {
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      MessageTypeTaskCompleted,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	return p.Publish(ctx, ExchangeTasks, RoutingKeyCompleted, msg)
}

// PublishJSON публикует произвольный JSON payload.
func (p *Publisher) PublishJSON(ctx context.Context, exchange Exchange, routingKey RoutingKey, msgType MessageType, payload any) error {
	msg := &Message{
		ID:        uuid.New().String(),
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	return p.Publish(ctx, exchange, routingKey, msg)
}
