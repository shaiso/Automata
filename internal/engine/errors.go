package engine

import "errors"

// Ошибки валидации FlowSpec.
var (
	// ErrEmptySteps — flow не содержит шагов.
	ErrEmptySteps = errors.New("flow spec has no steps")

	// ErrEmptyStepID — шаг не имеет ID.
	ErrEmptyStepID = errors.New("step has empty ID")

	// ErrDuplicateStepID — несколько шагов с одинаковым ID.
	ErrDuplicateStepID = errors.New("duplicate step ID")

	// ErrUnknownStepType — неизвестный тип шага.
	ErrUnknownStepType = errors.New("unknown step type")

	// ErrMissingDependency — шаг зависит от несуществующего шага.
	ErrMissingDependency = errors.New("step depends on unknown step")

	// ErrCyclicDependency — обнаружен цикл в зависимостях.
	ErrCyclicDependency = errors.New("cyclic dependency detected")

	// ErrSelfDependency — шаг зависит от самого себя.
	ErrSelfDependency = errors.New("step depends on itself")
)

// Ошибки рендеринга шаблонов.
var (
	// ErrTemplateRender — ошибка рендеринга шаблона.
	ErrTemplateRender = errors.New("template render failed")

	// ErrTemplateParse — ошибка парсинга шаблона.
	ErrTemplateParse = errors.New("template parse failed")
)

// Ошибки parallel шагов.
var (
	// ErrEmptyBranches — parallel шаг не содержит веток.
	ErrEmptyBranches = errors.New("parallel step has no branches")

	// ErrEmptyBranchID — ветка не имеет ID.
	ErrEmptyBranchID = errors.New("branch has empty ID")

	// ErrDuplicateBranchID — несколько веток с одинаковым ID.
	ErrDuplicateBranchID = errors.New("duplicate branch ID")

	// ErrEmptyBranchSteps — ветка не содержит шагов.
	ErrEmptyBranchSteps = errors.New("branch has no steps")
)

// ValidationError — ошибка валидации с контекстом.
type ValidationError struct {
	StepID  string // ID шага, где произошла ошибка
	Field   string // поле, вызвавшее ошибку
	Message string // описание ошибки
	Err     error  // базовая ошибка
}

// Error реализует интерфейс error.
func (e *ValidationError) Error() string {
	if e.StepID != "" {
		return "step " + e.StepID + ": " + e.Message
	}
	return e.Message
}

// Unwrap возвращает базовую ошибку.
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// NewValidationError создаёт новую ошибку валидации.
func NewValidationError(stepID, field, message string, err error) *ValidationError {
	return &ValidationError{
		StepID:  stepID,
		Field:   field,
		Message: message,
		Err:     err,
	}
}
