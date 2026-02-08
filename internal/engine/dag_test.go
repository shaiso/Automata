package engine

import (
	"errors"
	"testing"

	"github.com/shaiso/Automata/internal/domain"
)

func TestBuildDAG_SimpleChain(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "delay", DependsOn: []string{"A"}},
			{ID: "C", Type: "transform", DependsOn: []string{"B"}},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем количество узлов
	if dag.Size() != 3 {
		t.Errorf("expected 3 nodes, got %d", dag.Size())
	}

	// Проверяем корневые узлы
	if len(dag.RootNodes) != 1 {
		t.Errorf("expected 1 root node, got %d", len(dag.RootNodes))
	}
	if dag.RootNodes[0].ID != "A" {
		t.Errorf("expected root node A, got %s", dag.RootNodes[0].ID)
	}

	// Проверяем зависимости
	nodeB := dag.GetNode("B")
	if len(nodeB.DependsOn) != 1 || nodeB.DependsOn[0].ID != "A" {
		t.Error("node B should depend on A")
	}

	nodeC := dag.GetNode("C")
	if len(nodeC.DependsOn) != 1 || nodeC.DependsOn[0].ID != "B" {
		t.Error("node C should depend on B")
	}
}

func TestBuildDAG_Diamond(t *testing.T) {
	// A → B → D
	// A → C → D
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "http", DependsOn: []string{"A"}},
			{ID: "C", Type: "http", DependsOn: []string{"A"}},
			{ID: "D", Type: "http", DependsOn: []string{"B", "C"}},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if dag.Size() != 4 {
		t.Errorf("expected 4 nodes, got %d", dag.Size())
	}

	// Проверяем, что D зависит от B и C
	nodeD := dag.GetNode("D")
	if len(nodeD.DependsOn) != 2 {
		t.Errorf("node D should have 2 dependencies, got %d", len(nodeD.DependsOn))
	}

	// Проверяем inDegree
	if dag.GetNode("A").InDegree != 0 {
		t.Error("A should have inDegree 0")
	}
	if dag.GetNode("B").InDegree != 1 {
		t.Error("B should have inDegree 1")
	}
	if dag.GetNode("C").InDegree != 1 {
		t.Error("C should have inDegree 1")
	}
	if dag.GetNode("D").InDegree != 2 {
		t.Error("D should have inDegree 2")
	}
}

func TestBuildDAG_Parallel(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "start", Type: "http"},
			{
				ID:        "parallel",
				Type:      "parallel",
				DependsOn: []string{"start"},
				Branches: []domain.Branch{
					{
						ID: "branch_a",
						Steps: []domain.StepDef{
							{ID: "step1", Type: "http"},
							{ID: "step2", Type: "delay"},
						},
					},
					{
						ID: "branch_b",
						Steps: []domain.StepDef{
							{ID: "step1", Type: "http"},
						},
					},
				},
			},
			{ID: "end", Type: "http", DependsOn: []string{"parallel"}},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем наличие узлов
	// start, parallel, parallel.branch_a.step1, parallel.branch_a.step2,
	// parallel.branch_b.step1, parallel.join, end
	expectedNodes := []string{
		"start",
		"parallel",
		"parallel.branch_a.step1",
		"parallel.branch_a.step2",
		"parallel.branch_b.step1",
		"parallel.join",
		"end",
	}

	for _, id := range expectedNodes {
		if dag.GetNode(id) == nil {
			t.Errorf("expected node %s to exist", id)
		}
	}

	// Проверяем, что шаги веток зависят от parallel start
	step1a := dag.GetNode("parallel.branch_a.step1")
	if step1a == nil {
		t.Fatal("parallel.branch_a.step1 not found")
	}
	if len(step1a.DependsOn) != 1 || step1a.DependsOn[0].ID != "parallel" {
		t.Error("parallel.branch_a.step1 should depend on parallel")
	}

	// Проверяем, что join зависит от последних шагов веток
	joinNode := dag.GetNode("parallel.join")
	if joinNode == nil {
		t.Fatal("parallel.join not found")
	}
	if len(joinNode.DependsOn) != 2 {
		t.Errorf("join should have 2 dependencies, got %d", len(joinNode.DependsOn))
	}
}

func TestBuildDAG_CyclicDependency(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http", DependsOn: []string{"C"}},
			{ID: "B", Type: "http", DependsOn: []string{"A"}},
			{ID: "C", Type: "http", DependsOn: []string{"B"}},
		},
	}

	_, err := BuildDAG(spec)
	if !errors.Is(err, ErrCyclicDependency) {
		t.Errorf("expected ErrCyclicDependency, got %v", err)
	}
}

func TestGetReadyNodes(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "http"},
			{ID: "C", Type: "http", DependsOn: []string{"A"}},
			{ID: "D", Type: "http", DependsOn: []string{"A", "B"}},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Изначально готовы A и B (без зависимостей)
	ready := dag.GetReadyNodes(nil, nil)
	if len(ready) != 2 {
		t.Errorf("expected 2 ready nodes, got %d", len(ready))
	}

	readyIDs := make(map[string]bool)
	for _, node := range ready {
		readyIDs[node.ID] = true
	}
	if !readyIDs["A"] || !readyIDs["B"] {
		t.Error("A and B should be ready initially")
	}

	// После завершения A, готов C
	completed := map[string]bool{"A": true}
	ready = dag.GetReadyNodes(completed, nil)

	readyIDs = make(map[string]bool)
	for _, node := range ready {
		readyIDs[node.ID] = true
	}
	if !readyIDs["B"] || !readyIDs["C"] {
		t.Error("B and C should be ready after A completes")
	}
	if readyIDs["D"] {
		t.Error("D should not be ready (depends on B)")
	}

	// После завершения A и B, готов D
	completed = map[string]bool{"A": true, "B": true}
	ready = dag.GetReadyNodes(completed, nil)

	readyIDs = make(map[string]bool)
	for _, node := range ready {
		readyIDs[node.ID] = true
	}
	if !readyIDs["C"] || !readyIDs["D"] {
		t.Error("C and D should be ready after A and B complete")
	}
}

func TestGetReadyNodes_WithRunning(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "http"},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A выполняется, B готов
	running := map[string]bool{"A": true}
	ready := dag.GetReadyNodes(nil, running)

	if len(ready) != 1 {
		t.Errorf("expected 1 ready node, got %d", len(ready))
	}
	if ready[0].ID != "B" {
		t.Errorf("expected B to be ready, got %s", ready[0].ID)
	}
}

func TestTopologicalSort(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "http", DependsOn: []string{"A"}},
			{ID: "C", Type: "http", DependsOn: []string{"A"}},
			{ID: "D", Type: "http", DependsOn: []string{"B", "C"}},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем, что порядок корректный
	order := dag.Order
	if len(order) != 4 {
		t.Errorf("expected 4 nodes in order, got %d", len(order))
	}

	// A должен быть перед B, C, D
	// B и C должны быть перед D
	positions := make(map[string]int)
	for i, node := range order {
		positions[node.ID] = i
	}

	if positions["A"] > positions["B"] {
		t.Error("A should come before B")
	}
	if positions["A"] > positions["C"] {
		t.Error("A should come before C")
	}
	if positions["B"] > positions["D"] {
		t.Error("B should come before D")
	}
	if positions["C"] > positions["D"] {
		t.Error("C should come before D")
	}
}

func TestDAG_IsComplete(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "A", Type: "http"},
			{ID: "B", Type: "http"},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Не завершён
	if dag.IsComplete(nil) {
		t.Error("should not be complete with no completed nodes")
	}

	if dag.IsComplete(map[string]bool{"A": true}) {
		t.Error("should not be complete with only A completed")
	}

	// Завершён
	if !dag.IsComplete(map[string]bool{"A": true, "B": true}) {
		t.Error("should be complete with all nodes completed")
	}
}

func TestDAG_GetExecutableNodes(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{
				ID:   "parallel",
				Type: "parallel",
				Branches: []domain.Branch{
					{ID: "branch", Steps: []domain.StepDef{{ID: "step", Type: "http"}}},
				},
			},
		},
	}

	dag, err := BuildDAG(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Должны быть только исполняемые узлы (не join)
	executable := dag.GetExecutableNodes()

	for _, node := range executable {
		if node.IsJoin {
			t.Errorf("join node %s should not be in executable list", node.ID)
		}
	}
}
