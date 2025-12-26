package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler — функция обработки сообщения.
// Возвращает error, если обработка не удалась (сообщение будет nack).
type Handler func(ctx context.Context, msg *Delivery) error

// Delivery — доставленное сообщение с методами ack/nack.
type Delivery struct {
	// Message — распарсенное сообщение.
	Message Message

	// Raw — сырое AMQP сообщение.
	Raw amqp.Delivery
}

// Ack подтверждает успешную обработку сообщения.
func (d *Delivery) Ack() error {
	return d.Raw.Ack(false)
}

// Nack отклоняет сообщение.
// requeue=true — вернуть в очередь, false — отправить в DLQ.
func (d *Delivery) Nack(requeue bool) error {
	return d.Raw.Nack(false, requeue)
}

// Consumer потребляет сообщения из очереди RabbitMQ.
type Consumer struct {
	conn     *Connection
	logger   *slog.Logger
	queue    string
	handler  Handler
	prefetch int

	cancelFunc context.CancelFunc
}

// ConsumerConfig — конфигурация consumer.
type ConsumerConfig struct {
	// Queue — имя очереди.
	Queue string

	// Handler — обработчик сообщений.
	Handler Handler

	// Prefetch — количество сообщений для предварительной загрузки.
	Prefetch int
}

// NewConsumer создаёт новый Consumer.
func NewConsumer(conn *Connection, logger *slog.Logger, cfg ConsumerConfig) *Consumer {
	prefetch := cfg.Prefetch
	if prefetch <= 0 {
		prefetch = 1
	}

	return &Consumer{
		conn:     conn,
		logger:   logger,
		queue:    cfg.Queue,
		handler:  cfg.Handler,
		prefetch: prefetch,
	}
}

// Start запускает потребление сообщений.
func (c *Consumer) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	c.cancelFunc = cancel

	// Запускаем основной цикл потребления
	return c.consume(ctx)
}

// consume — основной цикл потребления.
func (c *Consumer) consume(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Получаем канал доставки
		deliveries, err := c.setupConsume()
		if err != nil {
			c.logger.Error("failed to setup consume", "queue", c.queue, "error", err)
			// Ждём переподключения
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-c.conn.ReconnectNotify():
				c.logger.Info("reconnected, restarting consumer", "queue", c.queue)
				continue
			}
		}

		c.logger.Info("consumer started", "queue", c.queue)

		// Обрабатываем сообщения
		if err := c.processDeliveries(ctx, deliveries); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Warn("deliveries channel closed, reconnecting", "queue", c.queue)
			// Канал закрыт, ждём переподключения
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-c.conn.ReconnectNotify():
				continue
			}
		}
	}
}

// setupConsume настраивает канал и начинает потребление.
func (c *Consumer) setupConsume() (<-chan amqp.Delivery, error) {
	ch := c.conn.Channel()
	if ch == nil {
		return nil, fmt.Errorf("no channel available")
	}

	// Устанавливаем prefetch
	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("set qos: %w", err)
	}

	// Начинаем потребление
	deliveries, err := ch.Consume(
		c.queue, // queue
		"",      // consumer tag (auto-generated)
		false,   // auto-ack (мы ack вручную)
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return nil, fmt.Errorf("consume: %w", err)
	}

	return deliveries, nil
}

// processDeliveries обрабатывает сообщения из канала.
func (c *Consumer) processDeliveries(ctx context.Context, deliveries <-chan amqp.Delivery) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case raw, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("deliveries channel closed")
			}

			c.handleDelivery(ctx, raw)
		}
	}
}

// handleDelivery обрабатывает одно сообщение.
func (c *Consumer) handleDelivery(ctx context.Context, raw amqp.Delivery) {
	// Парсим сообщение
	var msg Message
	if err := json.Unmarshal(raw.Body, &msg); err != nil {
		c.logger.Error("failed to unmarshal message",
			"queue", c.queue,
			"error", err,
			"body", string(raw.Body),
		)
		// Некорректное сообщение — отправляем в DLQ
		raw.Nack(false, false)
		return
	}

	delivery := &Delivery{
		Message: msg,
		Raw:     raw,
	}

	c.logger.Debug("received message",
		"queue", c.queue,
		"message_id", msg.ID,
		"type", msg.Type,
	)

	// Вызываем обработчик
	if err := c.handler(ctx, delivery); err != nil {
		c.logger.Error("handler failed",
			"queue", c.queue,
			"message_id", msg.ID,
			"type", msg.Type,
			"error", err,
		)
		// Ошибка обработки — возвращаем в очередь для retry
		// (если retry исчерпаны, DLQ настроен на уровне очереди)
		raw.Nack(false, true)
		return
	}

	// Успешно обработано
	raw.Ack(false)
}

// Stop останавливает consumer.
func (c *Consumer) Stop() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
}

// ParsePayload парсит payload сообщения в указанный тип.
func ParsePayload[T any](msg *Message) (T, error) {
	var result T

	// Payload может быть уже распарсен как map или быть raw json
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return result, fmt.Errorf("marshal payload: %w", err)
	}

	if err := json.Unmarshal(payloadBytes, &result); err != nil {
		return result, fmt.Errorf("unmarshal payload: %w", err)
	}

	return result, nil
}
