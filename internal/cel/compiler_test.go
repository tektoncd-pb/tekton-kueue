package cel

import (
	"testing"

	. "github.com/onsi/gomega"
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
			g := NewWithT(t)
			programs, err := CompileCELPrograms(tt.expressions)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				if tt.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
				}
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(programs).To(HaveLen(len(tt.expressions)))

			// Test GetExpression method
			for i, program := range programs {
				g.Expect(program.GetExpression()).To(Equal(tt.expressions[i]))
			}
		})
	}
}

func TestValidateExpressionReturnType(t *testing.T) {
	g := NewWithT(t)

	// Create a simple CEL environment for testing
	env, err := createCELEnvironment()
	g.Expect(err).NotTo(HaveOccurred())

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
			g := NewWithT(t)

			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			if issues != nil && issues.Err() != nil {
				if tt.expectValid {
					g.Expect(issues.Err()).NotTo(HaveOccurred(), "Expression should compile but failed")
				}
				return
			}

			// Validate the return type
			err := ValidateExpressionReturnType(ast)

			if tt.expectValid {
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
			}
		})
	}
}

func TestCompiledProgram_GetExpression(t *testing.T) {
	g := NewWithT(t)

	expression := `annotation("test-key", "test-value")`
	programs, err := CompileCELPrograms([]string{expression})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(programs).To(HaveLen(1))
	g.Expect(programs[0].GetExpression()).To(Equal(expression))
}
