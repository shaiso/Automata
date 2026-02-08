package engine

import (
	"fmt"

	"github.com/shaiso/Automata/internal/domain"
)

// Допустимые типы шагов.
var validStepTypes = map[string]bool{
	"http":      true,
	"delay":     true,
	"transform": true,
	"parallel":  true,
}

// Validate выполняет полную валидацию FlowSpec.
//
// Проверяет:
// - Наличие шагов
// - Уникальность ID шагов
// - Корректность типов шагов
// - Валидность зависимостей (depends_on)
// - Отсутствие циклов (делегируется DAG)
// - Валидность parallel веток
func Validate(spec *domain.FlowSpec) error {
	if spec == nil {
		return ErrEmptySteps
	}

	if len(spec.Steps) == 0 {
		return ErrEmptySteps
	}

	// Собираем все ID шагов (включая вложенные в parallel)
	stepIDs := make(map[string]bool)

	// Валидируем каждый шаг
	for i := range spec.Steps {
		step := &spec.Steps[i]

		if err := ValidateStep(step, stepIDs); err != nil {
			return err
		}
	}

	// Валидируем зависимости
	if err := validateDependencies(spec.Steps, stepIDs); err != nil {
		return err
	}

	return nil
}

// ValidateStep валидирует один шаг.
// stepIDs — уже встреченные ID шагов (для проверки уникальности).
func ValidateStep(step *domain.StepDef, stepIDs map[string]bool) error {
	// Проверка ID
	if step.ID == "" {
		return NewValidationError("", "id", "step has empty ID", ErrEmptyStepID)
	}

	// Проверка уникальности ID
	if stepIDs[step.ID] {
		return NewValidationError(step.ID, "id",
			fmt.Sprintf("duplicate step ID: %s", step.ID), ErrDuplicateStepID)
	}
	stepIDs[step.ID] = true

	// Проверка типа
	if err := validateStepType(step.ID, step.Type); err != nil {
		return err
	}

	// Проверка self-dependency
	for _, dep := range step.DependsOn {
		if dep == step.ID {
			return NewValidationError(step.ID, "depends_on",
				"step depends on itself", ErrSelfDependency)
		}
	}

	// Специальная валидация для parallel
	if step.Type == "parallel" {
		if err := validateParallelStep(step, stepIDs); err != nil {
			return err
		}
	}

	return nil
}

// validateStepType проверяет, что тип шага известен.
func validateStepType(stepID, stepType string) error {
	if stepType == "" {
		return NewValidationError(stepID, "type",
			"step has empty type", ErrUnknownStepType)
	}

	if !validStepTypes[stepType] {
		return NewValidationError(stepID, "type",
			fmt.Sprintf("unknown step type: %s", stepType), ErrUnknownStepType)
	}

	return nil
}

// validateDependencies проверяет, что все depends_on ссылаются на существующие шаги.
func validateDependencies(steps []domain.StepDef, stepIDs map[string]bool) error {
	for i := range steps {
		step := &steps[i]

		for _, dep := range step.DependsOn {
			if !stepIDs[dep] {
				return NewValidationError(step.ID, "depends_on",
					fmt.Sprintf("depends on unknown step: %s", dep), ErrMissingDependency)
			}
		}

		// Рекурсивно проверяем зависимости внутри parallel веток
		if step.Type == "parallel" {
			for _, branch := range step.Branches {
				if err := validateDependencies(branch.Steps, stepIDs); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validateParallelStep валидирует parallel шаг и его ветки.
func validateParallelStep(step *domain.StepDef, stepIDs map[string]bool) error {
	if len(step.Branches) == 0 {
		return NewValidationError(step.ID, "branches",
			"parallel step has no branches", ErrEmptyBranches)
	}

	branchIDs := make(map[string]bool)

	for i := range step.Branches {
		branch := &step.Branches[i]

		// Проверка ID ветки
		if branch.ID == "" {
			return NewValidationError(step.ID, "branches",
				fmt.Sprintf("branch %d has empty ID", i), ErrEmptyBranchID)
		}

		// Проверка уникальности ID ветки
		if branchIDs[branch.ID] {
			return NewValidationError(step.ID, "branches",
				fmt.Sprintf("duplicate branch ID: %s", branch.ID), ErrDuplicateBranchID)
		}
		branchIDs[branch.ID] = true

		// Проверка наличия шагов в ветке
		if len(branch.Steps) == 0 {
			return NewValidationError(step.ID, "branches",
				fmt.Sprintf("branch %s has no steps", branch.ID), ErrEmptyBranchSteps)
		}

		// Валидируем шаги внутри ветки
		// ID шагов в ветках получают префикс: {parallel_id}.{branch_id}.{step_id}
		for j := range branch.Steps {
			branchStep := &branch.Steps[j]

			// Формируем полный ID для шага внутри ветки
			fullStepID := fmt.Sprintf("%s.%s.%s", step.ID, branch.ID, branchStep.ID)

			// Временно подменяем ID для валидации
			originalID := branchStep.ID
			branchStep.ID = fullStepID

			if err := ValidateStep(branchStep, stepIDs); err != nil {
				branchStep.ID = originalID
				return err
			}

			branchStep.ID = originalID
		}
	}

	return nil
}

// IsValidStepType проверяет, является ли тип шага допустимым.
func IsValidStepType(stepType string) bool {
	return validStepTypes[stepType]
}

// GetValidStepTypes возвращает список допустимых типов шагов.
func GetValidStepTypes() []string {
	types := make([]string, 0, len(validStepTypes))
	for t := range validStepTypes {
		types = append(types, t)
	}
	return types
}
