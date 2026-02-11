package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// Default configuration values.
const (
	defaultPollInterval = 10 * time.Second
	defaultBatchSize    = 100
)

// Orchestrator управляет выполнением runs.
//
// Orchestrator — центральный компонент системы, который:
//   - Получает новые runs из очереди RabbitMQ (event-driven)
//   - Периодически проверяет pending runs в БД (polling fallback)
//   - Строит DAG для каждого run
//   - Создаёт tasks для готовых шагов
//   - Отслеживает завершение tasks
//   - Финализирует runs (SUCCEEDED/FAILED)
type Orchestrator struct {
	// Repositories
	runRepo  *repo.RunRepo
	taskRepo *repo.TaskRepo
	flowRepo *repo.FlowRepo

	// MQ
	publisher *mq.Publisher
	conn      *mq.Connection

	// Active runs — runs в процессе выполнения (runID → state)
	activeRuns map[uuid.UUID]*RunState
	mu         sync.RWMutex

	// Consumers
	runConsumer  *mq.Consumer
	taskConsumer *mq.Consumer

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

// Config — конфигурация Orchestrator.
type Config struct {
	// Repositories
	RunRepo  *repo.RunRepo
	TaskRepo *repo.TaskRepo
	FlowRepo *repo.FlowRepo

	// MQ
	Publisher *mq.Publisher
	Conn      *mq.Connection

	// Polling configuration
	PollInterval time.Duration // интервал polling (default: 10s)
	BatchSize    int           // количество runs за один poll (default: 100)

	// Logger
	Logger *slog.Logger
}

// New создаёт новый Orchestrator.
func New(cfg Config) *Orchestrator {
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

	return &Orchestrator{
		runRepo:      cfg.RunRepo,
		taskRepo:     cfg.TaskRepo,
		flowRepo:     cfg.FlowRepo,
		publisher:    cfg.Publisher,
		conn:         cfg.Conn,
		activeRuns:   make(map[uuid.UUID]*RunState),
		pollInterval: pollInterval,
		batchSize:    batchSize,
		logger:       logger,
	}
}

// Start запускает Orchestrator.
//
// Запускает:
//   - Consumer для runs.pending
//   - Consumer для tasks.completed
//   - Polling горутину для fallback
func (o *Orchestrator) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	o.cancelFunc = cancel

	o.logger.Info("starting orchestrator",
		"poll_interval", o.pollInterval,
		"batch_size", o.batchSize,
	)

	// Создаём consumers
	o.runConsumer = mq.NewConsumer(o.conn, o.logger, mq.ConsumerConfig{
		Queue:    string(mq.QueueRunsPending),
		Handler:  o.handleRunPending,
		Prefetch: 10,
	})

	o.taskConsumer = mq.NewConsumer(o.conn, o.logger, mq.ConsumerConfig{
		Queue:    string(mq.QueueTasksCompleted),
		Handler:  o.handleTaskCompleted,
		Prefetch: 10,
	})

	// Запускаем run consumer
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		if err := o.runConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			o.logger.Error("run consumer error", "error", err)
		}
	}()

	// Запускаем task consumer
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		if err := o.taskConsumer.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			o.logger.Error("task consumer error", "error", err)
		}
	}()

	// Запускаем polling
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		o.pollLoop(ctx)
	}()

	o.logger.Info("orchestrator started")
	return nil
}

// Stop останавливает Orchestrator.
func (o *Orchestrator) Stop() {
	o.stoppedMu.Lock()
	o.stopped = true
	o.stoppedMu.Unlock()

	o.logger.Info("stopping orchestrator...")

	if o.cancelFunc != nil {
		o.cancelFunc()
	}

	// Останавливаем consumers
	if o.runConsumer != nil {
		o.runConsumer.Stop()
	}
	if o.taskConsumer != nil {
		o.taskConsumer.Stop()
	}

	// Ждём завершения горутин
	o.wg.Wait()

	o.logger.Info("orchestrator stopped",
		"active_runs", len(o.activeRuns),
	)
}

// IsStopped проверяет, остановлен ли Orchestrator.
func (o *Orchestrator) IsStopped() bool {
	o.stoppedMu.RLock()
	defer o.stoppedMu.RUnlock()
	return o.stopped
}

// pollLoop — цикл polling для fallback.
func (o *Orchestrator) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(o.pollInterval)
	defer ticker.Stop()

	// Первый poll сразу при старте (подхватываем runs созданные пока были выключены)
	o.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.poll(ctx)
		}
	}
}

// poll выполняет один цикл polling.
func (o *Orchestrator) poll(ctx context.Context) {
	runs, err := o.runRepo.ListPending(ctx, o.batchSize)
	if err != nil {
		o.logger.Error("failed to list pending runs", "error", err)
		return
	}

	if len(runs) == 0 {
		return
	}

	o.logger.Debug("poll found pending runs", "count", len(runs))

	for i := range runs {
		run := &runs[i]

		// Проверяем, не обрабатывается ли уже
		if o.isRunActive(run.ID) {
			continue
		}

		// Обрабатываем run
		if err := o.processRun(ctx, run.ID); err != nil {
			o.logger.Error("failed to process run from poll",
				"run_id", run.ID,
				"error", err,
			)
		}
	}
}

// isRunActive проверяет, находится ли run в обработке.
func (o *Orchestrator) isRunActive(runID uuid.UUID) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	_, exists := o.activeRuns[runID]
	return exists
}

// getActiveRun возвращает активный RunState.
func (o *Orchestrator) getActiveRun(runID uuid.UUID) *RunState {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.activeRuns[runID]
}

// addActiveRun добавляет run в активные.
func (o *Orchestrator) addActiveRun(state *RunState) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.activeRuns[state.RunID()]; exists {
		return ErrRunAlreadyActive
	}

	o.activeRuns[state.RunID()] = state
	return nil
}

// removeActiveRun удаляет run из активных.
func (o *Orchestrator) removeActiveRun(runID uuid.UUID) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.activeRuns, runID)
}

// ActiveRunsCount возвращает количество активных runs.
func (o *Orchestrator) ActiveRunsCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.activeRuns)
}

// GetActiveRunStats возвращает статистику по активному run.
func (o *Orchestrator) GetActiveRunStats(runID uuid.UUID) (RunStats, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	state, exists := o.activeRuns[runID]
	if !exists {
		return RunStats{}, false
	}

	return state.Stats(), true
}
