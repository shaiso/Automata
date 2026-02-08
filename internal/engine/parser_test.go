package engine

import (
	"errors"
	"testing"

	"github.com/shaiso/Automata/internal/domain"
)

func TestValidate_EmptySteps(t *testing.T) {
	tests := []struct {
		name string
		spec *domain.FlowSpec
	}{
		{
			name: "nil spec",
			spec: nil,
		},
		{
			name: "empty steps",
			spec: &domain.FlowSpec{
				Steps: []domain.StepDef{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.spec)
			if !errors.Is(err, ErrEmptySteps) {
				t.Errorf("expected ErrEmptySteps, got %v", err)
			}
		})
	}
}

func TestValidate_EmptyStepID(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "", Type: "http"},
		},
	}

	err := Validate(spec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !errors.Is(vErr.Err, ErrEmptyStepID) {
		t.Errorf("expected ErrEmptyStepID, got %v", vErr.Err)
	}
}

func TestValidate_DuplicateStepID(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "step1", Type: "http"},
			{ID: "step1", Type: "delay"},
		},
	}

	err := Validate(spec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !errors.Is(vErr.Err, ErrDuplicateStepID) {
		t.Errorf("expected ErrDuplicateStepID, got %v", vErr.Err)
	}
}

func TestValidate_UnknownStepType(t *testing.T) {
	tests := []struct {
		name     string
		stepType string
	}{
		{name: "empty type", stepType: ""},
		{name: "unknown type", stepType: "unknown"},
		{name: "typo", stepType: "htpp"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &domain.FlowSpec{
				Steps: []domain.StepDef{
					{ID: "step1", Type: tt.stepType},
				},
			}

			err := Validate(spec)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("expected ValidationError, got %T", err)
			}
			if !errors.Is(vErr.Err, ErrUnknownStepType) {
				t.Errorf("expected ErrUnknownStepType, got %v", vErr.Err)
			}
		})
	}
}

func TestValidate_MissingDependency(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "step1", Type: "http"},
			{ID: "step2", Type: "delay", DependsOn: []string{"nonexistent"}},
		},
	}

	err := Validate(spec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !errors.Is(vErr.Err, ErrMissingDependency) {
		t.Errorf("expected ErrMissingDependency, got %v", vErr.Err)
	}
}

func TestValidate_SelfDependency(t *testing.T) {
	spec := &domain.FlowSpec{
		Steps: []domain.StepDef{
			{ID: "step1", Type: "http", DependsOn: []string{"step1"}},
		},
	}

	err := Validate(spec)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if !errors.Is(vErr.Err, ErrSelfDependency) {
		t.Errorf("expected ErrSelfDependency, got %v", vErr.Err)
	}
}

func TestValidate_ValidSpec(t *testing.T) {
	tests := []struct {
		name string
		spec *domain.FlowSpec
	}{
		{
			name: "single step",
			spec: &domain.FlowSpec{
				Steps: []domain.StepDef{
					{ID: "step1", Type: "http"},
				},
			},
		},
		{
			name: "chain of steps",
			spec: &domain.FlowSpec{
				Steps: []domain.StepDef{
					{ID: "step1", Type: "http"},
					{ID: "step2", Type: "delay", DependsOn: []string{"step1"}},
					{ID: "step3", Type: "transform", DependsOn: []string{"step2"}},
				},
			},
		},
		{
			name: "diamond dependency",
			spec: &domain.FlowSpec{
				Steps: []domain.StepDef{
					{ID: "A", Type: "http"},
					{ID: "B", Type: "http", DependsOn: []string{"A"}},
					{ID: "C", Type: "http", DependsOn: []string{"A"}},
					{ID: "D", Type: "http", DependsOn: []string{"B", "C"}},
				},
			},
		},
		{
			name: "all step types",
			spec: &domain.FlowSpec{
				Steps: []domain.StepDef{
					{ID: "http_step", Type: "http"},
					{ID: "delay_step", Type: "delay"},
					{ID: "transform_step", Type: "transform"},
					{
						ID:   "parallel_step",
						Type: "parallel",
						Branches: []domain.Branch{
							{
								ID: "branch_a",
								Steps: []domain.StepDef{
									{ID: "inner_step", Type: "http"},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.spec)
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestValidate_ParallelStep(t *testing.T) {
	t.Run("empty branches", func(t *testing.T) {
		spec := &domain.FlowSpec{
			Steps: []domain.StepDef{
				{ID: "parallel", Type: "parallel", Branches: []domain.Branch{}},
			},
		}

		err := Validate(spec)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %T", err)
		}
		if !errors.Is(vErr.Err, ErrEmptyBranches) {
			t.Errorf("expected ErrEmptyBranches, got %v", vErr.Err)
		}
	})

	t.Run("empty branch ID", func(t *testing.T) {
		spec := &domain.FlowSpec{
			Steps: []domain.StepDef{
				{
					ID:   "parallel",
					Type: "parallel",
					Branches: []domain.Branch{
						{ID: "", Steps: []domain.StepDef{{ID: "step", Type: "http"}}},
					},
				},
			},
		}

		err := Validate(spec)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %T", err)
		}
		if !errors.Is(vErr.Err, ErrEmptyBranchID) {
			t.Errorf("expected ErrEmptyBranchID, got %v", vErr.Err)
		}
	})

	t.Run("duplicate branch ID", func(t *testing.T) {
		spec := &domain.FlowSpec{
			Steps: []domain.StepDef{
				{
					ID:   "parallel",
					Type: "parallel",
					Branches: []domain.Branch{
						{ID: "branch", Steps: []domain.StepDef{{ID: "step1", Type: "http"}}},
						{ID: "branch", Steps: []domain.StepDef{{ID: "step2", Type: "http"}}},
					},
				},
			},
		}

		err := Validate(spec)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %T", err)
		}
		if !errors.Is(vErr.Err, ErrDuplicateBranchID) {
			t.Errorf("expected ErrDuplicateBranchID, got %v", vErr.Err)
		}
	})

	t.Run("empty branch steps", func(t *testing.T) {
		spec := &domain.FlowSpec{
			Steps: []domain.StepDef{
				{
					ID:   "parallel",
					Type: "parallel",
					Branches: []domain.Branch{
						{ID: "branch", Steps: []domain.StepDef{}},
					},
				},
			},
		}

		err := Validate(spec)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var vErr *ValidationError
		if !errors.As(err, &vErr) {
			t.Fatalf("expected ValidationError, got %T", err)
		}
		if !errors.Is(vErr.Err, ErrEmptyBranchSteps) {
			t.Errorf("expected ErrEmptyBranchSteps, got %v", vErr.Err)
		}
	})

	t.Run("valid parallel", func(t *testing.T) {
		spec := &domain.FlowSpec{
			Steps: []domain.StepDef{
				{
					ID:   "parallel",
					Type: "parallel",
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
								{ID: "step1", Type: "transform"},
							},
						},
					},
				},
			},
		}

		err := Validate(spec)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestIsValidStepType(t *testing.T) {
	validTypes := []string{"http", "delay", "transform", "parallel"}
	for _, typ := range validTypes {
		if !IsValidStepType(typ) {
			t.Errorf("expected %s to be valid", typ)
		}
	}

	invalidTypes := []string{"", "unknown", "HTTP", "Delay"}
	for _, typ := range invalidTypes {
		if IsValidStepType(typ) {
			t.Errorf("expected %s to be invalid", typ)
		}
	}
}

func TestGetValidStepTypes(t *testing.T) {
	types := GetValidStepTypes()
	if len(types) != 4 {
		t.Errorf("expected 4 types, got %d", len(types))
	}

	expected := map[string]bool{
		"http":      true,
		"delay":     true,
		"transform": true,
		"parallel":  true,
	}

	for _, typ := range types {
		if !expected[typ] {
			t.Errorf("unexpected type: %s", typ)
		}
	}
}
