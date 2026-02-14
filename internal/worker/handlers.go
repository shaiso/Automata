package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
	"github.com/shaiso/Automata/internal/mq"
	"github.com/shaiso/Automata/internal/repo"
)

// handleTaskReady обрабатывает событие о новой task из очереди tasks.ready.
func (w *Worker) handleTaskReady(ctx context.Context, delivery *mq.Delivery) error {
	// Парсим payload
	payload, err := mq.ParsePayload[mq.TaskReadyPayload](&delivery.Message)
	if err != nil {
		w.logger.Error("failed to parse task.ready payload", "error", err)
		return err
	}

	w.logger.Debug("received task.ready event",
		"task_id", payload.TaskID,
		"run_id", payload.RunID,
	)

	// Обрабатываем task
	if err := w.processTask(ctx, payload.TaskID); err != nil {
		// Ожидаемые ситуации — не возвращаем ошибку (ack)
		if errors.Is(err, ErrTaskNotFound) || errors.Is(err, ErrTaskNotQueued) {
			w.logger.Debug("task not processed", "task_id", payload.TaskID, "reason", err)
			return nil
		}
		w.logger.Error("failed to process task", "task_id", payload.TaskID, "error", err)
		return err
	}

	return nil
}

// processTask загружает task из БД, выполняет и обрабатывает результат.
func (w *Worker) processTask(ctx context.Context, taskID uuid.UUID) error {
	// 1. Загружаем task из БД
	task, err := w.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return fmt.Errorf("%w: %s", ErrTaskNotFound, taskID)
		}
		return fmt.Errorf("get task: %w", err)
	}

	// 2. Проверяем статус
	if task.Status != domain.TaskStatusQueued {
		return ErrTaskNotQueued
	}

	// 3. Помечаем как running
	task.MarkRunning()
	if err := w.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task to running: %w", err)
	}

	w.logger.Info("task started",
		"task_id", task.ID,
		"run_id", task.RunID,
		"step_id", task.StepID,
		"type", task.Type,
		"attempt", task.Attempt,
	)

	// 4. Загружаем RetryPolicy
	retryPolicy := w.getRetryPolicy(ctx, task)

	// 5. Выполняем с retry
	result, execErr := w.executeWithRetry(ctx, task, retryPolicy)

	// 6. Обрабатываем результат
	if execErr == nil && result.Error == "" {
		// Успех
		task.MarkSucceeded(result.Outputs)
		if err := w.taskRepo.Update(ctx, task); err != nil {
			return fmt.Errorf("update task to succeeded: %w", err)
		}

		w.logger.Info("task succeeded",
			"task_id", task.ID,
			"run_id", task.RunID,
			"step_id", task.StepID,
			"attempt", task.Attempt,
		)

		return w.publishCompletion(ctx, task, "")
	}

	// Ошибка
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
	} else {
		errMsg = result.Error
	}

	task.MarkFailed(errMsg)
	if err := w.taskRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("update task to failed: %w", err)
	}

	w.logger.Warn("task failed",
		"task_id", task.ID,
		"run_id", task.RunID,
		"step_id", task.StepID,
		"attempt", task.Attempt,
		"error", errMsg,
	)

	return w.publishCompletion(ctx, task, errMsg)
}

// publishCompletion публикует событие task.completed.
func (w *Worker) publishCompletion(ctx context.Context, task *domain.Task, errMsg string) error {
	if w.publisher == nil {
		w.logger.Warn("publisher not available, skipping task.completed publish",
			"task_id", task.ID,
		)
		return nil
	}

	payload := mq.TaskCompletedPayload{
		TaskID:  task.ID,
		RunID:   task.RunID,
		StepID:  task.StepID,
		Status:  string(task.Status),
		Error:   errMsg,
		Attempt: task.Attempt,
	}

	if err := w.publisher.PublishTaskCompleted(ctx, payload); err != nil {
		w.logger.Warn("failed to publish task.completed",
			"task_id", task.ID,
			"error", err,
		)
		// Не возвращаем ошибку — task обновлён в БД, оркестратор подхватит через polling
	}

	return nil
}

// executeWithRetry выполняет task с retry согласно RetryPolicy.
func (w *Worker) executeWithRetry(ctx context.Context, task *domain.Task, policy *domain.RetryPolicy) (*ExecutionResult, error) {
	// Получаем executor
	executor, err := w.registry.Get(task.Type)
	if err != nil {
		return nil, err
	}

	maxAttempts := 1
	if policy != nil && policy.MaxAttempts > 0 {
		maxAttempts = policy.MaxAttempts
	}

	var lastResult *ExecutionResult
	var lastErr error

	for {
		// Выполняем
		lastResult, lastErr = executor.Execute(ctx, task)

		// Успех — инфраструктурной ошибки нет и логической ошибки нет
		if lastErr == nil && (lastResult == nil || lastResult.Error == "") {
			return lastResult, nil
		}

		// Проверяем, можно ли делать retry
		if !task.CanRetry(maxAttempts) {
			break
		}

		// Для HTTP: проверяем OnStatus
		if !w.shouldRetry(lastResult, lastErr, policy) {
			break
		}

		// Считаем backoff
		delay := calculateBackoff(task.Attempt, policy)

		w.logger.Debug("retrying task",
			"task_id", task.ID,
			"attempt", task.Attempt,
			"delay", delay,
		)

		// Ждём с учётом context
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		// Сброс и новая попытка
		task.ResetForRetry()
		task.MarkRunning()
		if err := w.taskRepo.Update(ctx, task); err != nil {
			return nil, fmt.Errorf("update task for retry: %w", err)
		}
	}

	return lastResult, lastErr
}

// shouldRetry определяет, нужно ли делать retry.
func (w *Worker) shouldRetry(result *ExecutionResult, execErr error, policy *domain.RetryPolicy) bool {
	// Инфраструктурная ошибка — всегда retry
	if execErr != nil {
		return true
	}

	// Нет policy — нет retry
	if policy == nil {
		return false
	}

	// Для HTTP: проверяем OnStatus
	if result != nil && result.Outputs != nil && len(policy.OnStatus) > 0 {
		if statusCode, ok := result.Outputs["status_code"]; ok {
			if code, ok := statusCode.(int); ok {
				return shouldRetryHTTPStatus(code, policy.OnStatus)
			}
		}
		// Если есть OnStatus но нет status_code — не retry
		return false
	}

	// Логическая ошибка без OnStatus — retry
	return true
}

// shouldRetryHTTPStatus проверяет, входит ли HTTP-код в список для retry.
func shouldRetryHTTPStatus(statusCode int, onStatus []int) bool {
	for _, code := range onStatus {
		if statusCode == code {
			return true
		}
	}
	return false
}

// calculateBackoff вычисляет задержку перед retry.
func calculateBackoff(attempt int, policy *domain.RetryPolicy) time.Duration {
	if policy == nil {
		return time.Second
	}

	initialDelay := time.Duration(policy.InitialDelayMs) * time.Millisecond
	if initialDelay <= 0 {
		initialDelay = time.Second
	}

	maxDelay := time.Duration(policy.MaxDelayMs) * time.Millisecond
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	var delay time.Duration
	switch policy.Backoff {
	case "exponential":
		// delay = initialDelay * 2^(attempt-1)
		delay = initialDelay
		for i := 1; i < attempt; i++ {
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
				break
			}
		}
	default:
		// "fixed" или неизвестный — используем initialDelay
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// getRetryPolicy загружает RetryPolicy для task из FlowVersion.
func (w *Worker) getRetryPolicy(ctx context.Context, task *domain.Task) *domain.RetryPolicy {
	// Загружаем run для FlowID и Version
	run, err := w.runRepo.GetByID(ctx, task.RunID)
	if err != nil {
		w.logger.Debug("failed to load run for retry policy", "run_id", task.RunID, "error", err)
		return nil
	}

	// Загружаем FlowVersion
	version, err := w.flowRepo.GetVersion(ctx, run.FlowID, run.Version)
	if err != nil {
		w.logger.Debug("failed to load flow version for retry policy", "flow_id", run.FlowID, "error", err)
		return nil
	}

	// Ищем StepDef по StepID
	stepDef := findStepDef(version.Spec.Steps, task.StepID)
	if stepDef != nil && stepDef.Retry != nil {
		return stepDef.Retry
	}

	// Fallback на defaults
	if version.Spec.Defaults != nil && version.Spec.Defaults.Retry != nil {
		return version.Spec.Defaults.Retry
	}

	return nil
}

// findStepDef ищет StepDef по ID, включая шаги внутри parallel-веток.
func findStepDef(steps []domain.StepDef, stepID string) *domain.StepDef {
	for i := range steps {
		step := &steps[i]
		if step.ID == stepID {
			return step
		}

		// Для parallel — ищем в ветках (с учётом prefix: parallel_id.branch_id.step_id)
		if step.Type == "parallel" {
			for _, branch := range step.Branches {
				for j := range branch.Steps {
					branchStep := &branch.Steps[j]
					// Проверяем как прямой ID, так и prefixed
					prefixedID := fmt.Sprintf("%s.%s.%s", step.ID, branch.ID, branchStep.ID)
					if branchStep.ID == stepID || prefixedID == stepID {
						return branchStep
					}
				}
			}
		}
	}
	return nil
}
