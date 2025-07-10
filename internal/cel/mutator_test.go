package cel

import (
	"testing"

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewCELMutator(t *testing.T) {
	programs, err := CompileCELPrograms([]string{
		`annotation("test-key", "test-value")`,
		`label("env", "production")`,
	})
	if err != nil {
		t.Fatalf("Failed to compile programs: %v", err)
	}

	mutator := NewCELMutator(programs)

	if mutator == nil {
		t.Fatal("NewCELMutator returned nil")
	}

	if len(mutator.programs) != 2 {
		t.Errorf("Expected 2 programs, got %d", len(mutator.programs))
	}
}

func TestCELMutator_Mutate(t *testing.T) {
	tests := []struct {
		name                string
		expressions         []string
		initialLabels       map[string]string
		initialAnnotations  map[string]string
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
		expectErr           bool
		errMsg              string
	}{
		{
			name: "single annotation mutation",
			expressions: []string{
				`annotation("tekton.dev/pipeline", "my-pipeline")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels:     nil,
			expectedAnnotations: map[string]string{
				"tekton.dev/pipeline": "my-pipeline",
			},
			expectErr: false,
		},
		{
			name: "single label mutation",
			expressions: []string{
				`label("environment", "production")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"environment": "production",
			},
			expectedAnnotations: nil,
			expectErr:           false,
		},
		{
			name: "multiple mutations from single expression",
			expressions: []string{
				`[annotation("owner", "team-a"), label("env", "prod")]`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"env": "prod",
			},
			expectedAnnotations: map[string]string{
				"owner": "team-a",
			},
			expectErr: false,
		},
		{
			name: "multiple expressions",
			expressions: []string{
				`annotation("pipeline", "test-pipeline")`,
				`label("stage", "testing")`,
				`annotation("version", "1.0")`,
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectedLabels: map[string]string{
				"stage": "testing",
			},
			expectedAnnotations: map[string]string{
				"pipeline": "test-pipeline",
				"version":  "1.0",
			},
			expectErr: false,
		},
		{
			name: "merge with existing labels and annotations",
			expressions: []string{
				`annotation("new-annotation", "new-value")`,
				`label("new-label", "new-value")`,
			},
			initialLabels: map[string]string{
				"existing-label": "existing-value",
			},
			initialAnnotations: map[string]string{
				"existing-annotation": "existing-value",
			},
			expectedLabels: map[string]string{
				"existing-label": "existing-value",
				"new-label":      "new-value",
			},
			expectedAnnotations: map[string]string{
				"existing-annotation": "existing-value",
				"new-annotation":      "new-value",
			},
			expectErr: false,
		},
		{
			name: "overwrite existing values",
			expressions: []string{
				`annotation("existing-annotation", "updated-value")`,
				`label("existing-label", "updated-value")`,
			},
			initialLabels: map[string]string{
				"existing-label": "old-value",
			},
			initialAnnotations: map[string]string{
				"existing-annotation": "old-value",
			},
			expectedLabels: map[string]string{
				"existing-label": "updated-value",
			},
			expectedAnnotations: map[string]string{
				"existing-annotation": "updated-value",
			},
			expectErr: false,
		},
		{
			name: "runtime error in expression",
			expressions: []string{
				`annotation("", "test-value")`, // empty key should cause error
			},
			initialLabels:      nil,
			initialAnnotations: nil,
			expectErr:          true,
			errMsg:             "annotation key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create PipelineRun with initial state
			pipelineRun := &tekv1.PipelineRun{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pipeline",
					Namespace:   "test-namespace",
					Labels:      copyMap(tt.initialLabels),
					Annotations: copyMap(tt.initialAnnotations),
				},
				Spec: tekv1.PipelineRunSpec{
					PipelineRef: &tekv1.PipelineRef{
						Name: "test-pipeline",
					},
				},
			}

			// Compile programs and create mutator
			programs, err := CompileCELPrograms(tt.expressions)
			if err != nil {
				t.Fatalf("Failed to compile expressions: %v", err)
			}

			mutator := NewCELMutator(programs)

			// Apply mutations
			err = mutator.Mutate(pipelineRun)

			// Check for expected errors
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("Expected error message %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Verify labels
			if !mapsEqual(pipelineRun.Labels, tt.expectedLabels) {
				t.Errorf("Labels mismatch:\nexpected: %v\ngot: %v", tt.expectedLabels, pipelineRun.Labels)
			}

			// Verify annotations
			if !mapsEqual(pipelineRun.Annotations, tt.expectedAnnotations) {
				t.Errorf("Annotations mismatch:\nexpected: %v\ngot: %v", tt.expectedAnnotations, pipelineRun.Annotations)
			}
		})
	}
}

func TestCELMutator_Mutate_NilPipelineRun(t *testing.T) {
	programs, err := CompileCELPrograms([]string{
		`annotation("test-key", "test-value")`,
	})
	if err != nil {
		t.Fatalf("Failed to compile programs: %v", err)
	}

	mutator := NewCELMutator(programs)
	err = mutator.Mutate(nil)

	if err == nil {
		t.Error("Expected error for nil PipelineRun but got none")
	}
}

func TestCELMutator_EmptyPrograms(t *testing.T) {
	mutator := NewCELMutator([]*CompiledProgram{})

	pipelineRun := &tekv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "test-namespace",
		},
	}

	err := mutator.Mutate(pipelineRun)
	if err != nil {
		t.Errorf("Unexpected error with empty programs: %v", err)
	}

	// Should not crash or modify the PipelineRun
	if pipelineRun.Labels != nil {
		t.Error("Labels should remain nil")
	}
	if pipelineRun.Annotations != nil {
		t.Error("Annotations should remain nil")
	}
}

// Helper functions for testing

func copyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(substr) > 0 && indexString(s, substr) >= 0))
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
