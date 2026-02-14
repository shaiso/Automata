package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shaiso/Automata/internal/domain"
)

const defaultHTTPTimeout = 30 * time.Second

// HTTPExecutor — executor для шага типа "http".
//
// Выполняет HTTP-запрос на основе конфигурации из task.Payload.
//
// Config (из task.Payload):
//   - method (string): HTTP-метод (GET, POST, PUT, DELETE). Default: GET
//   - url (string): URL для запроса (обязательно)
//   - headers (map[string]any): HTTP-заголовки
//   - body (any): тело запроса (сериализуется в JSON)
//   - timeout_sec (number): таймаут запроса в секундах. Default: 30
//
// Outputs:
//   - status_code (int): HTTP-код ответа
//   - headers (map[string]string): заголовки ответа
//   - body (any): тело ответа (JSON или строка)
type HTTPExecutor struct{}

// Execute выполняет HTTP-запрос.
func (e *HTTPExecutor) Execute(ctx context.Context, task *domain.Task) (*ExecutionResult, error) {
	// Извлекаем конфигурацию
	method := getString(task.Payload, "method", "GET")
	url := getString(task.Payload, "url", "")
	if url == "" {
		return nil, fmt.Errorf("%w: url is required", ErrHTTPRequest)
	}

	timeout := getTimeout(task.Payload)

	// Таймаут
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Подготавливаем body
	var bodyReader io.Reader
	if body, ok := task.Payload["body"]; ok && body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("%w: marshal body: %v", ErrHTTPRequest, err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	// Создаём запрос
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("%w: create request: %v", ErrHTTPRequest, err)
	}

	// Устанавливаем заголовки
	setHeaders(req, task.Payload)

	// Content-Type по умолчанию для запросов с body
	if bodyReader != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Выполняем запрос
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHTTPRequest, err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrHTTPRequest, err)
	}

	// Формируем outputs
	outputs := buildOutputs(resp, respBody)

	// HTTP >= 400 — логическая ошибка (outputs сохраняются для retry по status_code)
	if resp.StatusCode >= 400 {
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
		return &ExecutionResult{
			Outputs: outputs,
			Error:   errMsg,
		}, nil
	}

	return &ExecutionResult{Outputs: outputs}, nil
}

// buildOutputs формирует outputs из HTTP-ответа.
func buildOutputs(resp *http.Response, body []byte) map[string]any {
	// Парсим заголовки
	headers := make(map[string]string, len(resp.Header))
	for key := range resp.Header {
		headers[key] = resp.Header.Get(key)
	}

	// Парсим body: пробуем JSON, иначе строка
	var parsedBody any
	if err := json.Unmarshal(body, &parsedBody); err != nil {
		parsedBody = string(body)
	}

	return map[string]any{
		"status_code": resp.StatusCode,
		"headers":     headers,
		"body":        parsedBody,
	}
}

// getString извлекает строку из map с default значением.
func getString(m map[string]any, key, defaultVal string) string {
	if val, ok := m[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// getTimeout извлекает таймаут из payload.
func getTimeout(payload map[string]any) time.Duration {
	if val, ok := payload["timeout_sec"]; ok {
		switch v := val.(type) {
		case float64:
			if v > 0 {
				return time.Duration(v * float64(time.Second))
			}
		case int:
			if v > 0 {
				return time.Duration(v) * time.Second
			}
		}
	}
	return defaultHTTPTimeout
}

// setHeaders устанавливает заголовки из payload.
func setHeaders(req *http.Request, payload map[string]any) {
	headers, ok := payload["headers"]
	if !ok || headers == nil {
		return
	}

	switch h := headers.(type) {
	case map[string]any:
		for key, val := range h {
			if s, ok := val.(string); ok {
				req.Header.Set(key, s)
			}
		}
	case map[string]string:
		for key, val := range h {
			req.Header.Set(key, val)
		}
	}
}

// truncate обрезает строку до указанной длины.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
