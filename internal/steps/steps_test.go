package steps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shaiso/Automata/internal/engine"
)

// Registry Tests

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// Пустой реестр
	if r.Count() != 0 {
		t.Errorf("expected empty registry")
	}

	// Регистрация
	r.Register(NewDelayStep())
	if r.Count() != 1 {
		t.Errorf("expected 1 step, got %d", r.Count())
	}

	// Получение
	step, err := r.Get("delay")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if step.Type() != "delay" {
		t.Errorf("expected delay, got %s", step.Type())
	}

	// Несуществующий тип
	_, err = r.Get("unknown")
	if !errors.Is(err, ErrStepNotFound) {
		t.Errorf("expected ErrStepNotFound, got %v", err)
	}

	// Has
	if !r.Has("delay") {
		t.Error("should have delay")
	}
	if r.Has("unknown") {
		t.Error("should not have unknown")
	}

	// Unregister
	r.Unregister("delay")
	if r.Has("delay") {
		t.Error("should not have delay after unregister")
	}
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()

	expectedTypes := []string{"delay", "http", "transform", "parallel"}
	for _, typ := range expectedTypes {
		if !r.Has(typ) {
			t.Errorf("default registry should have %s", typ)
		}
	}

	types := r.Types()
	if len(types) != len(expectedTypes) {
		t.Errorf("expected %d types, got %d", len(expectedTypes), len(types))
	}
}

// Delay Step Tests

func TestDelayStep_Type(t *testing.T) {
	step := NewDelayStep()
	if step.Type() != "delay" {
		t.Errorf("expected 'delay', got %s", step.Type())
	}
}

func TestDelayStep_Execute(t *testing.T) {
	step := NewDelayStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"duration_ms": 50,
		},
	}

	start := time.Now()
	resp, err := step.Execute(ctx, req)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}

	// Проверяем, что задержка была выполнена
	if elapsed < 50*time.Millisecond {
		t.Errorf("delay was too short: %v", elapsed)
	}

	// Проверяем outputs
	if resp.Outputs["duration_ms"] == nil {
		t.Error("outputs should contain duration_ms")
	}
}

func TestDelayStep_Execute_Seconds(t *testing.T) {
	step := NewDelayStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"duration_sec": 1,
		},
	}

	start := time.Now()

	// Отменяем через 100ms
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	_, err := step.Execute(ctx, req)
	elapsed := time.Since(start)

	// Должна быть ошибка отмены
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, ErrStepCancelled) {
		t.Errorf("expected ErrStepCancelled, got %v", err)
	}

	// Проверяем, что отмена произошла быстро
	if elapsed > 200*time.Millisecond {
		t.Errorf("cancellation took too long: %v", elapsed)
	}
}

func TestDelayStep_InvalidConfig(t *testing.T) {
	step := NewDelayStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{}, // Нет duration
	}

	_, err := step.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing duration")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

// HTTP Step Tests

func TestHTTPStep_Type(t *testing.T) {
	step := NewHTTPStep()
	if step.Type() != "http" {
		t.Errorf("expected 'http', got %s", step.Type())
	}
}

func TestHTTPStep_GET(t *testing.T) {
	// Создаём тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"data":   []int{1, 2, 3},
		})
	}))
	defer server.Close()

	step := NewHTTPStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL,
		},
	}

	resp, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем status_code
	if resp.Outputs["status_code"] != 200 {
		t.Errorf("expected status_code 200, got %v", resp.Outputs["status_code"])
	}

	// Проверяем body
	body, ok := resp.Outputs["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected body to be map, got %T", resp.Outputs["body"])
	}
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", body["status"])
	}
}

func TestHTTPStep_POST_JSON(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json")
		}

		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": 123})
	}))
	defer server.Close()

	step := NewHTTPStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"method": "POST",
			"url":    server.URL,
			"body": map[string]any{
				"name":  "test",
				"value": 42,
			},
		},
	}

	resp, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем status_code
	if resp.Outputs["status_code"] != 201 {
		t.Errorf("expected status_code 201, got %v", resp.Outputs["status_code"])
	}

	// Проверяем, что body был отправлен
	if receivedBody["name"] != "test" {
		t.Errorf("expected name 'test', got %v", receivedBody["name"])
	}
}

func TestHTTPStep_WithHeaders(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	step := NewHTTPStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"method": "GET",
			"url":    server.URL,
			"headers": map[string]any{
				"Authorization": "Bearer secret123",
			},
		},
	}

	_, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer secret123" {
		t.Errorf("expected auth header, got %s", receivedAuth)
	}
}

func TestHTTPStep_InvalidConfig(t *testing.T) {
	step := NewHTTPStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{}, // Нет URL
	}

	_, err := step.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestHTTPStep_Cancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	step := NewHTTPStep()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"url": server.URL,
		},
	}

	_, err := step.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, ErrStepCancelled) {
		t.Errorf("expected ErrStepCancelled, got %v", err)
	}
}

// Transform Step Tests

func TestTransformStep_Type(t *testing.T) {
	step := NewTransformStep()
	if step.Type() != "transform" {
		t.Errorf("expected 'transform', got %s", step.Type())
	}
}

func TestTransformStep_Execute(t *testing.T) {
	step := NewTransformStep()
	ctx := context.Background()

	// Создаём контекст с данными предыдущего шага
	tmplCtx := engine.NewContext(map[string]any{
		"multiplier": 2,
	})
	tmplCtx.AddStepResult("fetch", map[string]any{
		"items": []any{
			map[string]any{"id": 1, "name": "a"},
			map[string]any{"id": 2, "name": "b"},
		},
		"count": 2,
	}, "SUCCEEDED")

	req := &Request{
		StepID:          "transform_test",
		TemplateContext: tmplCtx,
		Config: map[string]any{
			"mappings": map[string]any{
				"total":  "{{ .Steps.fetch.Outputs.count }}",
				"status": "{{ .Steps.fetch.Status }}",
			},
		},
	}

	resp, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем результаты
	if resp.Outputs["status"] != "SUCCEEDED" {
		t.Errorf("expected status SUCCEEDED, got %v", resp.Outputs["status"])
	}

	// total парсится как int64 из "2"
	if resp.Outputs["total"] != int64(2) {
		t.Errorf("expected total 2, got %v (type %T)", resp.Outputs["total"], resp.Outputs["total"])
	}
}

func TestTransformStep_EmptyMappings(t *testing.T) {
	step := NewTransformStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{},
	}

	resp, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Outputs) != 0 {
		t.Errorf("expected empty outputs, got %v", resp.Outputs)
	}
}

func TestTransformStep_Cancellation(t *testing.T) {
	step := NewTransformStep()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Сразу отменяем

	req := &Request{
		StepID: "test",
		Config: map[string]any{
			"mappings": map[string]any{
				"test": "value",
			},
		},
	}

	_, err := step.Execute(ctx, req)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, ErrStepCancelled) {
		t.Errorf("expected ErrStepCancelled, got %v", err)
	}
}

// Parallel Step Tests

func TestParallelStep_Type(t *testing.T) {
	step := NewParallelStep()
	if step.Type() != "parallel" {
		t.Errorf("expected 'parallel', got %s", step.Type())
	}
}

func TestParallelStep_Execute(t *testing.T) {
	step := NewParallelStep()
	ctx := context.Background()

	req := &Request{
		StepID: "test",
		Config: map[string]any{},
	}

	// Parallel step просто возвращает пустой результат
	resp, err := step.Execute(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response should not be nil")
	}
	if len(resp.Outputs) != 0 {
		t.Errorf("expected empty outputs, got %v", resp.Outputs)
	}
}

func TestAggregateParallelOutputs(t *testing.T) {
	branchOutputs := map[string]map[string]map[string]any{
		"branch_a": {
			"step1": {"data": "a1"},
			"step2": {"data": "a2"},
		},
		"branch_b": {
			"step1": {"data": "b1"},
		},
	}

	result := AggregateParallelOutputs(branchOutputs)

	// Проверяем структуру
	branchA, ok := result["branch_a"].(map[string]any)
	if !ok {
		t.Fatal("expected branch_a to be map")
	}
	if branchA["step1"].(map[string]any)["data"] != "a1" {
		t.Error("expected data 'a1'")
	}
	if branchA["step2"].(map[string]any)["data"] != "a2" {
		t.Error("expected data 'a2'")
	}

	branchB, ok := result["branch_b"].(map[string]any)
	if !ok {
		t.Fatal("expected branch_b to be map")
	}
	if branchB["step1"].(map[string]any)["data"] != "b1" {
		t.Error("expected data 'b1'")
	}
}

func TestExtractBranchOutputs(t *testing.T) {
	outputs := map[string]any{
		"branch_a": map[string]any{
			"step1": map[string]any{"data": "test"},
		},
	}

	// Существующая ветка
	branch := ExtractBranchOutputs(outputs, "branch_a")
	if branch == nil {
		t.Fatal("expected branch_a outputs")
	}

	// Несуществующая ветка
	branch = ExtractBranchOutputs(outputs, "nonexistent")
	if branch != nil {
		t.Error("expected nil for nonexistent branch")
	}
}

func TestExtractStepOutputs(t *testing.T) {
	outputs := map[string]any{
		"branch_a": map[string]any{
			"step1": map[string]any{"data": "test"},
		},
	}

	// Существующий шаг
	step := ExtractStepOutputs(outputs, "branch_a", "step1")
	if step == nil {
		t.Fatal("expected step1 outputs")
	}
	if step["data"] != "test" {
		t.Errorf("expected data 'test', got %v", step["data"])
	}

	// Несуществующий шаг
	step = ExtractStepOutputs(outputs, "branch_a", "nonexistent")
	if step != nil {
		t.Error("expected nil for nonexistent step")
	}
}

// Helper Functions Tests

func TestGetConfigHelpers(t *testing.T) {
	config := map[string]any{
		"string_val":     "test",
		"int_val":        42,
		"float_val":      3.14,
		"bool_val":       true,
		"map_val":        map[string]any{"key": "value"},
		"string_map_val": map[string]string{"key": "value"},
	}

	// GetConfigString
	if GetConfigString(config, "string_val") != "test" {
		t.Error("GetConfigString failed")
	}
	if GetConfigString(config, "missing") != "" {
		t.Error("GetConfigString should return empty for missing")
	}

	// GetConfigInt
	if GetConfigInt(config, "int_val") != 42 {
		t.Error("GetConfigInt failed for int")
	}
	if GetConfigInt(config, "float_val") != 3 {
		t.Error("GetConfigInt failed for float")
	}
	if GetConfigInt(config, "missing") != 0 {
		t.Error("GetConfigInt should return 0 for missing")
	}

	// GetConfigBool
	if !GetConfigBool(config, "bool_val", false) {
		t.Error("GetConfigBool failed")
	}
	if !GetConfigBool(config, "missing", true) {
		t.Error("GetConfigBool should return default for missing")
	}

	// GetConfigMap
	m := GetConfigMap(config, "map_val")
	if m == nil || m["key"] != "value" {
		t.Error("GetConfigMap failed")
	}

	// GetConfigMapString
	ms := GetConfigMapString(config, "string_map_val")
	if ms == nil || ms["key"] != "value" {
		t.Error("GetConfigMapString failed for string map")
	}

	// GetConfigMapString с map[string]any
	ms = GetConfigMapString(config, "map_val")
	if ms == nil || ms["key"] != "value" {
		t.Error("GetConfigMapString failed for any map")
	}
}
