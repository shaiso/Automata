package engine

import (
	"fmt"

	"github.com/shaiso/Automata/internal/domain"
)

// Node — узел в DAG.
type Node struct {
	// Step — определение шага из FlowSpec.
	Step *domain.StepDef

	// ID — идентификатор узла (может отличаться от Step.ID для вложенных шагов).
	ID string

	// InDegree — количество входящих рёбер (зависимостей).
	InDegree int

	// DependsOn — узлы, от которых зависит этот узел.
	DependsOn []*Node

	// Dependents — узлы, которые зависят от этого узла.
	Dependents []*Node

	// IsJoin — true, если это виртуальный join-узел для parallel.
	IsJoin bool

	// ParallelID — ID родительского parallel шага (для шагов внутри веток).
	ParallelID string

	// BranchID — ID ветки (для шагов внутри parallel).
	BranchID string
}

// DAG — направленный ациклический граф шагов flow.
type DAG struct {
	// Nodes — все узлы графа (stepID → Node).
	Nodes map[string]*Node

	// RootNodes — узлы без зависимостей (точки входа).
	RootNodes []*Node

	// Order — топологически отсортированный список узлов.
	Order []*Node
}

// BuildDAG строит DAG из FlowSpec.
//
// Для parallel шагов создаются дополнительные узлы:
// - Сам parallel шаг как "start" узел
// - Шаги внутри веток с prefixed ID
// - Виртуальный "join" узел, объединяющий все ветки
func BuildDAG(spec *domain.FlowSpec) (*DAG, error) {
	dag := &DAG{
		Nodes:     make(map[string]*Node),
		RootNodes: make([]*Node, 0),
	}

	// Первый проход: создаём все узлы
	for i := range spec.Steps {
		step := &spec.Steps[i]

		if err := dag.addNode(step); err != nil {
			return nil, err
		}
	}

	// Второй проход: связываем узлы по зависимостям
	for i := range spec.Steps {
		step := &spec.Steps[i]

		if err := dag.linkDependencies(step); err != nil {
			return nil, err
		}
	}

	// Находим корневые узлы
	dag.findRootNodes()

	// Проверяем на циклы и строим топологический порядок
	order, err := dag.topologicalSort()
	if err != nil {
		return nil, err
	}
	dag.Order = order

	return dag, nil
}

// addNode добавляет узел в DAG.
func (d *DAG) addNode(step *domain.StepDef) error {
	node := &Node{
		Step:       step,
		ID:         step.ID,
		DependsOn:  make([]*Node, 0),
		Dependents: make([]*Node, 0),
	}
	d.Nodes[step.ID] = node

	// Для parallel шагов добавляем вложенные узлы
	if step.Type == "parallel" {
		if err := d.addParallelNodes(step); err != nil {
			return err
		}
	}

	return nil
}

// addParallelNodes добавляет узлы для parallel шага.
func (d *DAG) addParallelNodes(parallelStep *domain.StepDef) error {
	// Добавляем шаги из каждой ветки
	for _, branch := range parallelStep.Branches {
		for i := range branch.Steps {
			branchStep := &branch.Steps[i]

			// Полный ID: parallel_id.branch_id.step_id
			fullID := fmt.Sprintf("%s.%s.%s", parallelStep.ID, branch.ID, branchStep.ID)

			node := &Node{
				Step:       branchStep,
				ID:         fullID,
				ParallelID: parallelStep.ID,
				BranchID:   branch.ID,
				DependsOn:  make([]*Node, 0),
				Dependents: make([]*Node, 0),
			}
			d.Nodes[fullID] = node

			// Рекурсивно обрабатываем вложенные parallel
			if branchStep.Type == "parallel" {
				// Для вложенных parallel нужно использовать полный ID как базу
				// Это сложный случай, который можно оставить на будущее
				// Пока просто добавляем узел
			}
		}
	}

	// Создаём join-узел для parallel
	joinID := parallelStep.ID + ".join"
	joinNode := &Node{
		ID:         joinID,
		Step:       nil, // виртуальный узел
		IsJoin:     true,
		ParallelID: parallelStep.ID,
		DependsOn:  make([]*Node, 0),
		Dependents: make([]*Node, 0),
	}
	d.Nodes[joinID] = joinNode

	return nil
}

// linkDependencies связывает узлы по зависимостям.
func (d *DAG) linkDependencies(step *domain.StepDef) error {
	node := d.Nodes[step.ID]

	// Связываем depends_on на уровне flow
	for _, depID := range step.DependsOn {
		depNode, exists := d.Nodes[depID]
		if !exists {
			// Проверяем, может это join-узел
			depNode, exists = d.Nodes[depID+".join"]
			if !exists {
				return NewValidationError(step.ID, "depends_on",
					fmt.Sprintf("depends on unknown step: %s", depID), ErrMissingDependency)
			}
		}

		d.addEdge(depNode, node)
	}

	// Для parallel шагов связываем внутренние зависимости
	if step.Type == "parallel" {
		if err := d.linkParallelDependencies(step); err != nil {
			return err
		}
	}

	return nil
}

// linkParallelDependencies связывает зависимости внутри parallel шага.
func (d *DAG) linkParallelDependencies(parallelStep *domain.StepDef) error {
	parallelNode := d.Nodes[parallelStep.ID]
	joinNode := d.Nodes[parallelStep.ID+".join"]

	for _, branch := range parallelStep.Branches {
		var prevNode *Node

		for i := range branch.Steps {
			branchStep := &branch.Steps[i]
			fullID := fmt.Sprintf("%s.%s.%s", parallelStep.ID, branch.ID, branchStep.ID)
			node := d.Nodes[fullID]

			if i == 0 {
				// Первый шаг ветки зависит от parallel start
				d.addEdge(parallelNode, node)
			} else {
				// Остальные шаги зависят от предыдущего шага в ветке
				d.addEdge(prevNode, node)
			}

			// Также обрабатываем явные depends_on внутри ветки
			for _, depID := range branchStep.DependsOn {
				// Проверяем локальный ID (внутри той же ветки)
				localDepID := fmt.Sprintf("%s.%s.%s", parallelStep.ID, branch.ID, depID)
				if depNode, exists := d.Nodes[localDepID]; exists {
					d.addEdge(depNode, node)
				} else if depNode, exists := d.Nodes[depID]; exists {
					// Зависимость на внешний шаг
					d.addEdge(depNode, node)
				}
			}

			prevNode = node

			// Последний шаг ветки → join
			if i == len(branch.Steps)-1 {
				d.addEdge(node, joinNode)
			}
		}
	}

	return nil
}

// addEdge добавляет ребро между узлами.
// Дополнительно проверяет на дубликаты, чтобы избежать двойного учета InDegree.
func (d *DAG) addEdge(from, to *Node) {
	for _, dep := range to.DependsOn {
		if dep.ID == from.ID {
			return // уже связаны
		}
	}
	from.Dependents = append(from.Dependents, to)
	to.DependsOn = append(to.DependsOn, from)
	to.InDegree++
}

// findRootNodes находит узлы без входящих рёбер.
func (d *DAG) findRootNodes() {
	d.RootNodes = make([]*Node, 0)
	for _, node := range d.Nodes {
		if node.InDegree == 0 {
			d.RootNodes = append(d.RootNodes, node)
		}
	}
}

// topologicalSort выполняет топологическую сортировку (алгоритм Кана).
// Возвращает ошибку, если обнаружен цикл.
func (d *DAG) topologicalSort() ([]*Node, error) {
	// Копируем inDegree, чтобы не модифицировать оригинал
	inDegree := make(map[string]int)
	for id, node := range d.Nodes {
		inDegree[id] = node.InDegree
	}

	// Очередь узлов с inDegree = 0
	queue := make([]*Node, len(d.RootNodes))
	copy(queue, d.RootNodes)

	order := make([]*Node, 0, len(d.Nodes))

	for len(queue) > 0 {
		// Извлекаем узел из очереди
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		// Уменьшаем inDegree у зависимых узлов
		for _, dependent := range node.Dependents {
			inDegree[dependent.ID]--
			if inDegree[dependent.ID] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Если не все узлы обработаны — есть цикл
	if len(order) != len(d.Nodes) {
		return nil, ErrCyclicDependency
	}

	return order, nil
}

// GetReadyNodes возвращает узлы, готовые к выполнению.
//
// Узел готов, если:
// - Все его зависимости завершены (в completed)
// - Сам узел ещё не завершён и не в процессе (не в completed и не в running)
//
// completed — map stepID → true для завершённых шагов.
// running — map stepID → true для шагов в процессе выполнения.
func (d *DAG) GetReadyNodes(completed, running map[string]bool) []*Node {
	if completed == nil {
		completed = make(map[string]bool)
	}
	if running == nil {
		running = make(map[string]bool)
	}

	ready := make([]*Node, 0)

	for _, node := range d.Nodes {
		// Пропускаем уже завершённые или выполняющиеся
		if completed[node.ID] || running[node.ID] {
			continue
		}

		// Пропускаем виртуальные join-узлы — они не выполняются напрямую
		if node.IsJoin {
			// Join считается завершённым, когда все его зависимости завершены
			allDepsCompleted := true
			for _, dep := range node.DependsOn {
				if !completed[dep.ID] {
					allDepsCompleted = false
					break
				}
			}
			if allDepsCompleted {
				// Автоматически помечаем join как завершённый
				completed[node.ID] = true
			}
			continue
		}

		// Проверяем, что все зависимости завершены
		allDepsCompleted := true
		for _, dep := range node.DependsOn {
			if !completed[dep.ID] {
				allDepsCompleted = false
				break
			}
		}

		if allDepsCompleted {
			ready = append(ready, node)
		}
	}

	return ready
}

// GetNode возвращает узел по ID.
func (d *DAG) GetNode(id string) *Node {
	return d.Nodes[id]
}

// Size возвращает количество узлов в DAG.
func (d *DAG) Size() int {
	return len(d.Nodes)
}

// GetExecutableNodes возвращает только исполняемые узлы (не join).
func (d *DAG) GetExecutableNodes() []*Node {
	nodes := make([]*Node, 0)
	for _, node := range d.Nodes {
		if !node.IsJoin {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// IsComplete проверяет, все ли узлы завершены.
func (d *DAG) IsComplete(completed map[string]bool) bool {
	for _, node := range d.Nodes {
		if !completed[node.ID] {
			return false
		}
	}
	return true
}

// GetParallelBranchNodes возвращает все узлы для конкретной ветки parallel.
func (d *DAG) GetParallelBranchNodes(parallelID, branchID string) []*Node {
	nodes := make([]*Node, 0)
	prefix := fmt.Sprintf("%s.%s.", parallelID, branchID)

	for id, node := range d.Nodes {
		if len(id) > len(prefix) && id[:len(prefix)] == prefix {
			nodes = append(nodes, node)
		}
	}

	return nodes
}
