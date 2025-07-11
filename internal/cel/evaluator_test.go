package cel

import (
	"strings"
	"testing"

	tekv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCompiledProgram_Evaluate_TypeSafety(t *testing.T) {
	// Create a sample PipelineRun
	pipelineRun := &tekv1.PipelineRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pipeline",
			Namespace: "test-namespace",
		},
		Spec: tekv1.PipelineRunSpec{
			PipelineRef: &tekv1.PipelineRef{
				Name: "test-pipeline",
			},
		},
	}

	tests := []struct {
		name        string
		expression  string
		pipelineRun *tekv1.PipelineRun
		expected    []MutationRequest
		expectErr   bool
		errMsg      string
	}{
		{
			name:        "valid single annotation",
			expression:  `annotation("test-key", "test-value")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "test-key", Value: "test-value"},
			},
		},
		{
			name:        "valid single label",
			expression:  `label("env", "production")`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeLabel, Key: "env", Value: "production"},
			},
		},
		{
			name:        "valid list of mutations",
			expression:  `[annotation("key1", "value1"), label("key2", "value2")]`,
			pipelineRun: pipelineRun,
			expected: []MutationRequest{
				{Type: MutationTypeAnnotation, Key: "key1", Value: "value1"},
				{Type: MutationTypeLabel, Key: "key2", Value: "value2"},
			},
		},
		{
			name:        "nil PipelineRun",
			expression:  `annotation("test-key", "test-value")`,
			pipelineRun: nil,
			expectErr:   true,
			errMsg:      "pipelineRun cannot be nil",
		},
		{
			name:        "runtime error - empty key",
			expression:  `annotation("", "test-value")`,
			pipelineRun: pipelineRun,
			expectErr:   true,
			errMsg:      "annotation key cannot be empty",
		},
		{
			name:        "runtime error - empty value causes validation failure",
			expression:  `annotation("test-key", "")`,
			pipelineRun: pipelineRun,
			expectErr:   true,
			errMsg:      "mutation value cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			programs, err := CompileCELPrograms([]string{tt.expression})
			if err != nil {
				t.Fatalf("failed to compile expression: %v", err)
			}

			if len(programs) != 1 {
				t.Fatalf("expected 1 program, got %d", len(programs))
			}

			mutations, err := programs[0].Evaluate(tt.pipelineRun)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error message %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(mutations) != len(tt.expected) {
				t.Errorf("expected %d mutations, got %d", len(tt.expected), len(mutations))
				return
			}

			for i, expected := range tt.expected {
				if mutations[i].Type != expected.Type {
					t.Errorf("mutation %d: expected type %v, got %v", i, expected.Type, mutations[i].Type)
				}
				if mutations[i].Key != expected.Key {
					t.Errorf("mutation %d: expected key %v, got %v", i, expected.Key, mutations[i].Key)
				}
				if mutations[i].Value != expected.Value {
					t.Errorf("mutation %d: expected value %v, got %v", i, expected.Value, mutations[i].Value)
				}
			}
		})
	}
}
