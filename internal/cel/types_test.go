package cel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMutationType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		mt   MutationType
		want bool
	}{
		{"valid annotation", MutationTypeAnnotation, true},
		{"valid label", MutationTypeLabel, true},
		{"invalid type", MutationType("invalid"), false},
		{"empty type", MutationType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.IsValid(); got != tt.want {
				t.Errorf("MutationType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMutationType_JSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  MutationType
	}{
		{
			name:      "valid annotation",
			input:     `"annotation"`,
			expectErr: false,
			expected:  MutationTypeAnnotation,
		},
		{
			name:      "valid label",
			input:     `"label"`,
			expectErr: false,
			expected:  MutationTypeLabel,
		},
		{
			name:      "invalid type",
			input:     `"invalid"`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mt MutationType
			err := json.Unmarshal([]byte(tt.input), &mt)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if mt != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, mt)
			}
		})
	}
}

func TestMutationRequest_Validate(t *testing.T) {
	tests := []struct {
		name      string
		request   MutationRequest
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid annotation",
			request: MutationRequest{
				Type:  MutationTypeAnnotation,
				Key:   "test-key",
				Value: "test-value",
			},
			expectErr: false,
		},
		{
			name: "valid label",
			request: MutationRequest{
				Type:  MutationTypeLabel,
				Key:   "env",
				Value: "production",
			},
			expectErr: false,
		},
		{
			name: "invalid type",
			request: MutationRequest{
				Type:  MutationType("invalid"),
				Key:   "test-key",
				Value: "test-value",
			},
			expectErr: true,
			errMsg:    "invalid mutation type: invalid",
		},
		{
			name: "empty key",
			request: MutationRequest{
				Type:  MutationTypeAnnotation,
				Key:   "",
				Value: "test-value",
			},
			expectErr: true,
			errMsg:    "mutation key cannot be empty",
		},
		{
			name: "empty value",
			request: MutationRequest{
				Type:  MutationTypeAnnotation,
				Key:   "test-key",
				Value: "",
			},
			expectErr: true,
			errMsg:    "mutation value cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()

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
			}
		})
	}
}

func TestMutationRequest_Usage(t *testing.T) {
	// Test typical usage patterns for MutationRequest
	annotation := MutationRequest{
		Type:  MutationTypeAnnotation,
		Key:   "tekton.dev/pipeline",
		Value: "my-pipeline",
	}

	if err := annotation.Validate(); err != nil {
		t.Errorf("Valid annotation should not produce error: %v", err)
	}

	label := MutationRequest{
		Type:  MutationTypeLabel,
		Key:   "environment",
		Value: "production",
	}

	if err := label.Validate(); err != nil {
		t.Errorf("Valid label should not produce error: %v", err)
	}

	// Test JSON marshaling/unmarshaling
	data, err := json.Marshal(annotation)
	if err != nil {
		t.Errorf("Failed to marshal annotation: %v", err)
	}

	var unmarshaled MutationRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal annotation: %v", err)
	}

	if unmarshaled != annotation {
		t.Errorf("Unmarshaled annotation doesn't match original: got %+v, want %+v", unmarshaled, annotation)
	}
} 
