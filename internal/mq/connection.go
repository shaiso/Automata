package mq

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Connection — обёртка над AMQP соединением с автоматическим reconnect.
//
// Особенности:
// - Автоматическое переподключение при разрыве
// - Потокобезопасный доступ к каналам
// - Graceful shutdown
type Connection struct {
	url    string
	logger *slog.Logger

	mu      sync.RWMutex
	conn    *amqp.Connection
	channel *amqp.Channel

	closed   bool
	closedCh chan struct{}

	// Для уведомления о переподключении
	reconnectCh chan struct{}
}

// NewConnection создаёт новое соединение с RabbitMQ.
func NewConnection(url string, logger *slog.Logger) (*Connection, error) {
	c := &Connection{
		url:         url,
		logger:      logger,
		closedCh:    make(chan struct{}),
		reconnectCh: make(chan struct{}, 1),
	}

	if err := c.connect(); err != nil {
		return nil, err
	}

	// Запускаем горутину для мониторинга соединения
	go c.watchConnection()

	return c, nil
}

// connect устанавливает соединение и открывает канал.
func (c *Connection) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("dial amqp: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	c.conn = conn
	c.channel = ch

	c.logger.Info("connected to RabbitMQ")

	return nil
}

// watchConnection следит за соединением и переподключается при разрыве.
func (c *Connection) watchConnection() {
	for {
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			return
		}
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			time.Sleep(time.Second)
			continue
		}

		// Ждём уведомления о закрытии соединения
		notifyClose := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-c.closedCh:
			return
		case err := <-notifyClose:
			if err != nil {
				c.logger.Warn("connection closed", "error", err)
			}

			// Переподключаемся с экспоненциальной задержкой
			c.reconnect()
		}
	}
}

// reconnect пытается переподключиться с экспоненциальной задержкой.
func (c *Connection) reconnect() {
	delay := time.Second

	for {
		c.mu.RLock()
		if c.closed {
			c.mu.RUnlock()
			return
		}
		c.mu.RUnlock()

		c.logger.Info("attempting to reconnect", "delay", delay)
		time.Sleep(delay)

		if err := c.connect(); err != nil {
			c.logger.Warn("reconnect failed", "error", err)
			// Увеличиваем задержку (максимум 30 секунд)
			delay = min(delay*2, 30*time.Second)
			continue
		}

		c.logger.Info("reconnected to RabbitMQ")

		// Уведомляем о переподключении
		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}

		return
	}
}

// Channel возвращает текущий AMQP канал.
func (c *Connection) Channel() *amqp.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channel
}

// ReconnectNotify возвращает канал для уведомлений о переподключении.
func (c *Connection) ReconnectNotify() <-chan struct{} {
	return c.reconnectCh
}

// Close закрывает соединение.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.closedCh)

	var errs []error

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close channel: %w", err))
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close connection: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	c.logger.Info("connection closed")
	return nil
}

// IsConnected проверяет, установлено ли соединение.
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return false
	}

	return !c.conn.IsClosed()
}

// WithChannel выполняет функцию с текущим каналом.
func (c *Connection) WithChannel(ctx context.Context, fn func(ch *amqp.Channel) error) error {
	c.mu.RLock()
	ch := c.channel
	c.mu.RUnlock()

	if ch == nil {
		return fmt.Errorf("no channel available")
	}

	return fn(ch)
}

// DefaultURL возвращает URL по умолчанию для локальной разработки.
func DefaultURL() string {
	return "amqp://automata:automata@localhost:5672/"
}
