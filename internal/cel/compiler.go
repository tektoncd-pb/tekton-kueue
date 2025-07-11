package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
)

// CompileCELPrograms compiles a list of CEL expressions into type-safe programs
func CompileCELPrograms(expressions []string) ([]*CompiledProgram, error) {
	if len(expressions) == 0 {
		return nil, fmt.Errorf("expressions list cannot be empty")
	}

	env, err := createCELEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	programs := make([]*CompiledProgram, 0, len(expressions))
	for i, expr := range expressions {
		if expr == "" {
			return nil, fmt.Errorf("expression %d cannot be empty", i)
		}

		program, err := compileSingleExpression(env, expr)
		if err != nil {
			return nil, fmt.Errorf("failed to compile expression %d (%q): %w", i, expr, err)
		}
		programs = append(programs, program)
	}

	return programs, nil
}

// createCELEnvironment sets up a type-safe CEL environment with PipelineRun context
func createCELEnvironment() (*cel.Env, error) {
	// Define the MutationRequest type structure for return type validation
	mutationRequestType := cel.MapType(cel.StringType, cel.AnyType)

	// Create CEL environment with proper type declarations
	env, err := cel.NewEnv(

		cel.Variable("pipelineRun", cel.MapType(cel.StringType, cel.AnyType)),
		cel.Variable("plrNamespace", cel.StringType),
		cel.Variable("pacEventType", cel.StringType),
		// Add type-safe functions for creating MutationRequests
		createMutationFunction("annotation", MutationTypeAnnotation, mutationRequestType),
		createMutationFunction("label", MutationTypeLabel, mutationRequestType),
		createPriorityMutationFunction("priority", mutationRequestType),

		// Enable standard library functions
		cel.StdLib(),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create type-safe CEL environment: %w", err)
	}

	return env, nil
}

// createMutationFunction creates a CEL function for the specified mutation type
func createMutationFunction(name string, mutationType MutationType, returnType *cel.Type) cel.EnvOption {
	return cel.Function(
		name,
		cel.Overload(
			name+"_string_string_to_mutation",
			[]*cel.Type{cel.StringType, cel.StringType},
			returnType,
			cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
				key, keyOk := lhs.Value().(string)
				value, valueOk := rhs.Value().(string)

				if !keyOk || !valueOk {
					return types.NewErr("%s function requires string arguments", name)
				}

				if key == "" {
					return types.NewErr("%s key cannot be empty", name)
				}

				// Create strongly-typed MutationRequest structure as map
				mutationMap := map[string]interface{}{
					"type":  string(mutationType),
					"key":   key,
					"value": value,
				}

				return types.NewStringInterfaceMap(types.DefaultTypeAdapter, mutationMap)
			}),
		),
	)
}

// createPriorityMutationFunction creates a CEL function for priority mutations with hardcoded key
func createPriorityMutationFunction(name string, returnType *cel.Type) cel.EnvOption {
	return cel.Function(
		name,
		cel.Overload(
			name+"_string_to_mutation",
			[]*cel.Type{cel.StringType},
			returnType,
			cel.UnaryBinding(func(val ref.Val) ref.Val {
				value, valueOk := val.Value().(string)

				if !valueOk {
					return types.NewErr("%s function requires string argument", name)
				}

				// Create strongly-typed MutationRequest structure as map with hardcoded key
				mutationMap := map[string]interface{}{
					"type":  string(MutationTypeLabel),
					"key":   "kueue.x-k8s.io/priority-class",
					"value": value,
				}

				return types.NewStringInterfaceMap(types.DefaultTypeAdapter, mutationMap)
			}),
		),
	)
}

// isValidOutputType checks if the CEL expression returns a valid type
// Valid return types: map<string, any> or list<map<string, any>>
func isValidOutputType(outputType *cel.Type) bool {
	switch outputType.Kind() {
	case cel.MapKind:
		return outputType.Parameters()[0].Kind() == cel.StringKind
	case cel.ListKind:
		elementType := outputType.Parameters()[0]
		return elementType.Kind() == cel.MapKind && elementType.Parameters()[0].Kind() == cel.StringKind
	default:
		return false
	}
}

// ValidateExpressionReturnType validates that a CEL expression returns the expected type
func ValidateExpressionReturnType(ast *cel.Ast) error {
	if !isValidOutputType(ast.OutputType()) {
		return fmt.Errorf("expression must return MutationRequest-compatible map<string, any> or list<map<string, any>>, got %v", ast.OutputType())
	}
	return nil
}

// compileSingleExpression compiles a single CEL expression with comprehensive type checking
func compileSingleExpression(env *cel.Env, expression string) (*CompiledProgram, error) {
	// Parse the expression with type checking
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("type checking failed for expression %q: %w", expression, issues.Err())
	}

	// Validate the output type matches our expected return types
	if err := ValidateExpressionReturnType(ast); err != nil {
		return nil, fmt.Errorf("invalid return type for expression %q: %w", expression, err)
	}

	// Create the program
	program, err := env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("program creation failed for expression %q: %w", expression, err)
	}

	return &CompiledProgram{
		program:    program,
		ast:        ast,
		expression: expression,
	}, nil
}
