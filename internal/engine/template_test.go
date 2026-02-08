package engine

import (
	"strings"
	"testing"
)

func TestNewContext(t *testing.T) {
	// С nil inputs
	ctx := NewContext(nil)
	if ctx.Inputs == nil {
		t.Error("Inputs should not be nil")
	}
	if ctx.Steps == nil {
		t.Error("Steps should not be nil")
	}

	// С inputs
	inputs := map[string]any{"key": "value"}
	ctx = NewContext(inputs)
	if ctx.Inputs["key"] != "value" {
		t.Error("Inputs should contain provided values")
	}
}

func TestContext_AddStepResult(t *testing.T) {
	ctx := NewContext(nil)

	outputs := map[string]any{"data": "test"}
	ctx.AddStepResult("step1", outputs, "SUCCEEDED")

	if ctx.Steps["step1"] == nil {
		t.Fatal("step1 should be in Steps")
	}
	if ctx.Steps["step1"].Status != "SUCCEEDED" {
		t.Error("status should be SUCCEEDED")
	}
	if ctx.Steps["step1"].Outputs["data"] != "test" {
		t.Error("outputs should contain data")
	}

	// С nil outputs
	ctx.AddStepResult("step2", nil, "FAILED")
	if ctx.Steps["step2"].Outputs == nil {
		t.Error("Outputs should not be nil even when passed nil")
	}
}

func TestRender_SimpleInput(t *testing.T) {
	ctx := NewContext(map[string]any{
		"name": "test",
		"count": 42,
	})

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "string input",
			template: "Hello, {{ .Inputs.name }}!",
			expected: "Hello, test!",
		},
		{
			name:     "number input",
			template: "Count: {{ .Inputs.count }}",
			expected: "Count: 42",
		},
		{
			name:     "no template",
			template: "Plain text",
			expected: "Plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRender_StepOutput(t *testing.T) {
	ctx := NewContext(nil)
	ctx.AddStepResult("fetch", map[string]any{
		"data": map[string]any{
			"items": []any{"a", "b", "c"},
			"count": 3,
		},
	}, "SUCCEEDED")

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "access step output",
			template: "{{ .Steps.fetch.Status }}",
			expected: "SUCCEEDED",
		},
		{
			name:     "nested access",
			template: "{{ .Steps.fetch.Outputs.data.count }}",
			expected: "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRender_TemplateFunctions(t *testing.T) {
	ctx := NewContext(map[string]any{
		"text": "Hello World",
		"list": []string{"a", "b", "c"},
	})

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "lower",
			template: "{{ lower .Inputs.text }}",
			expected: "hello world",
		},
		{
			name:     "upper",
			template: "{{ upper .Inputs.text }}",
			expected: "HELLO WORLD",
		},
		{
			name:     "contains",
			template: "{{ contains .Inputs.text \"World\" }}",
			expected: "true",
		},
		{
			name:     "hasPrefix",
			template: "{{ hasPrefix .Inputs.text \"Hello\" }}",
			expected: "true",
		},
		{
			name:     "default with value",
			template: "{{ default \"fallback\" .Inputs.text }}",
			expected: "Hello World",
		},
		{
			name:     "default with nil",
			template: "{{ default \"fallback\" .Inputs.missing }}",
			expected: "fallback",
		},
		{
			name:     "json",
			template: `{{ json .Inputs.list }}`,
			expected: `["a","b","c"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	ctx := NewContext(nil)

	// Некорректный синтаксис
	_, err := Render("{{ .Invalid syntax", ctx)
	if err == nil {
		t.Error("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "template parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestRenderValue(t *testing.T) {
	ctx := NewContext(map[string]any{"name": "test"})

	tests := []struct {
		name     string
		value    any
		expected any
	}{
		{
			name:     "nil",
			value:    nil,
			expected: nil,
		},
		{
			name:     "string without template",
			value:    "plain",
			expected: "plain",
		},
		{
			name:     "string with template",
			value:    "Hello, {{ .Inputs.name }}",
			expected: "Hello, test",
		},
		{
			name:     "int",
			value:    42,
			expected: 42,
		},
		{
			name:     "bool",
			value:    true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderValue(tt.value, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRenderValue_Map(t *testing.T) {
	ctx := NewContext(map[string]any{
		"name": "test",
		"url":  "https://example.com",
	})

	value := map[string]any{
		"method": "POST",
		"url":    "{{ .Inputs.url }}/api",
		"body": map[string]any{
			"name": "{{ .Inputs.name }}",
		},
	}

	result, err := RenderValue(value, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	if resultMap["url"] != "https://example.com/api" {
		t.Errorf("expected rendered url, got %v", resultMap["url"])
	}

	body, ok := resultMap["body"].(map[string]any)
	if !ok {
		t.Fatalf("expected body to be map")
	}
	if body["name"] != "test" {
		t.Errorf("expected rendered name, got %v", body["name"])
	}
}

func TestRenderValue_Slice(t *testing.T) {
	ctx := NewContext(map[string]any{"prefix": "item"})

	value := []any{
		"{{ .Inputs.prefix }}_1",
		"{{ .Inputs.prefix }}_2",
		42,
	}

	result, err := RenderValue(value, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultSlice, ok := result.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", result)
	}

	if len(resultSlice) != 3 {
		t.Errorf("expected 3 items, got %d", len(resultSlice))
	}
	if resultSlice[0] != "item_1" {
		t.Errorf("expected item_1, got %v", resultSlice[0])
	}
	if resultSlice[1] != "item_2" {
		t.Errorf("expected item_2, got %v", resultSlice[1])
	}
	if resultSlice[2] != 42 {
		t.Errorf("expected 42, got %v", resultSlice[2])
	}
}

func TestRenderConfig(t *testing.T) {
	ctx := NewContext(map[string]any{
		"api_url": "https://api.example.com",
		"token":   "secret123",
	})

	config := map[string]any{
		"method": "GET",
		"url":    "{{ .Inputs.api_url }}/users",
		"headers": map[string]any{
			"Authorization": "Bearer {{ .Inputs.token }}",
		},
	}

	result, err := RenderConfig(config, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["url"] != "https://api.example.com/users" {
		t.Errorf("expected rendered url")
	}

	headers, ok := result["headers"].(map[string]any)
	if !ok {
		t.Fatal("expected headers to be map")
	}
	if headers["Authorization"] != "Bearer secret123" {
		t.Errorf("expected rendered auth header")
	}
}

func TestRenderConfig_Nil(t *testing.T) {
	ctx := NewContext(nil)

	result, err := RenderConfig(nil, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result should not be nil")
	}
	if len(result) != 0 {
		t.Error("result should be empty")
	}
}

func TestRenderCondition(t *testing.T) {
	ctx := NewContext(map[string]any{
		"enabled": true,
		"count":   5,
	})
	ctx.AddStepResult("check", map[string]any{
		"is_valid": true,
	}, "SUCCEEDED")

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{
			name:      "empty condition",
			condition: "",
			expected:  true,
		},
		{
			name:      "true condition",
			condition: ".Inputs.enabled",
			expected:  true,
		},
		{
			name:      "comparison true",
			condition: "gt .Inputs.count 3",
			expected:  true,
		},
		{
			name:      "comparison false",
			condition: "gt .Inputs.count 10",
			expected:  false,
		},
		{
			name:      "step output",
			condition: ".Steps.check.Outputs.is_valid",
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderCondition(tt.condition, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMustRender(t *testing.T) {
	ctx := NewContext(map[string]any{"name": "test"})

	result := MustRender("Hello, {{ .Inputs.name }}", ctx)
	if result != "Hello, test" {
		t.Errorf("expected 'Hello, test', got %q", result)
	}

	// Проверяем panic при ошибке
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid template")
		}
	}()
	MustRender("{{ .Invalid", ctx)
}
