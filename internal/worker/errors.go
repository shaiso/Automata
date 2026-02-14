package worker

import "errors"

// Ошибки воркера.
var (
	// ErrTaskNotFound — task не найден в БД.
	ErrTaskNotFound = errors.New("task not found")

	// ErrTaskNotQueued — task не в статусе QUEUED.
	ErrTaskNotQueued = errors.New("task is not in QUEUED status")

	// ErrUnknownStepType — нет executor'а для данного типа шага.
	ErrUnknownStepType = errors.New("unknown step type")

	// ErrExecutionTimeout — выполнение task превысило таймаут.
	ErrExecutionTimeout = errors.New("execution timeout")

	// ErrExecutionFailed — выполнение task завершилось ошибкой.
	ErrExecutionFailed = errors.New("execution failed")

	// ErrWorkerStopped — воркер остановлен.
	ErrWorkerStopped = errors.New("worker stopped")

	// ErrRetryExhausted — все попытки retry исчерпаны.
	ErrRetryExhausted = errors.New("retry attempts exhausted")

	// ErrStepDefNotFound — определение шага не найдено.
	ErrStepDefNotFound = errors.New("step definition not found")

	// ErrHTTPRequest — HTTP-запрос завершился ошибкой.
	ErrHTTPRequest = errors.New("http request failed")
)
