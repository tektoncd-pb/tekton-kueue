package cel

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

const (
	// Kubernetes label value length limit
	maxLabelValueLength = 63

	// Kubernetes label key length limit (for the part after the prefix)
	maxLabelKeyLength = 63

	// Kubernetes domain prefix length limit
	maxDomainPrefixLength = 253
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

func TestValidateExpressionReturnType_ValidCases(t *testing.T) {
	g := NewWithT(t)

	// Create a simple CEL environment for testing
	env, err := createCELEnvironment()
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name        string
		expression  string
		description string
	}{
		{
			name:        "valid single annotation",
			expression:  `annotation("key", "value")`,
			description: "Returns map<string, any> representing MutationRequest",
		},
		{
			name:        "valid annotation list",
			expression:  `[annotation("key1", "value1"), annotation("key2", "value2")]`,
			description: "Returns list<map<string, any>> representing []MutationRequest",
		},
		{
			name:        "valid mixed list",
			expression:  `[annotation("key1", "value1"), label("key2", "value2")]`,
			description: "Returns list<map<string, any>> with mixed mutation types",
		},
		{
			name:        "valid priority function",
			expression:  `priority("high")`,
			description: "Returns map<string, any> representing priority MutationRequest",
		},
		{
			name:        "valid priority in list",
			expression:  `[priority("medium"), annotation("queue", "default")]`,
			description: "Returns list<map<string, any>> with priority and annotation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			g.Expect(issues.Err()).NotTo(HaveOccurred(), "Expression should compile successfully")

			// Validate the return type
			err := validateExpressionReturnType(ast)
			g.Expect(err).NotTo(HaveOccurred(), tt.description)
		})
	}
}

func TestValidateExpressionReturnType_InvalidCases(t *testing.T) {
	g := NewWithT(t)

	// Create a simple CEL environment for testing
	env, err := createCELEnvironment()
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name        string
		expression  string
		description string
	}{
		{
			name:        "invalid string return",
			expression:  `"just a string"`,
			description: "Returns string instead of MutationRequest structure",
		},
		{
			name:        "invalid number return",
			expression:  `42`,
			description: "Returns number instead of MutationRequest structure",
		},
		{
			name:        "invalid list of strings",
			expression:  `["string1", "string2"]`,
			description: "Returns list<string> instead of list<map<string, any>>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			g.Expect(issues.Err()).NotTo(HaveOccurred(), "Expression should compile successfully")

			// Validate the return type
			err := validateExpressionReturnType(ast)
			g.Expect(err).To(HaveOccurred(), tt.description)
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

func TestReplaceFunction(t *testing.T) {
	g := NewWithT(t)

	// Create a CEL environment for testing
	env, err := createCELEnvironment()
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name       string
		expression string
		expected   string
	}{
		{
			name:       "replace forward slash with dash",
			expression: `replace("linux/amd64", "/", "-")`,
			expected:   "linux-amd64",
		},
		{
			name:       "replace multiple occurrences",
			expression: `replace("hello world hello", "hello", "hi")`,
			expected:   "hi world hi",
		},
		{
			name:       "replace with empty string",
			expression: `replace("test-value", "-", "")`,
			expected:   "testvalue",
		},
		{
			name:       "replace non-existent character",
			expression: `replace("test", "x", "y")`,
			expected:   "test",
		},
		{
			name:       "replace entire string",
			expression: `replace("old", "old", "new")`,
			expected:   "new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			g.Expect(issues.Err()).NotTo(HaveOccurred(), "Expression should compile successfully")

			// Create program and evaluate
			program, err := env.Program(ast)
			g.Expect(err).NotTo(HaveOccurred(), "Program creation should succeed")

			// Evaluate the expression
			result, _, err := program.Eval(map[string]interface{}{})
			g.Expect(err).NotTo(HaveOccurred(), "Evaluation should succeed")

			// Check the result
			g.Expect(result.Value()).To(Equal(tt.expected))
		})
	}
}

func TestKubernetesKeyValidation(t *testing.T) {
	g := NewWithT(t)

	// Create a CEL environment for testing
	env, err := createCELEnvironment()
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name        string
		expression  string
		expectError bool
		errorMsg    string
	}{
		// Valid annotation keys
		{
			name:        "valid annotation key without prefix",
			expression:  `annotation("simple-key", "value")`,
			expectError: false,
		},
		{
			name:        "valid annotation key with prefix",
			expression:  `annotation("example.com/my-key", "value")`,
			expectError: false,
		},
		{
			name:        "valid annotation key with complex prefix",
			expression:  `annotation("sub.domain.example.com/my-key", "value")`,
			expectError: false,
		},
		// Valid label keys
		{
			name:        "valid label key without prefix",
			expression:  `label("app", "value")`,
			expectError: false,
		},
		{
			name:        "valid label key with prefix",
			expression:  `label("kubernetes.io/os", "value")`,
			expectError: false,
		},
		// Invalid annotation keys
		{
			name:        "invalid annotation key - starts with dash",
			expression:  `annotation("-invalid", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		{
			name:        "invalid annotation key - ends with dash",
			expression:  `annotation("invalid-", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		{
			name:        "invalid annotation key - too long name",
			expression:  `annotation("` + strings.Repeat("a", maxLabelKeyLength+1) + `", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		{
			name:        "invalid annotation key - multiple slashes",
			expression:  `annotation("domain.com/path/invalid", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		{
			name:        "invalid annotation key - invalid prefix",
			expression:  `annotation("invalid-.com/key", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		{
			name:        "invalid annotation key - prefix too long",
			expression:  `annotation("` + strings.Repeat("a", maxDomainPrefixLength+1) + `.com/key", "value")`,
			expectError: true,
			errorMsg:    "annotation key validation failed",
		},
		// Invalid label keys
		{
			name:        "invalid label key - starts with dash",
			expression:  `label("-invalid", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		{
			name:        "invalid label key - ends with dash",
			expression:  `label("invalid-", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		{
			name:        "invalid label key - too long name",
			expression:  `label("` + strings.Repeat("a", maxLabelKeyLength+1) + `", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		{
			name:        "invalid label key - multiple slashes",
			expression:  `label("domain.com/path/invalid", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		{
			name:        "invalid label key - invalid prefix",
			expression:  `label("invalid-.com/key", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		{
			name:        "invalid label key - prefix too long",
			expression:  `label("` + strings.Repeat("a", maxDomainPrefixLength+1) + `.com/key", "value")`,
			expectError: true,
			errorMsg:    "label key validation failed",
		},
		// Invalid label values
		{
			name:        "invalid label value - starts with dash",
			expression:  `label("valid-key", "-invalid")`,
			expectError: true,
			errorMsg:    "label value validation failed",
		},
		{
			name:        "invalid label value - ends with dash",
			expression:  `label("valid-key", "invalid-")`,
			expectError: true,
			errorMsg:    "label value validation failed",
		},
		{
			name:        "invalid label value - too long",
			expression:  `label("valid-key", "` + strings.Repeat("a", maxLabelValueLength+1) + `")`,
			expectError: true,
			errorMsg:    "label value validation failed",
		},
		{
			name:        "invalid label value - contains invalid characters",
			expression:  `label("valid-key", "invalid/value")`,
			expectError: true,
			errorMsg:    "label value validation failed",
		},
		{
			name:        "invalid label value - contains spaces",
			expression:  `label("valid-key", "invalid value")`,
			expectError: true,
			errorMsg:    "label value validation failed",
		},
		{
			name:        "valid label value - empty",
			expression:  `label("valid-key", "")`,
			expectError: false,
		},
		{
			name:        "valid label value - alphanumeric",
			expression:  `label("valid-key", "valid123")`,
			expectError: false,
		},
		{
			name:        "valid label value - with dashes underscores dots",
			expression:  `label("valid-key", "valid-value_with.dots")`,
			expectError: false,
		},
		// Invalid annotation values - removed due to CEL expression size limits
		{
			name:        "valid annotation value - empty",
			expression:  `annotation("valid-key", "")`,
			expectError: false,
		},
		{
			name:        "valid annotation value - with special characters",
			expression:  `annotation("valid-key", "value with spaces and special chars: !@#$%^&*()")`,
			expectError: false,
		},
		{
			name:        "valid annotation value - unicode",
			expression:  `annotation("valid-key", "测试中文字符 🚀")`,
			expectError: false,
		},
		{
			name:        "valid annotation value - multiline",
			expression:  `annotation("valid-key", "line1\nline2\nline3")`,
			expectError: false,
		},
		// Note: Large annotation value testing is done in TestValidationFunctions
		// to avoid CEL expression size limits
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Compile the expression
			ast, issues := env.Compile(tt.expression)
			g.Expect(issues.Err()).NotTo(HaveOccurred(), "Expression should compile successfully")

			// Create program and evaluate
			program, err := env.Program(ast)
			g.Expect(err).NotTo(HaveOccurred(), "Program creation should succeed")

			// Evaluate the expression
			result, _, err := program.Eval(map[string]interface{}{})

			if tt.expectError {
				g.Expect(err).To(HaveOccurred(), "Expected evaluation to fail")
				g.Expect(err.Error()).To(ContainSubstring(tt.errorMsg))
			} else {
				g.Expect(err).NotTo(HaveOccurred(), "Expected evaluation to succeed")
				g.Expect(result).NotTo(BeNil(), "Expected valid result")
			}
		})
	}
}

func TestValidationFunctions(t *testing.T) {
	g := NewWithT(t)

	// Test validateLabelValue
	t.Run("validateLabelValue", func(t *testing.T) {
		// Valid label values
		g.Expect(validateLabelValue("")).To(Succeed())
		g.Expect(validateLabelValue("valid123")).To(Succeed())
		g.Expect(validateLabelValue("valid-value_with.dots")).To(Succeed())

		// Invalid label values
		err := validateLabelValue("-invalid")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("label value"))

		err = validateLabelValue("invalid-")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("label value"))

		err = validateLabelValue(strings.Repeat("a", maxLabelValueLength+1))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("label value"))

		err = validateLabelValue("invalid/value")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("label value"))
	})

	// Test validateAnnotationValue
	t.Run("validateAnnotationValue", func(t *testing.T) {
		// Valid annotation values
		g.Expect(validateAnnotationValue("")).To(Succeed())
		g.Expect(validateAnnotationValue("valid value with spaces")).To(Succeed())
		g.Expect(validateAnnotationValue("测试中文字符 🚀")).To(Succeed())
		g.Expect(validateAnnotationValue("line1\nline2\nline3")).To(Succeed())
		g.Expect(validateAnnotationValue(strings.Repeat("a", maxAnnotationValueSize))).To(Succeed())

		// Invalid annotation value - too long
		err := validateAnnotationValue(strings.Repeat("a", maxAnnotationValueSize+1))
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(And(
			ContainSubstring("annotation value is too long"),
			ContainSubstring("262145 bytes"),
			ContainSubstring("maximum allowed is 262144 bytes"),
		))
	})
}
