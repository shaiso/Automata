package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/engine"
	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// handleRunPending обрабатывает событие о новом pending run.
func (o *Orchestrator) handleRunPending(ctx context.Context, delivery *mq.Delivery) error {
	// Парсим payload
	payload, err := mq.ParsePayload[mq.RunPendingPayload](&delivery.Message)
	if err != nil {
		o.logger.Error("failed to parse run.pending payload", "error", err)
		return err
	}

	o.logger.Debug("received run.pending event", "run_id", payload.RunID)

	// Проверяем, не обрабатывается ли уже
	if o.isRunActive(payload.RunID) {
		o.logger.Debug("run already active, skipping", "run_id", payload.RunID)
		return nil
	}

	// Обрабатываем run
	if err := o.processRun(ctx, payload.RunID); err != nil {
		// Логируем, но не возвращаем ошибку для некоторых случаев
		if errors.Is(err, ErrRunNotPending) || errors.Is(err, ErrRunAlreadyActive) {
			o.logger.Debug("run not processed", "run_id", payload.RunID, "reason", err)
			return nil
		}
		o.logger.Error("failed to process run", "run_id", payload.RunID, "error", err)
		return err
	}

	return nil
}

// handleTaskCompleted обрабатывает событие о завершённом task.
func (o *Orchestrator) handleTaskCompleted(ctx context.Context, delivery *mq.Delivery) error {
	// Парсим payload
	payload, err := mq.ParsePayload[mq.TaskCompletedPayload](&delivery.Message)
	if err != nil {
		o.logger.Error("failed to parse task.completed payload", "error", err)
		return err
	}

	o.logger.Debug("received task.completed event",
		"task_id", payload.TaskID,
		"run_id", payload.RunID,
		"step_id", payload.StepID,
		"status", payload.Status,
	)

	// Обрабатываем завершение task
	if err := o.processTaskCompleted(ctx, payload); err != nil {
		o.logger.Error("failed to process task completion",
			"task_id", payload.TaskID,
			"run_id", payload.RunID,
			"error", err,
		)
		return err
	}

	return nil
}

// processRun обрабатывает новый run.
func (o *Orchestrator) processRun(ctx context.Context, runID uuid.UUID) error {
	// 1. Загружаем run из БД
	run, err := o.runRepo.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrRunNotFound, runID)
		}
		return fmt.Errorf("get run: %w", err)
	}

	// 2. Проверяем статус
	if run.Status != domain.RunStatusPending {
		return ErrRunNotPending
	}

	// 3. Загружаем FlowVersion
	version, err := o.flowRepo.GetVersion(ctx, run.FlowID, run.Version)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return o.failRun(ctx, run, fmt.Sprintf("flow version not found: %s v%d", run.FlowID, run.Version))
		}
		return fmt.Errorf("get flow version: %w", err)
	}

	// 4. Создаём RunState
	state := NewRunState(run, version)

	// 5. Инициализируем (валидация, DAG, контекст)
	if err := state.Initialize(); err != nil {
		return o.failRun(ctx, run, fmt.Sprintf("initialization failed: %v", err))
	}

	// 6. Добавляем в активные runs
	if err := o.addActiveRun(state); err != nil {
		return err
	}

	// 7. Переводим run в RUNNING
	run.MarkRunning()
	if err := o.runRepo.Update(ctx, run); err != nil {
		o.removeActiveRun(runID)
		return fmt.Errorf("update run to running: %w", err)
	}

	o.logger.Info("run started",
		"run_id", runID,
		"flow_id", run.FlowID,
		"version", run.Version,
		"steps", state.DAG.Size(),
	)

	// 8. Запускаем готовые шаги
	if err := o.dispatchReadySteps(ctx, state); err != nil {
		o.logger.Error("failed to dispatch initial steps", "run_id", runID, "error", err)
		// Не удаляем из активных — попробуем при следующем событии
	}

	return nil
}

// processTaskCompleted обрабатывает завершение task.
func (o *Orchestrator) processTaskCompleted(ctx context.Context, payload mq.TaskCompletedPayload) error {
	// 1. Получаем активный RunState
	state := o.getActiveRun(payload.RunID)

	// Если run не в памяти, пытаемся восстановить
	if state == nil {
		var err error
		state, err = o.restoreRunState(ctx, payload.RunID)
		if err != nil {
			return fmt.Errorf("restore run state: %w", err)
		}
		if state == nil {
			// Run уже завершён или не существует
			o.logger.Debug("run not active and cannot restore", "run_id", payload.RunID)
			return nil
		}
	}

	// 2. Загружаем task из БД (для получения актуальных outputs)
	task, err := o.taskRepo.GetByID(ctx, payload.TaskID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, payload.TaskID)
		}
		return fmt.Errorf("get task: %w", err)
	}

	// 3. Обновляем состояние шага
	stepID := payload.StepID

	if payload.Status == string(domain.TaskStatusSucceeded) {
		state.MarkStepCompleted(stepID, task.Outputs)
		o.logger.Debug("step completed",
			"run_id", payload.RunID,
			"step_id", stepID,
		)
	} else {
		// Task failed
		// TODO: retry logic (Фаза 7)
		state.MarkStepFailed(stepID, payload.Error)
		o.logger.Warn("step failed",
			"run_id", payload.RunID,
			"step_id", stepID,
			"error", payload.Error,
		)
	}

	// 4. Проверяем завершение run
	if state.HasFailed() {
		// Если есть упавший шаг — завершаем run с ошибкой
		return o.completeRun(ctx, state, false)
	}

	if state.IsComplete() {
		// Все шаги завершены успешно
		return o.completeRun(ctx, state, true)
	}

	// 5. Запускаем следующие готовые шаги
	return o.dispatchReadySteps(ctx, state)
}

// dispatchReadySteps создаёт tasks для готовых шагов и публикует их.
func (o *Orchestrator) dispatchReadySteps(ctx context.Context, state *RunState) error {
	readySteps := state.GetReadySteps()

	if len(readySteps) == 0 {
		return nil
	}

	o.logger.Debug("dispatching ready steps",
		"run_id", state.RunID(),
		"count", len(readySteps),
	)

	for _, node := range readySteps {
		if err := o.dispatchStep(ctx, state, node); err != nil {
			o.logger.Error("failed to dispatch step",
				"run_id", state.RunID(),
				"step_id", node.ID,
				"error", err,
			)
			// Продолжаем с другими шагами
		}
	}

	return nil
}

// dispatchStep создаёт task для шага и публикует его.
func (o *Orchestrator) dispatchStep(ctx context.Context, state *RunState, node *engine.Node) error {
	// Пропускаем join-узлы (виртуальные)
	if node.IsJoin {
		state.MarkStepCompleted(node.ID, nil)
		return nil
	}

	step := node.Step
	if step == nil {
		return fmt.Errorf("%w: node has no step definition", ErrStepNotFound)
	}

	// Рендерим конфигурацию шага
	config, err := engine.RenderConfig(step.Config, state.Context)
	if err != nil {
		return fmt.Errorf("render config for %s: %w", node.ID, err)
	}

	// Проверяем condition (если есть)
	if step.Condition != "" {
		shouldRun, err := engine.RenderCondition(step.Condition, state.Context)
		if err != nil {
			return fmt.Errorf("render condition for %s: %w", node.ID, err)
		}
		if !shouldRun {
			// Условие не выполнено — пропускаем шаг
			o.logger.Debug("step skipped due to condition",
				"run_id", state.RunID(),
				"step_id", node.ID,
			)
			state.MarkStepCompleted(node.ID, map[string]any{"skipped": true})
			return nil
		}
	}

	// Создаём task
	task := &domain.Task{
		ID:        uuid.New(),
		RunID:     state.RunID(),
		StepID:    node.ID,
		Name:      step.Name,
		Type:      step.Type,
		Attempt:   0,
		Status:    domain.TaskStatusQueued,
		Payload:   config,
		CreatedAt: time.Now(),
	}

	// Сохраняем в БД
	if err := o.taskRepo.Create(ctx, task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	// Помечаем шаг как running
	state.MarkStepRunning(node.ID, task)

	// Публикуем событие для Worker
	if err := o.publisher.PublishTaskReady(ctx, task.ID, task.RunID); err != nil {
		o.logger.Warn("failed to publish task.ready",
			"task_id", task.ID,
			"run_id", state.RunID(),
			"error", err,
		)
		// Task создан в БД — Worker может забрать через polling
	}

	o.logger.Debug("task dispatched",
		"task_id", task.ID,
		"run_id", state.RunID(),
		"step_id", node.ID,
		"type", step.Type,
	)

	return nil
}

// completeRun завершает run (успешно или с ошибкой).
func (o *Orchestrator) completeRun(ctx context.Context, state *RunState, success bool) error {
	run := state.Run

	if success {
		run.MarkSucceeded()
		o.logger.Info("run succeeded",
			"run_id", run.ID,
			"duration", run.Duration(),
		)
	} else {
		failedSteps := state.GetFailedSteps()
		errMsg := fmt.Sprintf("steps failed: %v", failedSteps)
		run.MarkFailed(errMsg)
		o.logger.Warn("run failed",
			"run_id", run.ID,
			"failed_steps", failedSteps,
			"duration", run.Duration(),
		)
	}

	// Обновляем в БД
	if err := o.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("update run status: %w", err)
	}

	// Удаляем из активных
	o.removeActiveRun(run.ID)

	return nil
}

// failRun переводит run в статус FAILED.
func (o *Orchestrator) failRun(ctx context.Context, run *domain.Run, errMsg string) error {
	run.MarkFailed(errMsg)

	if err := o.runRepo.Update(ctx, run); err != nil {
		return fmt.Errorf("update run to failed: %w", err)
	}

	o.logger.Warn("run failed early",
		"run_id", run.ID,
		"error", errMsg,
	)

	return fmt.Errorf("run failed: %s", errMsg)
}

// restoreRunState восстанавливает RunState из БД.
// Используется когда task.completed приходит для run, которого нет в памяти
// (после рестарта Orchestrator).
func (o *Orchestrator) restoreRunState(ctx context.Context, runID uuid.UUID) (*RunState, error) {
	// Загружаем run
	run, err := o.runRepo.GetByID(ctx, runID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, nil // Run не существует
		}
		return nil, fmt.Errorf("get run: %w", err)
	}

	// Если run уже завершён — ничего не делаем
	if run.IsFinished() {
		return nil, nil
	}

	// Загружаем FlowVersion
	version, err := o.flowRepo.GetVersion(ctx, run.FlowID, run.Version)
	if err != nil {
		return nil, fmt.Errorf("get flow version: %w", err)
	}

	// Создаём и инициализируем state
	state := NewRunState(run, version)
	if err := state.Initialize(); err != nil {
		return nil, fmt.Errorf("initialize state: %w", err)
	}

	// Загружаем tasks и восстанавливаем состояние
	tasks, err := o.taskRepo.ListByRunID(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	state.RestoreFromTasks(tasks)

	// Добавляем в активные
	if err := o.addActiveRun(state); err != nil {
		if errors.Is(err, ErrRunAlreadyActive) {
			// Кто-то уже восстановил — возвращаем его
			return o.getActiveRun(runID), nil
		}
		return nil, err
	}

	o.logger.Info("run state restored",
		"run_id", runID,
		"stats", state.Stats(),
	)

	return state, nil
}
