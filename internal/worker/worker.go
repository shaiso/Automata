package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// Default configuration values.
const (
	defaultPollInterval = 10 * time.Second
	defaultBatchSize    = 50
	defaultPrefetch     = 5
)

// Worker выполняет отдельные tasks.
//
// Worker — stateless компонент системы, который:
//   - Получает tasks из очереди RabbitMQ (event-driven)
//   - Периодически проверяет queued tasks в БД (polling fallback)
//   - Выполняет task в зависимости от типа шага (http, delay, transform)
//   - Реализует retry с exponential backoff
//   - Отправляет результат обратно в очередь tasks.completed
//
// Workers масштабируются горизонтально — несколько экземпляров
// могут потреблять из одной очереди.
type Worker struct {
	// Repositories
	taskRepo *repo.TaskRepo
	runRepo  *repo.RunRepo
	flowRepo *repo.FlowRepo

	// MQ
	publisher *mq.Publisher
	conn      *mq.Connection

	// Executor registry
	registry *Registry

	// Consumer
	consumer *mq.Consumer

	// Configuration
	pollInterval time.Duration
	batchSize    int

	// Lifecycle
	logger     *slog.Logger
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	stopped    bool
	stoppedMu  sync.RWMutex
}

// Config — конфигурация Worker.
type Config struct {
	// Repositories
	TaskRepo *repo.TaskRepo
	RunRepo  *repo.RunRepo
	FlowRepo *repo.FlowRepo

	// MQ
	Publisher *mq.Publisher
	Conn      *mq.Connection

	// Executor registry (опционально; если nil — используется NewRegistry())
	Registry *Registry

	// Polling configuration
	PollInterval time.Duration // интервал polling (default: 10s)
	BatchSize    int           // количество tasks за один poll (default: 50)

	// Logger
	Logger *slog.Logger
}

// New создаёт новый Worker.
func New(cfg Config) *Worker {
	pollInterval := cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	registry := cfg.Registry
	if registry == nil {
		registry = NewRegistry()
	}

	return &Worker{
		taskRepo:     cfg.TaskRepo,
		runRepo:      cfg.RunRepo,
		flowRepo:     cfg.FlowRepo,
		publisher:    cfg.Publisher,
		conn:         cfg.Conn,
		registry:     registry,
		pollInterval: pollInterval,
		batchSize:    batchSize,
		logger:       logger,
	}
}

// Start запускает Worker.
//
// Запускает:
//   - Consumer для tasks.ready
//   - Polling горутину для fallback
func (w *Worker) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel

	w.logger.Info("starting worker",
		"poll_interval", w.pollInterval,
		"batch_size", w.batchSize,
	)

	// Создаём consumer
	w.consumer = mq.NewConsumer(w.conn, w.logger, mq.ConsumerConfig{
		Queue:    string(mq.QueueTasksReady),
		Handler:  w.handleTaskReady,
		Prefetch: defaultPrefetch,
	})

	// Запускаем consumer
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.consumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("task consumer error", "error", err)
		}
	}()

	// Запускаем polling
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.pollLoop(ctx)
	}()

	w.logger.Info("worker started")
	return nil
}

// Stop останавливает Worker.
func (w *Worker) Stop() {
	w.stoppedMu.Lock()
	w.stopped = true
	w.stoppedMu.Unlock()

	w.logger.Info("stopping worker...")

	if w.cancelFunc != nil {
		w.cancelFunc()
	}

	if w.consumer != nil {
		w.consumer.Stop()
	}

	// Ждём завершения горутин
	w.wg.Wait()

	w.logger.Info("worker stopped")
}

// IsStopped проверяет, остановлен ли Worker.
func (w *Worker) IsStopped() bool {
	w.stoppedMu.RLock()
	defer w.stoppedMu.RUnlock()
	return w.stopped
}

// pollLoop — цикл polling для fallback.
func (w *Worker) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	// Первый poll сразу при старте (подхватываем tasks созданные пока были выключены)
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

// poll выполняет один цикл polling.
func (w *Worker) poll(ctx context.Context) {
	tasks, err := w.taskRepo.ListQueued(ctx, w.batchSize)
	if err != nil {
		w.logger.Error("failed to list queued tasks", "error", err)
		return
	}

	if len(tasks) == 0 {
		return
	}

	w.logger.Debug("poll found queued tasks", "count", len(tasks))

	for i := range tasks {
		task := &tasks[i]

		if err := w.processTask(ctx, task.ID); err != nil {
			w.logger.Error("failed to process task from poll",
				"task_id", task.ID,
				"error", err,
			)
		}
	}
}
