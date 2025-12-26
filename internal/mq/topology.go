package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Exchange — тип для имени обменника.
type Exchange string

// Queue — тип для имени очереди.
type Queue string

// RoutingKey — тип для ключа маршрутизации.
type RoutingKey string

// Exchanges — имена обменников.
const (
	ExchangeRuns  Exchange = "automata.runs"
	ExchangeTasks Exchange = "automata.tasks"
	ExchangeDLQ   Exchange = "automata.dlq"
)

// Queues — имена очередей.
const (
	QueueRunsPending    Queue = "runs.pending"
	QueueTasksReady     Queue = "tasks.ready"
	QueueTasksCompleted Queue = "tasks.completed"
	QueueDLQTasks       Queue = "dlq.tasks"
)

// Routing keys.
const (
	RoutingKeyPending   RoutingKey = "pending"
	RoutingKeyReady     RoutingKey = "ready"
	RoutingKeyCompleted RoutingKey = "completed"
	RoutingKeyDLQTasks  RoutingKey = "tasks"
)

func SetupTopology(ctx context.Context, conn *Connection) error {
	return conn.WithChannel(ctx, func(ch *amqp.Channel) error {
		// 1. Создаём exchanges
		if err := declareExchanges(ch); err != nil {
			return err
		}

		// 2. Создаём queues
		if err := declareQueues(ch); err != nil {
			return err
		}

		// 3. Привязываем queues к exchanges
		if err := bindQueues(ch); err != nil {
			return err
		}

		return nil
	})
}

// declareExchanges создаёт обменники.
func declareExchanges(ch *amqp.Channel) error {
	exchanges := []struct {
		name Exchange
		kind string
	}{
		{ExchangeRuns, "direct"},
		{ExchangeTasks, "direct"},
		{ExchangeDLQ, "direct"},
	}

	for _, ex := range exchanges {
		err := ch.ExchangeDeclare(
			string(ex.name), // name
			ex.kind,         // type
			true,            // durable
			false,           // auto-deleted
			false,           // internal
			false,           // no-wait
			nil,             // arguments
		)
		if err != nil {
			return fmt.Errorf("declare exchange %s: %w", ex.name, err)
		}
	}

	return nil
}

// declareQueues создаёт очереди.
func declareQueues(ch *amqp.Channel) error {
	// Аргументы для очередей с DLQ
	dlqArgs := amqp.Table{
		"x-dead-letter-exchange":    string(ExchangeDLQ),
		"x-dead-letter-routing-key": string(RoutingKeyDLQTasks),
	}

	queues := []struct {
		name Queue
		args amqp.Table
	}{
		// runs.pending — без DLQ (runs обрабатываются один раз)
		{QueueRunsPending, nil},

		// tasks.ready — с DLQ (задачи могут уходить в DLQ после retry)
		{QueueTasksReady, dlqArgs},

		// tasks.completed — без DLQ (события завершения)
		{QueueTasksCompleted, nil},

		// dlq.tasks — сама DLQ очередь
		{QueueDLQTasks, nil},
	}

	for _, q := range queues {
		_, err := ch.QueueDeclare(
			string(q.name), // name
			true,           // durable
			false,          // delete when unused
			false,          // exclusive
			false,          // no-wait
			q.args,         // arguments
		)
		if err != nil {
			return fmt.Errorf("declare queue %s: %w", q.name, err)
		}
	}

	return nil
}

// bindQueues привязывает очереди к обменникам.
func bindQueues(ch *amqp.Channel) error {
	bindings := []struct {
		queue      Queue
		routingKey RoutingKey
		exchange   Exchange
	}{
		{QueueRunsPending, RoutingKeyPending, ExchangeRuns},
		{QueueTasksReady, RoutingKeyReady, ExchangeTasks},
		{QueueTasksCompleted, RoutingKeyCompleted, ExchangeTasks},
		{QueueDLQTasks, RoutingKeyDLQTasks, ExchangeDLQ},
	}

	for _, b := range bindings {
		err := ch.QueueBind(
			string(b.queue),      // queue name
			string(b.routingKey), // routing key
			string(b.exchange),   // exchange
			false,                // no-wait
			nil,                  // arguments
		)
		if err != nil {
			return fmt.Errorf("bind queue %s to %s: %w", b.queue, b.exchange, err)
		}
	}

	return nil
}

// TopologyInfo возвращает описание топологии для логирования.
func TopologyInfo() string {
	return `                                                                                    
  Automata RabbitMQ Topology:                                                                       
                                                                                                    
    automata.runs (direct)                                                                          
    └── runs.pending [routing: pending]                                                             
            Consumer: Orchestrator                                                                  
                                                                                                    
    automata.tasks (direct)                                                                         
    ├── tasks.ready [routing: ready]                                                                
    │       Consumer: Worker                                                                        
    │       DLQ: dlq.tasks                                                                          
    └── tasks.completed [routing: completed]                                                        
            Consumer: Orchestrator                                                                  
                                                                                                    
    automata.dlq (direct)                                                                           
    └── dlq.tasks [routing: tasks]                                                                  
            Manual processing                                                                       
  `
}
