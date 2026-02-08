package steps

import (
	"fmt"
	"sort"
	"sync"
)

// Registry — реестр типов шагов.
//
// Позволяет регистрировать и получать реализации Step по типу.
// Потокобезопасен.
type Registry struct {
	mu    sync.RWMutex
	steps map[string]Step
}

// NewRegistry создаёт пустой реестр.
func NewRegistry() *Registry {
	return &Registry{
		steps: make(map[string]Step),
	}
}

// DefaultRegistry создаёт реестр со всеми стандартными шагами.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Регистрируем все стандартные шаги
	r.Register(NewDelayStep())
	r.Register(NewHTTPStep())
	r.Register(NewTransformStep())
	r.Register(NewParallelStep())

	return r
}

// Register регистрирует шаг в реестре.
// Если шаг с таким типом уже существует, он будет перезаписан.
func (r *Registry) Register(step Step) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps[step.Type()] = step
}

// Get возвращает шаг по типу.
// Возвращает ErrStepNotFound, если шаг не найден.
func (r *Registry) Get(stepType string) (Step, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	step, exists := r.steps[stepType]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrStepNotFound, stepType)
	}

	return step, nil
}

// Has проверяет, зарегистрирован ли шаг.
func (r *Registry) Has(stepType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.steps[stepType]
	return exists
}

// Types возвращает список всех зарегистрированных типов шагов.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	types := make([]string, 0, len(r.steps))
	for t := range r.steps {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// Count возвращает количество зарегистрированных шагов.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.steps)
}

// Unregister удаляет шаг из реестра.
func (r *Registry) Unregister(stepType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.steps, stepType)
}
