package steps

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// StepTypeHTTP — тип HTTP шага.
	StepTypeHTTP = "http"

	// Значения по умолчанию.
	defaultHTTPTimeout = 30 * time.Second
	maxResponseBody    = 10 * 1024 * 1024 // 10 MB
)

// Ключи конфигурации HTTP шага.
const (
	configMethod          = "method"
	configURL             = "url"
	configHeaders         = "headers"
	configBody            = "body"
	configFollowRedirects = "follow_redirects"
	configValidateSSL     = "validate_ssl"
	configTimeoutSec      = "timeout_sec"
)

// HTTPStep — шаг HTTP запроса.
//
// Выполняет HTTP запрос к внешнему API и возвращает результат.
//
// Конфигурация:
//
//	{
//	    "method": "POST",
//	    "url": "https://api.example.com/data",
//	    "headers": {
//	        "Content-Type": "application/json",
//	        "Authorization": "Bearer {{ .Inputs.token }}"
//	    },
//	    "body": {
//	        "data": "{{ .Steps.fetch.Outputs.items }}"
//	    },
//	    "follow_redirects": true,
//	    "validate_ssl": true,
//	    "timeout_sec": 30
//	}
//
// Outputs:
//
//	{
//	    "status_code": 200,
//	    "headers": {"Content-Type": "application/json", ...},
//	    "body": {...}  // parsed JSON or string
//	}
type HTTPStep struct {
	client *http.Client
}

// NewHTTPStep создаёт новый HTTPStep.
func NewHTTPStep() *HTTPStep {
	return &HTTPStep{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Type возвращает тип шага.
func (s *HTTPStep) Type() string {
	return StepTypeHTTP
}

// Execute выполняет HTTP запрос.
func (s *HTTPStep) Execute(ctx context.Context, req *Request) (*Response, error) {
	// Парсим конфигурацию
	cfg, err := s.parseConfig(req.Config)
	if err != nil {
		return nil, err
	}

	// Создаём HTTP клиент с нужными настройками
	client := s.buildClient(cfg, req.Timeout)

	// Создаём запрос
	httpReq, err := s.buildRequest(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	// Выполняем запрос
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("%w: %v", ErrStepCancelled, ctx.Err())
		}
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	// Читаем и парсим ответ
	return s.parseResponse(resp)
}

// httpConfig — распарсенная конфигурация HTTP шага.
type httpConfig struct {
	Method          string
	URL             string
	Headers         map[string]string
	Body            any
	FollowRedirects bool
	ValidateSSL     bool
	TimeoutSec      int
}

// parseConfig парсит конфигурацию HTTP шага.
func (s *HTTPStep) parseConfig(config map[string]any) (*httpConfig, error) {
	cfg := &httpConfig{
		Method:          GetConfigString(config, configMethod),
		URL:             GetConfigString(config, configURL),
		Headers:         GetConfigMapString(config, configHeaders),
		Body:            config[configBody],
		FollowRedirects: GetConfigBool(config, configFollowRedirects, true),
		ValidateSSL:     GetConfigBool(config, configValidateSSL, true),
		TimeoutSec:      GetConfigInt(config, configTimeoutSec),
	}

	// Валидация
	if cfg.URL == "" {
		return nil, fmt.Errorf("%w: %s: url is required", ErrInvalidConfig, StepTypeHTTP)
	}

	// Метод по умолчанию — GET
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	cfg.Method = strings.ToUpper(cfg.Method)

	// Инициализируем headers
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}

	return cfg, nil
}

// buildClient создаёт HTTP клиент с нужными настройками.
func (s *HTTPStep) buildClient(cfg *httpConfig, reqTimeout time.Duration) *http.Client {
	// Определяем таймаут
	timeout := defaultHTTPTimeout
	if cfg.TimeoutSec > 0 {
		timeout = time.Duration(cfg.TimeoutSec) * time.Second
	}
	if reqTimeout > 0 {
		timeout = reqTimeout
	}

	// Настройки TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !cfg.ValidateSSL,
	}

	// Настройка редиректов
	var checkRedirect func(*http.Request, []*http.Request) error
	if !cfg.FollowRedirects {
		checkRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
}

// buildRequest создаёт HTTP запрос.
func (s *HTTPStep) buildRequest(ctx context.Context, cfg *httpConfig) (*http.Request, error) {
	var bodyReader io.Reader

	// Подготавливаем body
	if cfg.Body != nil {
		bodyBytes, err := s.serializeBody(cfg.Body)
		if err != nil {
			return nil, fmt.Errorf("serialize body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)

		// Устанавливаем Content-Type, если не задан
		if _, hasContentType := cfg.Headers["Content-Type"]; !hasContentType {
			cfg.Headers["Content-Type"] = "application/json"
		}
	}

	// Создаём запрос
	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.URL, bodyReader)
	if err != nil {
		return nil, err
	}

	// Устанавливаем заголовки
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// serializeBody сериализует body в bytes.
func (s *HTTPStep) serializeBody(body any) ([]byte, error) {
	switch v := body.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return json.Marshal(v)
	}
}

// parseResponse парсит HTTP ответ в Response.
func (s *HTTPStep) parseResponse(resp *http.Response) (*Response, error) {
	// Читаем body с ограничением размера
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Парсим body
	var body any
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			// Если не удалось распарсить JSON, возвращаем как строку
			body = string(bodyBytes)
		}
	} else {
		body = string(bodyBytes)
	}

	// Преобразуем headers в map[string]string
	headers := make(map[string]string)
	for key := range resp.Header {
		headers[key] = resp.Header.Get(key)
	}

	outputs := map[string]any{
		"status_code": resp.StatusCode,
		"headers":     headers,
		"body":        body,
	}

	return &Response{Outputs: outputs}, nil
}

// HTTPError — ошибка HTTP запроса.
type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

// Error реализует интерфейс error.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Status)
}

// IsHTTPError проверяет, является ли ошибка HTTP ошибкой.
func IsHTTPError(err error) bool {
	_, ok := err.(*HTTPError)
	return ok
}
