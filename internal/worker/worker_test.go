package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shaiso/Automata/internal/domain"
)

// --- HTTPExecutor Tests ---

func TestHTTPExecutor_GET_Success(t *testing.T) {
	// Создаём mock сервер, возвращающий JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("X-Custom", "test-value")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"result": "ok"})
	}))
	defer server.Close()

	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"method": "GET",
			"url":    server.URL,
		},
	}

	result, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected execution error: %s", result.Error)
	}

	// Проверяем status_code
	if result.Outputs["status_code"] != http.StatusOK {
		t.Errorf("expected status 200, got %v", result.Outputs["status_code"])
	}

	// Проверяем headers
	headers, ok := result.Outputs["headers"].(map[string]string)
	if !ok {
		t.Fatal("headers should be map[string]string")
	}
	if headers["X-Custom"] != "test-value" {
		t.Errorf("expected X-Custom header, got %v", headers["X-Custom"])
	}

	// Проверяем body (должен быть распарсен как JSON)
	body, ok := result.Outputs["body"].(map[string]any)
	if !ok {
		t.Fatalf("body should be map, got %T", result.Outputs["body"])
	}
	if body["result"] != "ok" {
		t.Errorf("expected result=ok, got %v", body["result"])
	}
}

func TestHTTPExecutor_POST_WithBody(t *testing.T) {
	var receivedBody map[string]any
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		receivedContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"id": "123"})
	}))
	defer server.Close()

	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"method": "POST",
			"url":    server.URL,
			"body":   map[string]any{"name": "test"},
			"headers": map[string]any{
				"Authorization": "Bearer token123",
			},
		},
	}

	result, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("unexpected execution error: %s", result.Error)
	}

	// Проверяем что body получен сервером
	if receivedBody["name"] != "test" {
		t.Errorf("server should receive body, got %v", receivedBody)
	}
	if receivedContentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedContentType)
	}
	if result.Outputs["status_code"] != http.StatusCreated {
		t.Errorf("expected status 201, got %v", result.Outputs["status_code"])
	}
}

func TestHTTPExecutor_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal"}`))
	}))
	defer server.Close()

	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"url": server.URL,
		},
	}

	result, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("HTTP errors should not be infrastructure errors: %v", err)
	}

	// Error должен быть заполнен
	if result.Error == "" {
		t.Error("expected execution error for 500")
	}

	// Outputs всё равно заполнены
	if result.Outputs["status_code"] != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %v", result.Outputs["status_code"])
	}
}

func TestHTTPExecutor_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"url":         server.URL,
			"timeout_sec": 0.1, // 100ms — сервер не успеет ответить
		},
	}

	_, err := executor.Execute(context.Background(), task)
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestHTTPExecutor_MissingURL(t *testing.T) {
	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID:      uuid.New(),
		Payload: map[string]any{"method": "GET"},
	}

	_, err := executor.Execute(context.Background(), task)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestHTTPExecutor_DefaultMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	executor := &HTTPExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"url": server.URL,
			// method не указан — должен быть GET
		},
	}

	_, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedMethod != http.MethodGet {
		t.Errorf("expected GET by default, got %s", receivedMethod)
	}
}

// --- DelayExecutor Tests ---

func TestDelayExecutor_Success(t *testing.T) {
	executor := &DelayExecutor{}
	task := &domain.Task{
		ID:      uuid.New(),
		Payload: map[string]any{"duration_sec": 0.05}, // 50ms
	}

	start := time.Now()
	result, err := executor.Execute(context.Background(), task)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs["delayed_sec"] != 0.05 {
		t.Errorf("expected delayed_sec=0.05, got %v", result.Outputs["delayed_sec"])
	}
	if elapsed < 40*time.Millisecond {
		t.Error("should have waited at least 40ms")
	}
}

func TestDelayExecutor_ContextCancel(t *testing.T) {
	executor := &DelayExecutor{}
	task := &domain.Task{
		ID:      uuid.New(),
		Payload: map[string]any{"duration_sec": 10.0}, // 10 секунд
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Отменяем сразу

	_, err := executor.Execute(ctx, task)
	if err == nil {
		t.Error("expected context canceled error")
	}
}

func TestDelayExecutor_DefaultDuration(t *testing.T) {
	executor := &DelayExecutor{}
	task := &domain.Task{
		ID:      uuid.New(),
		Payload: map[string]any{}, // нет duration_sec
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := executor.Execute(ctx, task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs["delayed_sec"] != 1.0 {
		t.Errorf("expected default 1.0, got %v", result.Outputs["delayed_sec"])
	}
}

// --- TransformExecutor Tests ---

func TestTransformExecutor_Success(t *testing.T) {
	executor := &TransformExecutor{}
	task := &domain.Task{
		ID: uuid.New(),
		Payload: map[string]any{
			"key1": "value1",
			"key2": 42,
			"key3": true,
		},
	}

	result, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Outputs["key1"] != "value1" {
		t.Errorf("expected key1=value1, got %v", result.Outputs["key1"])
	}
	if result.Outputs["key2"] != 42 {
		t.Errorf("expected key2=42, got %v", result.Outputs["key2"])
	}
	if result.Outputs["key3"] != true {
		t.Errorf("expected key3=true, got %v", result.Outputs["key3"])
	}
}

func TestTransformExecutor_NilPayload(t *testing.T) {
	executor := &TransformExecutor{}
	task := &domain.Task{
		ID:      uuid.New(),
		Payload: nil,
	}

	result, err := executor.Execute(context.Background(), task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outputs == nil {
		t.Error("outputs should be empty map, not nil")
	}
	if len(result.Outputs) != 0 {
		t.Errorf("expected empty outputs, got %d items", len(result.Outputs))
	}
}

// --- Registry Tests ---

func TestNewRegistry_DefaultExecutors(t *testing.T) {
	r := NewRegistry()

	// Должны быть зарегистрированы http, delay, transform
	for _, stepType := range []string{"http", "delay", "transform"} {
		executor, err := r.Get(stepType)
		if err != nil {
			t.Errorf("expected executor for %s, got error: %v", stepType, err)
		}
		if executor == nil {
			t.Errorf("executor for %s should not be nil", stepType)
		}
	}
}

func TestRegistry_UnknownType(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("unknown")
	if err == nil {
		t.Error("expected error for unknown step type")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	// Регистрируем кастомный executor
	r.Register("custom", &DelayExecutor{})

	executor, err := r.Get("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Error("custom executor should be registered")
	}
}

// --- Backoff Tests ---

func TestCalculateBackoff_Exponential(t *testing.T) {
	policy := &domain.RetryPolicy{
		Backoff:        "exponential",
		InitialDelayMs: 1000,
		MaxDelayMs:     10000,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // capped at max
		{6, 10 * time.Second}, // stays at max
	}

	for _, tt := range tests {
		got := calculateBackoff(tt.attempt, policy)
		if got != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, got)
		}
	}
}

func TestCalculateBackoff_Fixed(t *testing.T) {
	policy := &domain.RetryPolicy{
		Backoff:        "fixed",
		InitialDelayMs: 2000,
		MaxDelayMs:     10000,
	}

	// Все попытки — одинаковая задержка
	for attempt := 1; attempt <= 5; attempt++ {
		got := calculateBackoff(attempt, policy)
		if got != 2*time.Second {
			t.Errorf("attempt %d: expected 2s, got %v", attempt, got)
		}
	}
}

func TestCalculateBackoff_NilPolicy(t *testing.T) {
	got := calculateBackoff(1, nil)
	if got != time.Second {
		t.Errorf("expected 1s default, got %v", got)
	}
}

func TestCalculateBackoff_ZeroValues(t *testing.T) {
	policy := &domain.RetryPolicy{
		Backoff: "exponential",
		// InitialDelayMs и MaxDelayMs = 0
	}

	got := calculateBackoff(1, policy)
	if got != time.Second {
		t.Errorf("expected 1s default for zero InitialDelayMs, got %v", got)
	}
}

func TestShouldRetryHTTPStatus(t *testing.T) {
	onStatus := []int{500, 502, 503}

	if !shouldRetryHTTPStatus(500, onStatus) {
		t.Error("500 should be retriable")
	}
	if !shouldRetryHTTPStatus(502, onStatus) {
		t.Error("502 should be retriable")
	}
	if shouldRetryHTTPStatus(400, onStatus) {
		t.Error("400 should not be retriable")
	}
	if shouldRetryHTTPStatus(404, onStatus) {
		t.Error("404 should not be retriable")
	}
}

// --- Worker Tests ---

func TestNew_DefaultConfig(t *testing.T) {
	w := New(Config{})

	if w.pollInterval != defaultPollInterval {
		t.Errorf("expected default poll interval %v, got %v", defaultPollInterval, w.pollInterval)
	}
	if w.batchSize != defaultBatchSize {
		t.Errorf("expected default batch size %d, got %d", defaultBatchSize, w.batchSize)
	}
	if w.registry == nil {
		t.Error("registry should be initialized")
	}
}

func TestNew_CustomConfig(t *testing.T) {
	w := New(Config{
		PollInterval: 5 * time.Second,
		BatchSize:    25,
	})

	if w.pollInterval != 5*time.Second {
		t.Errorf("expected poll interval 5s, got %v", w.pollInterval)
	}
	if w.batchSize != 25 {
		t.Errorf("expected batch size 25, got %d", w.batchSize)
	}
}

func TestWorker_IsStopped(t *testing.T) {
	w := New(Config{})

	if w.IsStopped() {
		t.Error("should not be stopped initially")
	}

	w.stoppedMu.Lock()
	w.stopped = true
	w.stoppedMu.Unlock()

	if !w.IsStopped() {
		t.Error("should be stopped")
	}
}

func TestNew_CustomRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register("custom", &DelayExecutor{})

	w := New(Config{
		Registry: r,
	})

	executor, err := w.registry.Get("custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executor == nil {
		t.Error("custom executor should be available")
	}
}
