package cel

import (
	"strings"
	"testing"
)

func TestCompileCELPrograms_TypeSafety(t *testing.T) {
	tests := []struct {
		name        string
		expressions []string
		expectErr   bool
		errMsg      string
	}{
		{
			name: "valid type-safe expressions",
			expressions: []string{
				`annotation("test-key", "test-value")`,
				`label("env", "production")`,
				`[annotation("key1", "value1"), label("key2", "value2")]`,
				`priority("high")`,
			},
			expectErr: false,
		},
		{
			name:        "empty expressions list",
			expressions: []string{},
			expectErr:   true,
			errMsg:      "expressions list cannot be empty",
		},
		{
			name: "empty expression",
			expressions: []string{
				`annotation("test-key", "test-value")`,
				"", // empty expression
			},
			expectErr: true,
			errMsg:    "expression 1 cannot be empty",
		},
		{
			name: "invalid function",
			expressions: []string{
				`invalid_function("test")`,
			},
			expectErr: true,
		},
		{
			name: "type error - missing arguments",
			expressions: []string{
				`annotation("test")`, // missing second argument
			},
			expectErr: true,
		},
		{
			name: "type error - wrong argument type",
			expressions: []string{
				`annotation(123, "test")`, // first argument should be string
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			programs, err := CompileCELPrograms(tt.expressions)

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

			if len(programs) != len(tt.expressions) {
				t.Errorf("expected %d programs, got %d", len(tt.expressions), len(programs))
			}

			// Test GetExpression method
			for i, program := range programs {
				if program.GetExpression() != tt.expressions[i] {
					t.Errorf("program %d: expected expression %q, got %q", i, tt.expressions[i], program.GetExpression())
				}
			}
		})
	}
}

func TestValidateExpressionReturnType(t *testing.T) {
	// Create a simple CEL environment for testing
	env, err := createCELEnvironment()
	if err != nil {
		t.Fatalf("Failed to create CEL environment: %v", err)
	}

	tests := []struct {
		name        string
		expression  string
		expectValid bool
		description string
	}{
		{
			name:        "valid single annotation",
			expression:  `annotation("key", "value")`,
			expectValid: true,
			description: "Returns map<string, any> representing MutationRequest",
		},
		{
			name:        "valid annotation list",
			expression:  `[annotation("key1", "value1"), annotation("key2", "value2")]`,
			expectValid: true,
			description: "Returns list<map<string, any>> representing []MutationRequest",
		},
		{
			name:        "valid mixed list",
			expression:  `[annotation("key1", "value1"), label("key2", "value2")]`,
			expectValid: true,
			description: "Returns list<map<string, any>> with mixed mutation types",
		},
		{
			name:        "valid priority function",
			expression:  `priority("high")`,
			expectValid: true,
			description: "Returns map<string, any> representing priority MutationRequest",
		},
		{
			name:        "valid priority in list",
			expression:  `[priority("medium"), annotation("queue", "default")]`,
			expectValid: true,
			description: "Returns list<map<string, any>> with priority and annotation",
		},
		{
			name:        "invalid string return",
			expression:  `"just a string"`,
			expectValid: false,
			description: "Returns string instead of MutationRequest structure",
		},
		{
			name:        "invalid number return",
			expression:  `42`,
			expectValid: false,
			description: "Returns number instead of MutationRequest structure",
		},
		{
			name:        "invalid list of strings",
			expression:  `["string1", "string2"]`,
			expectValid: false,
			description: "Returns list<string> instead of list<map<string, any>>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			if issues != nil && issues.Err() != nil {
				if tt.expectValid {
					t.Errorf("Expression should compile but failed: %v", issues.Err())
				}
				return
			}

			// Validate the return type
			err := ValidateExpressionReturnType(ast)

			if tt.expectValid {
				if err != nil {
					t.Errorf("Expected valid return type but got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected invalid return type but validation passed")
				} else {
					t.Logf("Correctly caught invalid return type: %v", err)
				}
			}
		})
	}
}

func TestCompiledProgram_GetExpression(t *testing.T) {
	expression := `annotation("test-key", "test-value")`
	programs, err := CompileCELPrograms([]string{expression})
	if err != nil {
		t.Fatalf("failed to compile expression: %v", err)
	}

	if len(programs) != 1 {
		t.Fatalf("expected 1 program, got %d", len(programs))
	}

	if programs[0].GetExpression() != expression {
		t.Errorf("expected expression %q, got %q", expression, programs[0].GetExpression())
	}
}
