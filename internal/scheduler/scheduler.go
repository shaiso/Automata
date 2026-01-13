package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// Scheduler — планировщик, обрабатывающий due schedules.
type Scheduler struct {
	scheduleRepo *repo.ScheduleRepo
	runRepo      *repo.RunRepo
	flowRepo     *repo.FlowRepo
	publisher    *mq.Publisher
	logger       *slog.Logger
	batchSize    int
}

// Config — конфигурация Scheduler.
type Config struct {
	ScheduleRepo *repo.ScheduleRepo
	RunRepo      *repo.RunRepo
	FlowRepo     *repo.FlowRepo
	Publisher    *mq.Publisher
	Logger       *slog.Logger
	BatchSize    int // количество schedules за один тик (default: 100)
}

// New создаёт новый Scheduler.
func New(cfg Config) *Scheduler {
	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	return &Scheduler{
		scheduleRepo: cfg.ScheduleRepo,
		runRepo:      cfg.RunRepo,
		flowRepo:     cfg.FlowRepo,
		publisher:    cfg.Publisher,
		logger:       cfg.Logger,
		batchSize:    batchSize,
	}
}

// Tick выполняет один тик планировщика.
//
// 1. Находит due schedules (enabled=true, next_due_at <= now)
// 2. Для каждого schedule создаёт run
// 3. Обновляет next_due_at
// 4. Публикует run.pending в RabbitMQ
//
// Ошибки одного schedule не блокируют обработку остальных.
func (s *Scheduler) Tick(ctx context.Context) error {
	now := time.Now()

	// 1. Находим due schedules
	schedules, err := s.scheduleRepo.ListDue(ctx, now, s.batchSize)
	if err != nil {
		return fmt.Errorf("list due schedules: %w", err)
	}

	if len(schedules) == 0 {
		return nil
	}

	s.logger.Debug("found due schedules", "count", len(schedules))

	// 2. Обрабатываем каждый schedule
	var processed, created int
	for i := range schedules {
		sched := &schedules[i]

		runCreated, err := s.processSchedule(ctx, sched, now)
		if err != nil {
			s.logger.Error("failed to process schedule",
				"schedule_id", sched.ID,
				"schedule_name", sched.Name,
				"error", err,
			)
			// Продолжаем обработку остальных
			continue
		}

		processed++
		if runCreated {
			created++
		}
	}

	s.logger.Info("scheduler tick completed",
		"due", len(schedules),
		"processed", processed,
		"runs_created", created,
	)

	return nil
}

// processSchedule обрабатывает один schedule.
// Возвращает true, если run был создан (не был дубликатом).
func (s *Scheduler) processSchedule(ctx context.Context, sched *domain.Schedule, now time.Time) (bool, error) {
	// 1. Проверяем, что flow существует и имеет версии
	version, err := s.flowRepo.GetLatestVersion(ctx, sched.FlowID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			s.logger.Warn("flow not found for schedule, skipping",
				"schedule_id", sched.ID,
				"flow_id", sched.FlowID,
			)
			// Не возвращаем ошибку — просто пропускаем
			return false, nil
		}
		return false, fmt.Errorf("get latest flow version: %w", err)
	}

	// 2. Формируем idempotency key: "{schedule_id}_{next_due_at_unix}"
	// Это гарантирует, что для одного schedule и конкретного времени
	// будет создан только один run
	idempKey := fmt.Sprintf("%s_%d", sched.ID, sched.NextDueAt.Unix())

	// 3. Проверяем, не создан ли уже run (idempotency)
	existingRun, err := s.runRepo.GetByIdempotencyKey(ctx, sched.FlowID, idempKey)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return false, fmt.Errorf("check idempotency: %w", err)
	}

	var runCreated bool
	var runID uuid.UUID

	if existingRun != nil {
		// Run уже существует — просто обновляем next_due_at
		s.logger.Debug("run already exists (idempotency)",
			"schedule_id", sched.ID,
			"run_id", existingRun.ID,
			"idempotency_key", idempKey,
		)
		runID = existingRun.ID
		runCreated = false
	} else {
		// 4. Создаём новый run
		run := &domain.Run{
			ID:             uuid.New(),
			FlowID:         sched.FlowID,
			Version:        version.Version,
			Status:         domain.RunStatusPending,
			Inputs:         sched.Inputs,
			IdempotencyKey: idempKey,
			IsSandbox:      false,
			CreatedAt:      now,
		}

		if err := s.runRepo.Create(ctx, run); err != nil {
			return false, fmt.Errorf("create run: %w", err)
		}

		s.logger.Info("created run from schedule",
			"run_id", run.ID,
			"schedule_id", sched.ID,
			"schedule_name", sched.Name,
			"flow_id", sched.FlowID,
			"version", version.Version,
		)

		runID = run.ID
		runCreated = true
	}

	// 5. Вычисляем следующее время выполнения
	nextDue, err := CalculateNextDue(sched, now)
	if err != nil {
		s.logger.Error("failed to calculate next due, disabling schedule",
			"schedule_id", sched.ID,
			"error", err,
		)
		// Schedule некорректный — лучше не трогать next_due_at
		return runCreated, nil
	}

	// 6. Обновляем schedule
	sched.RecordRun(runID, nextDue)
	if err := s.scheduleRepo.Update(ctx, sched); err != nil {
		return runCreated, fmt.Errorf("update schedule: %w", err)
	}

	// 7. Публикуем событие в RabbitMQ (если publisher настроен и run создан)
	if s.publisher != nil && runCreated {
		if err := s.publisher.PublishRunPending(ctx, runID); err != nil {
			// Не фатальная ошибка — run уже создан в БД
			// Orchestrator может забрать его через polling
			s.logger.Warn("failed to publish run.pending",
				"run_id", runID,
				"error", err,
			)
		}
	}

	return runCreated, nil
}
