// Package cel provides CEL (Common Expression Language) compilation and evaluation
// functionality for creating MutationRequests based on PipelineRun objects.
//
// # Overview
//
// This package enables type-safe compilation and evaluation of CEL expressions
// that generate Kubernetes mutations (annotations and labels) based on Tekton
// PipelineRun data. It provides compile-time type checking and runtime validation
// to ensure mutations are well-formed and safe.
//
// # Type Safety
//
//   - Input: *tekton.PipelineRun (strongly typed and validated)
//   - Output: []MutationRequest (validated structure and content)
//   - Functions: annotation(key, value) and label(key, value)
//   - Expressions: Single mutations or lists of mutations
//
// # Basic Usage
//
//	expressions := []string{
//		`annotation("build-info", "compiled-" + pipelineRun.metadata.name)`,
//		`label("environment", "production")`,
//		`[annotation("key1", "value1"), label("key2", "value2")]`,
//	}
//
//	programs, err := cel.CompileCELPrograms(expressions)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	pipelineRun := &tekton.PipelineRun{...}
//	for _, program := range programs {
//		mutations, err := program.Evaluate(pipelineRun)
//		if err != nil {
//			log.Printf("Error: %v", err)
//			continue
//		}
//		// Apply mutations to Kubernetes resources...
//	}
//
// # CELMutator Usage
//
// For convenient mutation application, use the CELMutator:
//
//	expressions := []string{
//		`annotation("tekton.dev/pipeline", "my-pipeline")`,
//		`[label("env", "production"), annotation("owner", "team-a")]`,
//	}
//
//	programs, err := cel.CompileCELPrograms(expressions)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	mutator := cel.NewCELMutator(programs)
//	pipelineRun := &tekton.PipelineRun{...}
//
//	err = mutator.Mutate(pipelineRun)
//	if err != nil {
//		log.Printf("Mutation failed: %v", err)
//	}
//	// PipelineRun is now modified with labels and annotations
//
// # Available CEL Functions
//
//   - annotation(key: string, value: string) -> MutationRequest
//   - label(key: string, value: string) -> MutationRequest
//
// # Package Structure
//
// This package is organized into focused modules:
//
//   - types.go: Core data types (MutationType, MutationRequest) and validation
//   - compiler.go: CEL environment setup, compilation, and type checking
//   - evaluator.go: Runtime program evaluation and result conversion
//   - mutator.go: CELMutator for convenient mutation application
//   - example_usage.go: Comprehensive usage examples and patterns
//
// # Validation Hierarchy
//
//  1. Compile-time: CEL type checker validates function signatures and return types
//  2. Runtime input: Validates PipelineRun is not nil and properly structured
//  3. Runtime output: Validates returned data has correct MutationRequest structure
//  4. Field validation: Validates all required fields (type, key, value) are present and valid
//
// # Error Handling
//
// The package provides detailed error messages for:
//   - Compilation failures with expression context
//   - Type mismatches with expected vs actual types
//   - Runtime evaluation errors with expression details
//   - Validation failures with field-specific information
package cel
